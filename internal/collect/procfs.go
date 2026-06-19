package collect

import (
	"bufio"
	"bytes"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// --- per-tick file cache -------------------------------------------------
//
// Several collectors read the same /proc file (cpu+sys read /proc/stat;
// disk+io read /proc/diskstats). Caching per tick keeps their views
// consistent and avoids redundant reads. Collection is single-threaded, so a
// plain map is fine. The engine calls BeginTick() once before each sample.

var tickGen uint64

type cachedFile struct {
	gen  uint64
	data []byte
	err  error
}

var fileCache = map[string]cachedFile{}

// BeginTick invalidates the per-tick /proc cache.
func BeginTick() { tickGen++ }

func readProc(path string) ([]byte, error) {
	if c, ok := fileCache[path]; ok && c.gen == tickGen {
		return c.data, c.err
	}
	d, err := os.ReadFile(path)
	fileCache[path] = cachedFile{tickGen, d, err}
	return d, err
}

// --- /proc/stat ----------------------------------------------------------

// Stat holds the fields rbstat needs from /proc/stat.
type Stat struct {
	CPU  [8]uint64 // user, nice, system, idle, iowait, irq, softirq, steal
	Intr uint64    // total interrupts
	Ctxt uint64    // context switches
}

// ParseStat parses /proc/stat content.
func ParseStat(data []byte) Stat {
	var s Stat
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Bytes()
		switch {
		case bytes.HasPrefix(line, []byte("cpu ")):
			f := bytes.Fields(line)
			for i := 0; i < 8 && i+1 < len(f); i++ {
				s.CPU[i], _ = strconv.ParseUint(string(f[i+1]), 10, 64)
			}
		case bytes.HasPrefix(line, []byte("intr ")):
			if f := bytes.Fields(line); len(f) > 1 {
				s.Intr, _ = strconv.ParseUint(string(f[1]), 10, 64)
			}
		case bytes.HasPrefix(line, []byte("ctxt ")):
			if f := bytes.Fields(line); len(f) > 1 {
				s.Ctxt, _ = strconv.ParseUint(string(f[1]), 10, 64)
			}
		}
	}
	return s
}

// ReadStat returns the cached /proc/stat for this tick.
func ReadStat() (Stat, error) {
	d, err := readProc("/proc/stat")
	if err != nil {
		return Stat{}, err
	}
	return ParseStat(d), nil
}

// --- /proc/diskstats -----------------------------------------------------

// DiskTotals aggregates cumulative counters across whole disks.
type DiskTotals struct {
	SectorsRead    uint64
	SectorsWritten uint64
	ReadsDone      uint64
	WritesDone     uint64
}

// A sector in /proc/diskstats is always 512 bytes.
const sectorSize = 512

var (
	reDiskVirtual = regexp.MustCompile(`^(ram|loop|fd|dm-|md|sr|zram|drbd|nbd)`)
	reDiskPart    = regexp.MustCompile(`(?:(?:sd|hd|vd)[a-z]+[0-9]+$)|(?:[0-9]+p[0-9]+$)`)
)

// isWholeDisk reports whether a /proc/diskstats device name is a whole disk
// (not a partition, and not a virtual/pseudo device), matching dstat's notion
// of "total" so partitions are not double-counted.
func isWholeDisk(name string) bool {
	if reDiskVirtual.MatchString(name) {
		return false
	}
	if reDiskPart.MatchString(name) {
		return false
	}
	return true
}

// ParseDiskstats parses /proc/diskstats content, summing whole disks.
func ParseDiskstats(data []byte) DiskTotals {
	var t DiskTotals
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		f := bytes.Fields(sc.Bytes())
		if len(f) < 10 {
			continue
		}
		if !isWholeDisk(string(f[2])) {
			continue
		}
		rd, _ := strconv.ParseUint(string(f[3]), 10, 64) // reads completed
		sr, _ := strconv.ParseUint(string(f[5]), 10, 64) // sectors read
		wr, _ := strconv.ParseUint(string(f[7]), 10, 64) // writes completed
		sw, _ := strconv.ParseUint(string(f[9]), 10, 64) // sectors written
		t.ReadsDone += rd
		t.SectorsRead += sr
		t.WritesDone += wr
		t.SectorsWritten += sw
	}
	return t
}

