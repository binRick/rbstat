# rbstat — agent guide

rbstat is a Go clone of the classic Linux `dstat` resource monitor. It reads
`/proc`, diffs counters, and prints colorized aligned columns matched against
pcp-dstat. See `README.md` for usage and `SPEC.md` for the full design.

## Building and testing

The program is **Linux-only** (it reads `/proc` and uses Linux ioctls). The
development host is macOS, so the loop is: cross-compile locally for a fast
compile check, sync to the Linux test host `mia`, then build and run there
against real `/proc`.

- Fast local compile check: `GOOS=linux GOARCH=amd64 go build -o /tmp/x .`
- Unit tests run anywhere (parsers/formatter take `[]byte`, render has a
  non-linux `termRows` stub): `go test ./internal/...`
- Full loop on mia: `scripts/mia.sh [args]` (rsyncs, builds, runs on mia).
  `BUILD_ONLY=1 scripts/mia.sh` to just compile.

## The reference: pcp-dstat on mia

`mia` (`ssh mia`) runs CentOS Stream 10 with Go at `/usr/local/go/bin` and the
reference `dstat` (pcp-dstat 7.1.5) at `/usr/bin/dstat`. The Python source is
readable there — the formatting/coloring logic is in `dchg`/`fchg`/`cprint`
(`sed -n '1140,1240p;1560,1700p' /usr/bin/dstat`). When changing rendering,
diff rbstat against dstat side by side, ideally concurrently:

```sh
ssh mia 'cd ~/rbstat; dstat -cdny --nocolor 1 6 >/tmp/d & ./rbstat -cdny 1 6 >/tmp/r & wait; paste /tmp/d /tmp/r'
```

Note rbstat's first row is the since-boot average (classic dstat), so align
rbstat line N+1 with dstat line N when comparing steady-state values.

## Deliberate differences from pcp-dstat

Do not "fix" these: since-boot first line (vs blank), `-cdngy` default with no
leading time column, and scrolling-only output (no in-place redraw).

## Maintain scc statistics

On any commit you make, regenerate the `## Code Statistics` section in
`README.md` (the block delimited by `<!-- scc-start -->` and
`<!-- scc-end -->`) using `scc --no-cocomo -f csv`. Stage the README
together with your other changes so the stats stay in sync with the code.
