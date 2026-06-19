# rbstat — Implementation Specification

**Module:** `github.com/binRick/rbstat` · **Target:** Linux-only, Go 1.22+, x86_64 · **Status:** original design spec

> **Post-implementation deltas** (the shipped code, verified against pcp-dstat
> 7.1.5 on a live host, refines a few details below):
> - **paging (`-g`)** reads swap pages `pswpin`/`pswpout` (dstat's
>   `swap.pagesin/out`), formatted as a base-1000 decimal **count** — not
>   `pgpgin`/`pgpgout` bytes, which would just mirror disk throughput.
> - **memory `used`** = `MemTotal − MemFree − Buffers − Cached − SReclaimable +
>   Shmem` (procps/`free`), matching dstat to the kilobyte.
> - **load** uses `colorstep = 0.5`; **net/total** also excludes `team\d+`.
> - **`str.center`** padding puts the extra dash on the **left**, and the number
>   formatter is integer-only for `b`/`d`/`p` types (only `f` adds decimals) —
>   both matching pcp-dstat exactly.
> - The binary compiles on non-Linux (build-tagged `isatty`/`termRows`); `/proc`
>   reads simply error at runtime off Linux.

---

## 1. Overview & goals

`rbstat` is a faithful Go clone of the classic Linux `dstat` resource monitor. It samples `/proc` at a fixed interval, diffs counters between samples, and prints a colorized, dash-padded, fixed-width column table that is **visually identical to `dstat`** for the supported plugin set.

**v1 scope:**
- Single static binary, stdlib-only (no external deps required; see §6 for the one optional dep decision — declined).
- Linux-only. The binary may compile on other OSes but `/proc` collectors return an error there; we gate with `//go:build linux` on collector files and provide a stub that errors on non-Linux for the rest.
- Ten collectors: cpu, disk, net, page, sys, mem, swap, load, time, io.
- Default plugin set `-cdngy` (cpu, disk, net, page, sys), exactly matching dstat's default.
- Positional `[delay [count]]`; clustered short flags (`-cdn`).
- Faithful rendering: dash-padded group titles via center-and-replace, blue `|` separators, magnitude-based ANSI coloring, `fchg`/`dchg`-style 5-char number formatting with k/M/G suffixes (base-1024 for bytes).
- `--nocolor`, `--noupdate`, `--noheaders`, `--version`, `--list`. `-C/-D/-N` device targeting is a **stretch goal** (interface designed for it; v1 ships total-only).

**Explicit non-goals for v1:** `--output` CSV, `--top-*`, `--bits`/`--integer`/`--float` (designed-for but off), `--update` in-place redraw (we ship scrolling mode; `--noupdate` is the default-on behavior, see §3), external plugins, per-device expansion (`-f`/`all`).

**Guiding principle:** when in doubt, do what dstat-real's source does. Correctness and visual fidelity beat cleverness.

---

## 2. Architecture

### Package layout

```
github.com/binRick/rbstat
├── main.go                  package main: wire CLI → engine → loop
├── cli.go                   package main: argument parser (clustered flags + positionals)
├── engine.go                package main: sample loop, dt timing, header cadence, signal handling
├── internal/render          rendering: header(), data row, separators, header cadence helper
├── internal/format          fchg/dchg number formatter + unit scaling
├── internal/color           ANSI table, theme, magnitude→color rule, tty detection
├── internal/collect         Collector interface + RateBase helper + registry
│   └── procfs.go            shared /proc file readers + field-split helpers (cached per tick)
└── internal/collectors      one file per stat group, each implements collect.Collector
    ├── cpu.go disk.go net.go page.go sys.go
    └── mem.go swap.go load.go time.go io.go
```

Rationale: `internal/` keeps the surface private. `collect` defines the contract; `collectors` implement it; `render`/`format`/`color` are pure presentation and have no `/proc` knowledge. The engine owns timing and the previous-snapshot lifecycle.

### Core abstraction — the Collector interface

