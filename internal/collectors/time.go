package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// timeCol reports the wall-clock time of each sample. The renderer formats the
// value (carried as Unix seconds) using dstat's default time layout.
type timeCol struct{}

// NewTime returns the time collector (flag -t).
func NewTime() collect.Collector { return &timeCol{} }

func (t *timeCol) Name() string  { return "time" }
func (t *timeCol) Title() string { return "system" }
func (t *timeCol) Fields() []collect.Field {
	return []collect.Field{{Nick: "time"}}
}
func (t *timeCol) Type() collect.ColType { return collect.Str }
func (t *timeCol) Width() int            { return 14 } // fits "%d-%m %H:%M:%S"
func (t *timeCol) ColorStep() float64    { return 0 }

func (t *timeCol) Collect(now time.Time) ([]float64, error) {
	return []float64{float64(now.Unix())}, nil
}
