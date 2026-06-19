package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// io reports disk I/O request rates (IOPS) over whole disks.
type io struct{ collect.RateBase }

// NewIO returns the I/O-request collector (flag -r).
func NewIO() collect.Collector { return &io{} }

func (i *io) Name() string  { return "io" }
func (i *io) Title() string { return "io/total" }
func (i *io) Fields() []collect.Field {
	return []collect.Field{{Nick: "read"}, {Nick: "writ"}}
}
func (i *io) Type() collect.ColType { return collect.Float }
func (i *io) Width() int            { return 5 }
func (i *io) ColorStep() float64    { return 0 }

func (i *io) Collect(now time.Time) ([]float64, error) {
	t, err := collect.ReadDiskstats()
	if err != nil {
		return nil, err
	}
	return i.Rates(now, t.ReadsDone, t.WritesDone), nil
}