Each stat group is a `Collector`. Rate-diffing is **NOT** duplicated in every collector — it lives in a shared `RateBase` helper that collectors embed. The collector's job is reduced to: (1) read raw counters/gauges this tick, (2) for gauges return values directly, (3) for rates hand the raw counters to `RateBase` which stores the previous sample and divides deltas by `dt`. This is the decisive design choice: **rate math is centralized**, collectors stay thin and only know their `/proc` fields and formulas.

```go
// internal/collect/collect.go
package collect

import "time"

// ColType drives the renderer's formatter & color choice (mirrors dstat's ctype).
type ColType int

const (
	TypePercent ColType = iota // 'p' — cpu; scale buckets, no unit suffix
	TypeByte                   // 'b' — disk/net/page; base-1024, B/k/M/G... suffix
	TypeDecimal                // 'd' — sys; base-1000, integer, k/M suffix
	TypeFloat                  // 'f' — load/io; base-1000, decimals allowed
	TypeString                 // 's' — time; left-justified text, no scaling
)

// Field describes one sub-column within a group.
type Field struct {
	Nick string // sub-column header text, e.g. "usr", "read"
}

// Collector is implemented by every stat group.
type Collector interface {
	// Name is a stable identifier (matches the long flag, e.g. "cpu", "disk").
	Name() string

	// Title is the group banner text BEFORE dash-padding, e.g. "total cpu usage".
	Title() string

	// Fields lists the sub-columns in display order.
	Fields() []Field

	// Type selects formatter/color behavior for the whole group.
	Type() ColType

	// Width is the per-sub-column field width (3 for cpu, 5 for everything else).
	Width() int

	// Scale is the per-group base/bucket size: 34 (cpu pct), 1024 (byte), 1000 (decimal/float).
	Scale() float64

	// Collect reads /proc for this tick and returns one float64 per Field, in order.
	// `now` is the wall-clock timestamp captured immediately before this tick's reads,
	// used for rate division. Length of the return slice MUST equal len(Fields()).
	Collect(now time.Time) ([]float64, error)
}
```

```go
// internal/collect/ratebase.go
package collect

import "time"

// RateBase centralizes counter-diffing for rate collectors. Embed it; on each
// tick call Rates(now, raw...) with the current raw cumulative counters.
// It stores the previous sample + timestamp and returns per-counter rates.
// First tick (no prior sample) returns zeros — matching dstat's first line.
type RateBase struct {
	prev   []uint64
	prevAt time.Time
	have   bool
}

// Rates returns (cur[i]-prev[i]) / dt for each counter, where dt is measured
// wall-clock seconds since the previous call. Counter wrap (cur < prev) yields 0
// for that counter rather than a huge spike. dt<=0 guard returns zeros.
func (b *RateBase) Rates(now time.Time, cur ...uint64) []float64 {
	out := make([]float64, len(cur))
	if b.have {
		dt := now.Sub(b.prevAt).Seconds()
		if dt > 0 {
			for i := range cur {
				if i < len(b.prev) && cur[i] >= b.prev[i] {
					out[i] = float64(cur[i]-b.prev[i]) / dt
				}
			}
		}
	}
	b.prev = append(b.prev[:0], cur...)
	b.prevAt = now
	b.have = true
	return out
}
```

Gauge collectors (mem, swap, load) ignore `RateBase` and return instantaneous values directly. CPU is a special rate (ratio of deltas where `dt` cancels): it stores its own previous raw 8-bucket sample and computes percentages — it embeds `RateBase` only to reuse the prev-sample storage convention but computes ratios itself (see §5). The registry maps flags → constructors:

```go
// internal/collect/registry.go
package collect

// New returns a fresh Collector for the given name, or (nil,false) if unknown.
// Implemented in the collectors package via init-registration to avoid import cycles,
// or a simple switch in main. v1 uses a switch in cli.go (see §3).
```

---

## 3. CLI

### Parsing strategy: **manual parser**, not `flag`.

