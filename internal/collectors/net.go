package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// net reports total network throughput in bytes/s.
type net struct{ collect.RateBase }

// NewNet returns the net collector (flag -n).
func NewNet() collect.Collector { return &net{} }

func (n *net) Name() string  { return "net" }
func (n *net) Title() string { return "net/total" }
func (n *net) Fields() []collect.Field {
	return []collect.Field{{Nick: "recv"}, {Nick: "send"}}
}
func (n *net) Type() collect.ColType { return collect.Byte }
func (n *net) Width() int            { return 5 }
func (n *net) ColorStep() float64    { return 0 }

func (n *net) Collect(now time.Time) ([]float64, error) {
	t, err := collect.ReadNetDev()
	if err != nil {
		return nil, err
	}
	return n.Rates(now, t.RecvBytes, t.SendBytes), nil
}
