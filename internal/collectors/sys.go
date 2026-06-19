package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// sys reports interrupts and context switches per second.
type sys struct{ collect.RateBase }

// NewSys returns the system collector (flag -y).
func NewSys() collect.Collector { return &sys{} }

func (s *sys) Name() string  { return "sys" }
func (s *sys) Title() string { return "system" }
func (s *sys) Fields() []collect.Field {
	return []collect.Field{{Nick: "int"}, {Nick: "csw"}}
}
func (s *sys) Type() collect.ColType { return collect.Decimal }
func (s *sys) Width() int            { return 5 }
func (s *sys) ColorStep() float64    { return 0 }

func (s *sys) Collect(now time.Time) ([]float64, error) {
	st, err := collect.ReadStat()
	if err != nil {
		return nil, err
	}
	return s.Rates(now, st.Intr, st.Ctxt), nil
}