The stdlib `flag` package cannot do clustered short flags (`-cdn` = `-c -d -n`) and is awkward with optional trailing positionals. We hand-roll a small parser in `cli.go`. Algorithm:

```
args = os.Args[1:]
for each arg:
  if arg == "--":            remaining are positionals
  else if arg startswith "--":  long option (split on "=" for value-bearing ones)
  else if arg startswith "-" and len>1 and not all-digits:
        cluster: iterate each rune after "-", each is a short flag
        (a short flag that consumes a value, e.g. -C, takes the rest of the
         cluster or the next arg as its value, then ends the cluster)
  else:                      positional (delay then count)
positionals: delay = atoi-or-float(pos[0]) default 1; count = atoi(pos[1]) default 0 (unlimited)
```

If **no** stat-selection flag was given, print to stderr:
`You did not select any stats, using -cdngy by default.`
and enable the default set `[cpu, disk, net, page, sys]` (preserving that fixed order).

When flags ARE given, collectors appear in the **order the flags were encountered** (dstat behavior), de-duplicated.

### v1 flag set → collector mapping

| Short | Long | Collector `Name()` | Notes |
|---|---|---|---|
| `-c` | `--cpu` | `cpu` | default-set member |
| `-d` | `--disk` | `disk` | default-set member |
| `-n` | `--net` | `net` | default-set member |
| `-g` | `--page` | `page` | default-set member |
| `-y` | `--sys` | `sys` | default-set member |
| `-m` | `--mem` | `mem` | |
| `-s` | `--swap` | `swap` | |
| `-l` | `--load` | `load` | |
| `-t` | `--time` | `time` | |
| `-r` | `--io` | `io` | I/O **requests** (IOPS), distinct from `-d` bytes |
| `-a` | `--all` | expands to `-cdngy` | alias |

### Behavior flags (no collector)

| Flag | Effect |
|---|---|
| `--nocolor` | force color off (also auto-off when stdout is not a tty — see §4) |
| `--noheaders` | never reprint the header rows (print once? No — dstat suppresses repeats; v1: print header once at top, never repeat) |
| `--noupdate` | one clean line per `delay` interval (this is v1's only/default emit mode; flag accepted as no-op-confirming for fidelity) |
| `--version` | print `rbstat <version>` and exit 0 |
| `--list` | print available collector names (one per line) and exit 0 |
| `-h`, `--help` | usage text, exit 0 |

### Stretch (designed-for, not in v1)

`-C <list>` (cpu targeting), `-D <list>` (disk), `-N <list>` (net), `-f/--full`. The `Collector` interface accommodates these later by allowing a constructor to take a device filter; v1 constructors take no filter and emit total-only. Do not implement in v1; reserve the flag letters so they error cleanly (`unknown handling: -C requires device targeting, not supported in v1`).

**Delay/count semantics:** `delay` (float seconds, default 1), `count` (int, default 0 = forever). The loop sleeps `delay` between ticks and exits after `count` data lines if `count>0`.

---

## 4. Rendering

### 4.1 Width model

- Default sub-column field width **W = 5** for all collectors **except cpu, which uses W = 3**.
- A group renders one device (total mode), so the **group-title bar width** = `len(fields)*W + (len(fields)-1)` (sum of sub-column widths + single spaces between them).

| Group | calc | title width |
|---|---|---|
| cpu | 5×3 + 4 | **19** |
| disk/net/page/sys | 2×5 + 1 | **11** |
| mem | 4×5 + 3 | **23** |
| swap | 2×5 + 1 | **11** |
| load | 3×5 + 2 | **17** |
| io | 2×5 + 1 | **11** |
| time | 1×5 + 0 | **5** (but time formats its own wider string; see §5) |

### 4.2 Group-title dash padding (center-then-dash)

Replicate Python `str.center(W)` exactly: total pad = `W - len(s)`; **left = pad/2 (floor), right = pad - left (ceil)** — when pad is odd the extra space goes RIGHT. Then replace every space (the pad spaces only; `s` has no interior spaces that should become dashes except dstat replaces ALL spaces in the centered result) with `-`.

```go
// format.CenterDash(s, w) -> dash-padded title
func CenterDash(s string, w int) string {
	if len(s) > w { s = s[:w] }
	pad := w - len(s)
	left := pad / 2
	right := pad - left
	b := strings.Repeat("-", left) + s + strings.Repeat("-", right)
	return strings.ReplaceAll(b, " ", "-") // spaces inside s (e.g. "total cpu usage") also become dashes
}
```

Verified outputs: cpu→`--total-cpu-usage--`, disk→`-dsk/total-`, net→`-net/total-`, page→`--paging---`, sys→`--system---`, mem(`memory usage`,len12,W23)→`-----memory-usage-----`? pad=11→left5,right6→`-----memory usage------`→`-----memory-usage------`.

### 4.3 Sub-title (sub-column header) line

Each nick is `center(W)` (same floor-left/ceil-right rule); nicks within a group joined by a single space. Verified: disk `read  writ`, net `recv  send`, page ` in   out `, sys ` int   csw `, cpu `usr sys idl wai stl`.

### 4.4 Separators & two-row header

- **Title row:** groups joined by a single **space**, painted with `frame` (darkblue).
- **Sub-title row AND every data row:** groups joined by the vertical bar **`|`**, painted `frame` (darkblue).
- Within a group, sub-columns joined by a single space.

So the header is two lines (title, then sub-title); data rows align under sub-titles with `|` at each group boundary.

### 4.5 Header reprint cadence

dstat reprints every `rows-2` data lines (scrolling mode), only if terminal height ≥ 6. **v1 simplification (decisive):** since v1 is scrolling-only and we don't track terminal height robustly cross-environment, reprint the two header lines every **N data lines where N is the terminal height minus 2**, obtained via `golang.org/x/term`? — **No external dep.** Use the `TIOCGWINSZ` ioctl directly (syscall) to get rows; if it fails or rows<6 or stdout is not a tty, print the header **once** at the top and never repeat. `--noheaders` forces header-once-suppressed → print header once, never repeat. (Matching dstat's "≥6 rows" gate and `rows-2` interval when a real tty is present.)

