package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// disk reports total disk throughput in bytes/s over whole disks.
type disk struct{ collect.RateBase }

// NewDisk returns the disk collector (flag -d).
func NewDisk() collect.Collector { return &disk{} }

func (d *disk) Name() string  { return "disk" }
func (d *disk) Title() string { return "dsk/total" }
func (d *disk) Fields() []collect.Field {
	return []collect.Field{{Nick: "read"}, {Nick: "writ"}}
}
func (d *disk) Type() collect.ColType { return collect.Byte }
func (d *disk) Width() int            { return 5 }
func (d *disk) ColorStep() float64    { return 0 }

func (d *disk) Collect(now time.Time) ([]float64, error) {
	t, err := collect.ReadDiskstats()
	if err != nil {
		return nil, err
	}
	return d.Rates(now,
		collect.SectorsToBytes(t.SectorsRead),
		collect.SectorsToBytes(t.SectorsWritten)), nil
}
