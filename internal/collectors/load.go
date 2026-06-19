package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// load reports the 1/5/15-minute load averages (a gauge).
type load struct{}

// NewLoad returns the load-average collector (flag -l).
func NewLoad() collect.Collector { return &load{} }

func (l *load) Name() string  { return "load" }
func (l *load) Title() string { return "load avg" }
func (l *load) Fields() []collect.Field {
	return []collect.Field{{Nick: "1m"}, {Nick: "5m"}, {Nick: "15m"}}
}
func (l *load) Type() collect.ColType { return collect.Float }
func (l *load) Width() int            { return 4 }
func (l *load) ColorStep() float64    { return 0.5 } // matches pcp-dstat /etc/pcp/dstat/load; non-zero => no unit char

func (l *load) Collect(now time.Time) ([]float64, error) {
	la, err := collect.ReadLoadavg()
	if err != nil {
		return nil, err
	}
	return []float64{la[0], la[1], la[2]}, nil
}