```
headerEvery := rows - 2          // when rows >= 6 and tty
if !tty || rows < 6 || noheaders { headerEvery = 0 }  // 0 => print once at start only
// in loop: if headerEvery>0 && line%headerEvery==0 { printHeader() }; if first line, printHeader()
```

### 4.6 Number formatter (fchg/dchg)

The formatter fits a value into a W-char column. For byte/decimal/float types with `scale ∈ {1000,1024}` and `W ≥ 4`, **reserve 1 char for the unit letter**: value gets `W-1` chars, unit gets 1. Base = 1024 if scale==1024 else 1000.

Unit tables:
- byte type, base 1024: `["B","k","M","G","T","P","E","Z","Y"]`
- decimal/float, base 1000: `[" ","k","M","G","T","P","E","Z","Y"]` (ones bucket is a space)
- cpu (percent, scale 34): no unit reservation; plain integer right-justified in W=3, no suffix.

**`dchg` (integer fit)** — for `TypeDecimal`:
```
c = 0
loop:
  ret = strconv.Itoa(int(round(var)))
  if len(ret) <= valWidth: break
  var /= base; c++
return ret, c
```

**`fchg` (float fit)** — for `TypeByte`/`TypeFloat`:
```
c = 0
loop:
  if var == 0 { ret = "0"; break }
  ret = strconv.Itoa(int(round(var)))           // integer string of (possibly reduced) var
  if len(ret) <= valWidth:
      i = valWidth - len(ret) - 1               // how many decimals could fit
      for i > 0:
          cand = fmt.Sprintf("%.*f", i, var)
          if len(cand) <= valWidth && cand != strconv.Itoa(int(round(var))):
              ret = cand; break
          i--
      // else ret stays the integer string
      break
  var /= base; c++
return ret, c
```

