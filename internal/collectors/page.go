package collectors

import (
	"time"

	"github.com/binRick/rbstat/internal/collect"
)

// page reports paging (swap) activity as a base-1000 decimal rate, matching
// pcp-dstat's [page] plugin: in=swap.pagesin, out=swap.pagesout, which map to
// the kernel pswpin/pswpout counters in /proc/vmstat (pages read from / written
// to swap devices). The PCP metrics carry no printtype/units (dimensionless
// counts), so cprint derives printtype 'd', base 1000, and the space/k/M unit
// table with no 'B' suffix.
type page struct{ collect.RateBase }

// NewPage returns the paging collector (flag -g).
func NewPage() collect.Collector { return &page{} }

func (p *page) Name() string  { return "page" }
func (p *page) Title() string { return "paging" }
func (p *page) Fields() []collect.Field {
	return []collect.Field{{Nick: "in"}, {Nick: "out"}}
}
func (p *page) Type() collect.ColType { return collect.Decimal }
func (p *page) Width() int            { return 5 }
func (p *page) ColorStep() float64    { return 0 }

func (p *page) Collect(now time.Time) ([]float64, error) {
	vm, err := collect.ReadVmstat()
	if err != nil {
		return nil, err
	}
	return p.Rates(now, vm["pswpin"], vm["pswpout"]), nil
}
