package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// mem reports memory usage (a gauge, in bytes).
type mem struct{}

// NewMem returns the memory collector (flag -m).
func NewMem() collect.Collector { return &mem{} }

func (m *mem) Name() string  { return "mem" }
func (m *mem) Title() string { return "memory usage" }
func (m *mem) Fields() []collect.Field {
	return []collect.Field{{Nick: "used"}, {Nick: "free"}, {Nick: "buf"}, {Nick: "cach"}}
}
func (m *mem) Type() collect.ColType { return collect.Byte }
func (m *mem) Width() int            { return 5 }
func (m *mem) ColorStep() float64    { return 0 }

func (m *mem) Collect(now time.Time) ([]float64, error) {
	mi, err := collect.ReadMeminfo()
	if err != nil {
		return nil, err
	}
	total, free, buf, cach := mi["MemTotal"], mi["MemFree"], mi["Buffers"], mi["Cached"]
	// "used" matches procps free/top: reclaimable slab is excluded (it's
	// effectively available) but Shmem is added back (it lives inside Cached
	// yet is genuinely used). The displayed cach column stays plain Cached, so
	// the four columns do not sum to total by design.
	used := int64(total) + int64(mi["Shmem"]) -
		int64(free) - int64(buf) - int64(cach) - int64(mi["SReclaimable"])
	if used < 0 {
		used = 0
	}
	const kB = 1024
	return []float64{
		float64(used) * kB,
		float64(free) * kB,
		float64(buf) * kB,
		float64(cach) * kB,
	}, nil
}