**Assembly (`cprint`-equivalent):**
```
1. choose base, units, valWidth (=W-1 if unit reserved else W)
2. (ret, c) = dchg/fchg(var, valWidth, base)   // percent: ret=itoa(round(var)), c=-1, no unit
3. colored = color(var, c, type, scale) + rightJustify(ret, valWidth)   // numbers right-justified
4. if unitReserved:
       if c != -1 && round(var) != 0:  colored += unitColor + units[c]
       else:                            colored += " "                  // trailing space keeps width
5. colored += reset
```

Verified examples (byte, base 1024, W=5 → valWidth 4): `0`→`   0`+space; `55`→`  55B`; `5457`→`5457B`; `56291`(~55k)→`  55k`; `10485760`(10MiB)→`10.0M`; `1234567`→`1206k`. **The governing rule: advance to the next unit only when the integer part no longer fits valWidth** (so it shows `9999k` before rolling to `10.0M`).

### 4.7 Color model

ANSI codes (dark-background default theme):

| Logical | Code | Use |
|---|---|---|
| title | `\033[0;34m` (darkblue) | group dash bars |
| subtitle | `\033[1;34m\033[4m` (blue+underline) | sub-column nicks |
| frame | `\033[0;34m` (darkblue) | `\|` separators, title-row spaces |
| default/reset | `\033[0;0m` | end of each colored token |
| unit | `\033[1;30m` (darkgray) | trailing unit letters, and a bare `0` value |
| error | `\033[1;37m\033[41m` (white on red) | negative values |
| done (pct ≥100) | `\033[1;37m` (white) | cpu column at 100% |

**Value color table (`colors_lo`, the bright set used every normal tick):**
```
index: 0=red(1;31) 1=yellow(1;33) 2=green(1;32) 3=blue(1;34)
       4=cyan(1;36) 5=white(1;37) 6=darkred(0;31) 7=darkgreen(0;32)
```

**Threshold rule (mirrors dstat `cprint`):**
```
if ret == "0":                          color = unit      (darkgray)
elif var < 0:                           color = error     (white-on-red, render "-")
elif type == percent && round(var)>=100: color = done     (white)
elif scale not in {1000,1024}:          color = colors[int(var/scale) % 8]   // cpu: scale 34
else (byte/decimal/float):              color = colors[c % 8]                // c = #divisions by base
```

So cpu buckets: 0–33%→red, 34–67%→yellow, 68–99%→green. Byte/decimal: ones→red, k→yellow, M→green, G→blue, T→cyan, … cycling mod 8. This gives the signature "hue shifts as the number crosses k/M/G".

**Color enable logic:** color ON unless `--nocolor` OR stdout is not a terminal. tty detection: `isatty` via `unix.IoctlGetTermios`/`TIOCGETA`? Linux: `ioctl(fd, TCGETS)` succeeds on a tty. Implement `color.IsTTY(fd)` with a raw `unix.Syscall(SYS_IOCTL, fd, TCGETS, ...)` — no external dep. When color is off, every color/reset string emitted by the renderer is `""`.

---

## 5. Per-collector spec

`dt` = measured wall-clock seconds (`now.Sub(prevAt)`), passed via `RateBase`. All counters parsed as `uint64`; wrap (cur<prev) → 0 for that field. All `/proc` files read once per tick and cached (see §6 `procfs.go`).

