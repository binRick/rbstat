// Package collectors implements one Collector per dstat stat group.
package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// cpu reports total CPU usage as percentages over the interval.
type cpu struct {
	prev [8]uint64
	have bool
}

// NewCPU returns the cpu collector (flag -c).
func NewCPU() collect.Collector { return &cpu{} }

func (c *cpu) Name() string  { return "cpu" }
func (c *cpu) Title() string { return "total usage" }
func (c *cpu) Fields() []collect.Field {
	return []collect.Field{{Nick: "usr"}, {Nick: "sys"}, {Nick: "idl"}, {Nick: "wai"}, {Nick: "stl"}}
}
func (c *cpu) Type() collect.ColType { return collect.Percent }
func (c *cpu) Width() int            { return 3 }
func (c *cpu) ColorStep() float64    { return 34 } // 0-33 red, 34-67 yellow, 68-99 green

// Prime seeds a zero previous sample so the first row is the since-boot average.
func (c *cpu) Prime(boot time.Time) { c.prev = [8]uint64{}; c.have = true }

func (c *cpu) Collect(now time.Time) ([]float64, error) {
	st, err := collect.ReadStat()
	if err != nil {
		return nil, err
	}
	out := make([]float64, 5)
	if c.have {
		out = cpuPercent(c.prev, st.CPU)
	}
	c.prev = st.CPU
	c.have = true
	return out, nil
}

// cpuPercent computes usr/sys/idl/wai/stl percentages from two /proc/stat cpu
// samples. Counters that went backwards contribute 0; a zero total interval
// yields all zeros.
func cpuPercent(prev, cur [8]uint64) []float64 {
	out := make([]float64, 5)
	var d [8]uint64
	var tot uint64
	for i := 0; i < 8; i++ {
		if cur[i] >= prev[i] {
			d[i] = cur[i] - prev[i]
		}
		tot += d[i]
	}
	if tot > 0 {
		pct := func(x uint64) float64 { return 100 * float64(x) / float64(tot) }
		out[0] = pct(d[0] + d[1])        // usr = user + nice
		out[1] = pct(d[2] + d[5] + d[6]) // sys = system + irq + softirq
		out[2] = pct(d[3])               // idl = idle
		out[3] = pct(d[4])               // wai = iowait
		out[4] = pct(d[7])               // stl = steal
	}
	return out
}