// ReadDiskstats returns the cached /proc/diskstats for this tick.
func ReadDiskstats() (DiskTotals, error) {
	d, err := readProc("/proc/diskstats")
	if err != nil {
		return DiskTotals{}, err
	}
	return ParseDiskstats(d), nil
}

// SectorsToBytes converts a sector count to bytes.
func SectorsToBytes(sectors uint64) uint64 { return sectors * sectorSize }

// --- /proc/net/dev -------------------------------------------------------

// NetTotals aggregates cumulative recv/send bytes across interfaces.
type NetTotals struct {
	RecvBytes uint64
	SendBytes uint64
}

// Exclude loopback, bonded/teamed slaves and VLAN sub-interfaces from "total",
// matching dstat's net/total cullinsts (^(?:lo|bond\d+|team\d+|face|.+\.\d+)$).
var reNetExclude = regexp.MustCompile(`^(lo|bond[0-9]+|team[0-9]+|face|.+\.[0-9]+)$`)

// ParseNetDev parses /proc/net/dev content, summing included interfaces.
func ParseNetDev(data []byte) NetTotals {
	var t NetTotals
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		idx := strings.IndexByte(line, ':')
		if idx < 0 {
			continue // header lines have no colon
		}
		name := strings.TrimSpace(line[:idx])
		if reNetExclude.MatchString(name) {
			continue
		}
		f := strings.Fields(line[idx+1:])
		if len(f) < 9 {
			continue
		}
		rb, _ := strconv.ParseUint(f[0], 10, 64) // recv bytes
		sb, _ := strconv.ParseUint(f[8], 10, 64) // transmit bytes
		t.RecvBytes += rb
		t.SendBytes += sb
	}
	return t
}

// ReadNetDev returns the cached /proc/net/dev for this tick.
func ReadNetDev() (NetTotals, error) {
	d, err := readProc("/proc/net/dev")
	if err != nil {
		return NetTotals{}, err
	}
	return ParseNetDev(d), nil
}

// --- key/value files (/proc/meminfo, /proc/vmstat) -----------------------

// ParseKV parses "key: value" / "key value" files into a map. Trailing units
// (e.g. "kB" in meminfo) are ignored; values are taken from the second field.
func ParseKV(data []byte) map[string]uint64 {
	m := make(map[string]uint64)
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		f := bytes.Fields(sc.Bytes())
		if len(f) < 2 {
			continue
		}
		key := string(bytes.TrimSuffix(f[0], []byte(":")))
		v, _ := strconv.ParseUint(string(f[1]), 10, 64)
		m[key] = v
	}
	return m
}

// ReadMeminfo returns the cached /proc/meminfo (values in kB) for this tick.
func ReadMeminfo() (map[string]uint64, error) {
	d, err := readProc("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	return ParseKV(d), nil
}

// ReadVmstat returns the cached /proc/vmstat for this tick.
func ReadVmstat() (map[string]uint64, error) {
	d, err := readProc("/proc/vmstat")
	if err != nil {
		return nil, err
	}
	return ParseKV(d), nil
}

// --- /proc/loadavg, /proc/uptime ----------------------------------------

// ParseLoadavg parses the 1/5/15-minute load averages.
func ParseLoadavg(data []byte) [3]float64 {
	var l [3]float64
	f := strings.Fields(string(data))
	for i := 0; i < 3 && i < len(f); i++ {
		l[i], _ = strconv.ParseFloat(f[i], 64)
	}
	return l
}

// ReadLoadavg returns the current load averages.
func ReadLoadavg() ([3]float64, error) {
	d, err := readProc("/proc/loadavg")
	if err != nil {
		return [3]float64{}, err
	}
	return ParseLoadavg(d), nil
}

// ReadUptime returns the system uptime in seconds.
func ReadUptime() (float64, error) {
	d, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	f := strings.Fields(string(d))
	if len(f) == 0 {
		return 0, nil
	}
	v, _ := strconv.ParseFloat(f[0], 64)
	return v, nil
}