| Collector | Title | Fields (nicks) | W | Type / Scale | /proc source | Field indices (token, 0-based incl. label) | Formula | Rate/Gauge | Unit |
|---|---|---|---|---|---|---|---|---|---|
| **cpu** | `total cpu usage` | usr sys idl wai stl | 3 | Percent / 34 | `/proc/stat` line `cpu ` | user=1 nice=2 system=3 idle=4 iowait=5 irq=6 softirq=7 steal=8 (ignore guest=9, guest_nice=10 — already in user/nice) | d_total = Σ deltas of tokens 1..8. usr=100·(d1+d2)/dt_tot; sys=100·(d3+d6+d7)/dt_tot; idl=100·d4/dt_tot; wai=100·d5/dt_tot; stl=100·d8/dt_tot. Guard d_total==0→zeros | Rate (ratio; dt cancels) | % |
| **disk** | `dsk/total` | read writ | 5 | Byte / 1024 | `/proc/diskstats` | sectors_read=5, sectors_written=9 | read=(Σ d_sec_read·512)/dt; writ=(Σ d_sec_writ·512)/dt over whole disks only | Rate | bytes/s |
| **net** | `net/total` | recv send | 5 | Byte / 1024 | `/proc/net/dev` | split on `:`; RHS numeric: recv_bytes=0, send_bytes=8 | recv=Σ d_recv/dt; send=Σ d_send/dt over included ifaces | Rate | bytes/s |
| **page** | `paging` | in out | 5 | Byte / 1024 | `/proc/vmstat` map | keys `pgpgin`, `pgpgout` (values already KiB) | in=(d_pgpgin·1024)/dt; out=(d_pgpgout·1024)/dt → report **bytes/s** | Rate | bytes/s |
| **sys** | `system` | int csw | 5 | Decimal / 1000 | `/proc/stat` | `intr` total=token1; `ctxt`=token1 | int=d_intr/dt; csw=d_ctxt/dt | Rate | count/s |
| **mem** | `memory usage` | used free buff cach | 5 | Byte / 1024 | `/proc/meminfo` map (kB→×1024) | MemTotal, MemFree, Buffers, Cached | used=(MemTotal−MemFree−Buffers−Cached)·1024; free=MemFree·1024; buff=Buffers·1024; cach=Cached·1024 | Gauge | bytes |
| **swap** | `swap` | used free | 5 | Byte / 1024 | `/proc/meminfo` map | SwapTotal, SwapFree | used=(SwapTotal−SwapFree)·1024; free=SwapFree·1024; if SwapTotal==0→0,0 | Gauge | bytes |
| **load** | `load avg` | 1m 5m 15m | 5 | Float / 1000 | `/proc/loadavg` | tokens 0,1,2 | parse as float64 directly | Gauge | ratio |
| **time** | `system` | time | (auto) | String | wall clock | — | `now.Format("02-01 15:04:05")` (dstat default `%d-%m %H:%M:%S`); title bar width = len of formatted string; sub-col `time` left-justified | Gauge | — |
| **io** | `io/total` | read writ | 5 | Float / 1000 | `/proc/diskstats` | reads_completed=3, writes_completed=7 | read=Σ d_reads/dt; writ=Σ d_writes/dt over whole disks only | Rate | requests/s (IOPS) |

**Disk/io "whole-disk-only" device filter (exclude partitions & virtual):** include a device unless its name matches either:
- partition: ends in a digit for `sd*/hd*/vd*` (e.g. `sda1`), or NVMe `…p<digits>` (e.g. `nvme0n1p1`), or `cciss/c\d+d\d+p\d+`, or `mmcblk\d+p\d+`;
- virtual/pseudo: matches `^(ram|loop|fd|dm-|md|sr|zram|drbd|nbd)`.
Implement as two compiled regexps in `procfs.go`. (Authoritative cross-check `/sys/block/<name>` existence is acceptable but the regexp matches dstat and avoids extra syscalls.)

**Net "total" iface filter (exclude):** name matches `^(lo|bond\d+|face|.+\.\d+)$` → excluded; sum the rest. (`lo`, bonded slaves, VLANs.)

**Map files** (`/proc/vmstat`, `/proc/meminfo`): build `map[string]uint64` by `key value` (strip trailing `kB` on meminfo); never rely on line order.

**`/proc/<pid>` (time collector does NOT need it).** No per-process scanning in v1.

---

## 6. File manifest

`go.mod`: `module github.com/binRick/rbstat` / `go 1.22`. **No external dependencies.** (We decline `golang.org/x/sys` and `golang.org/x/term`: tty size/detection via raw `syscall`/`golang.org/x/sys` would add a dep — instead use `syscall` constants `TCGETS`/`TIOCGWINSZ` directly through `unsafe` + `syscall.Syscall`, all stdlib. If the implementer finds the raw ioctl too fiddly, `golang.org/x/sys/unix` is the single justified small dep — but attempt stdlib first.)

