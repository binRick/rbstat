package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// swap reports swap usage (a gauge, in bytes).
type swap struct{}

// NewSwap returns the swap collector (flag -s).
func NewSwap() collect.Collector { return &swap{} }

func (s *swap) Name() string  { return "swap" }
func (s *swap) Title() string { return "total" }
func (s *swap) Fields() []collect.Field {
	return []collect.Field{{Nick: "used"}, {Nick: "free"}}
}
func (s *swap) Type() collect.ColType { return collect.Byte }
func (s *swap) Width() int            { return 5 }
func (s *swap) ColorStep() float64    { return 0 }

func (s *swap) Collect(now time.Time) ([]float64, error) {
	mi, err := collect.ReadMeminfo()
	if err != nil {
		return nil, err
	}
	total, free := mi["SwapTotal"], mi["SwapFree"]
	var used uint64
	if total > free {
		used = total - free
	}
	const kB = 1024
	return []float64{float64(used) * kB, float64(free) * kB}, nil
}