| File | Responsibility (one line) |
|---|---|
| `go.mod` | module `github.com/binRick/rbstat`, go 1.22, no deps |
| `main.go` | entrypoint: parse args, build collector list, run engine; handle `--version`/`--list`/`--help` |
| `cli.go` | manual arg parser: clustered short flags, long flags, `[delay [count]]`, default-set message |
| `engine.go` | sample loop, dt timing via `time.Now`, SIGINT handling, header cadence, calls render |
| `internal/collect/collect.go` | `Collector` interface, `ColType`, `Field` |
| `internal/collect/ratebase.go` | `RateBase` shared counter-diff helper |
| `internal/collect/procfs.go` | per-tick cached `/proc` file reads, line/field split helpers, device-filter regexps, meminfo/vmstat map builders |
| `internal/collectors/cpu.go` | cpu collector (`/proc/stat` cpu line, ratio formula) |
| `internal/collectors/disk.go` | disk collector (diskstats sectors, whole-disk sum) |
| `internal/collectors/net.go` | net collector (`/proc/net/dev`, iface filter) |
| `internal/collectors/page.go` | page collector (`/proc/vmstat` pgpgin/out) |
| `internal/collectors/sys.go` | sys collector (`/proc/stat` intr/ctxt) |
| `internal/collectors/mem.go` | mem collector (`/proc/meminfo` gauge) |
| `internal/collectors/swap.go` | swap collector (`/proc/meminfo` gauge) |
| `internal/collectors/load.go` | load collector (`/proc/loadavg` gauge) |
| `internal/collectors/time.go` | time collector (wall clock string) |
| `internal/collectors/io.go` | io collector (diskstats reads/writes completed, IOPS) |
| `internal/format/format.go` | `CenterDash`, `dchg`, `fchg`, unit tables, column assembly |
| `internal/color/color.go` | ANSI codes, theme, magnitude→color rule, tty detection, on/off gate |
| `internal/render/render.go` | two-row header builder, data-row builder, `\|`/space separators, header cadence + terminal rows via ioctl |

Optional: `internal/collect/registry.go` — name→constructor switch (or keep that switch in `cli.go`; v1 keeps it in `cli.go` to minimize files).

---

## 7. Implementation order

1. **Scaffold.** `go mod init github.com/binRick/rbstat`; create `main.go` printing nothing but exiting 0; `internal/collect/collect.go` with the interface; `internal/collect/ratebase.go`. `go build ./...` must pass.
2. **Format + color, unit-tested in isolation.** Implement `format.CenterDash`, `dchg`, `fchg`, and `color`'s table + rule. Write table-driven tests using the verified examples from §4.6 (`5457`→`5457B`, `56291`→`  55k`, `10485760`→`10.0M`, cpu `97`→`97` red/green per bucket) and §4.2 (`--total-cpu-usage--`, `-dsk/total-`, `--paging---`). These have no `/proc` dependency, so they run on any host (including the dev mac). **Get these green before touching collectors.**
3. **CPU collector + render + engine, end-to-end.** Implement `procfs.go` `/proc/stat` reader, `cpu.go`, `render.go` (header + data row + separators), `engine.go` (loop, dt, header once). Run `./rbstat -c 1 3` on a Linux host; eyeball `--total-cpu-usage--` header and three plausible percentage lines summing ~100.
4. **Add rate collectors** disk, net, page, sys (the rest of the default `-cdngy` set). Run bare `./rbstat` and diff visually against real `dstat` side-by-side: column widths, `|` positions, dash bars, colors. Fix the device/iface filters here using a host with multiple disks/ifaces.
5. **Add gauge collectors** mem, swap, load, then time and io. Test `-m`, `-s`, `-l`, `-t`, `-r` individually and combined (`-cdngymslrt`).
6. **CLI polish.** Clustered flags (`-cdn`), `-a`, default-set stderr message, `--nocolor`, `--noheaders`, `--noupdate`, `--version`, `--list`, `-h`. Test clustered parsing and `[delay [count]]` exit-after-count.
7. **Fidelity pass.** Pipe to a file (`./rbstat 1 3 > out.txt`) and confirm color auto-disables (no ANSI in file). Compare header cadence on a short terminal.

**Testing on Linux against real /proc:** unit tests for parsers can use fixture strings (paste real `/proc/stat`/`diskstats`/`net/dev` lines into `*_test.go` testdata) so they run anywhere. Integration: on a Linux box run alongside `dstat` and `vmstat 1`; numbers should track within rounding/timing jitter. For deterministic parser tests, factor parsing to take an `io.Reader`/`[]byte` (not a hardcoded path) so tests inject fixtures — `procfs.go` should expose `parseStat([]byte)`, `parseDiskstats([]byte)`, etc., with the file-reading wrapper thin on top.

---

## 8. Open questions / risks — with decided defaults

| # | Ambiguity | Decision (v1) |
|---|---|---|
| 1 | **`lo` in net total** | Exclude `lo`, `bondN`, `face`, VLANs (`.NNN`) — matches dstat. Decided. |
| 2 | **`pgpgin`/`pgpgout` units** (KiB vs pages) | Raw values are KiB. Report **bytes/s** = `KiB·1024/dt` with byte formatting/coloring. Unambiguous; differs slightly from classic dstat's pages but is more honest and visually identical in formatting. Document in code comment. |
| 3 | **partition vs whole-disk for disk/io total** | Whole disks only via regexp (exclude trailing-digit partitions + virtual `ram/loop/dm-/md/sr/zram/...`). Avoids double counting. Decided. |
| 4 | **guest/guest_nice double counting** | Ignore guest & guest_nice entirely; user/nice already include them. d_total uses the 8 base buckets. Decided. |
| 5 | **Cached vs Cached+SReclaimable for mem `cach`** | Use plain `Cached` (classic dstat). `used = MemTotal−MemFree−Buffers−Cached`. Note in code that some dstat builds add SReclaimable; v1 does not. |
| 6 | **dt source** | Always measured wall-clock (`time.Now`/`Sub`), never the nominal `delay`. Captured once immediately before the tick's reads. Decided. |
| 7 | **First-tick output** | Rate collectors emit zeros on the first line (no prior sample), matching dstat. Gauges emit real values immediately. Decided. |
| 8 | **Counter wrap** | `cur < prev` → 0 for that field (rare on 64-bit; possible on hotplug/reset). Decided. |
| 9 | **Header reprint cadence / terminal height** | Use `TIOCGWINSZ` ioctl for rows; reprint every `rows-2` lines if tty & rows≥6; else print header once. `--noheaders` → header once, no repeat. Decided (scrolling-only; no in-place `--update` in v1). |
| 10 | **Color on non-tty** | Auto-disable when stdout is not a tty, even without `--nocolor`. `--color` force-on is NOT in v1. Decided. |
| 11 | **`time` collector group title** | dstat labels time under `system`-style banner; v1 uses title `time` with bar sized to the formatted timestamp (`%d-%m %H:%M:%S` = 14 chars). Low-risk cosmetic; revisit if side-by-side diff shows mismatch. |
| 12 | **`--noupdate` semantics** | v1 is scrolling-only (one line per interval), which IS noupdate behavior; flag accepted as a confirming no-op. In-place `--update` deferred. Decided. |
| 13 | **Non-Linux build** | `//go:build linux` on collectors; non-Linux build emits a clear runtime error. Decided. |
| 14 | **One stdlib-only tty/ioctl risk** | If raw `syscall.Syscall(SYS_IOCTL,...)` for `TCGETS`/`TIOCGWINSZ` proves fragile across kernels, the single approved dependency is `golang.org/x/sys/unix`. Try stdlib first. |