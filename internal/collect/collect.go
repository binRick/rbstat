// Package collect defines the Collector contract every rbstat stat group
// implements, plus shared /proc parsing helpers.
package collect

import "time"

// ColType selects how a group's values are formatted and colored. It mirrors
// pcp-dstat's per-column "printtype".
type ColType int

const (
	Percent ColType = iota // 'p' — cpu: integer, colored by magnitude bucket
	Byte                   // 'b' — disk/net/page/mem/swap: base-1024, B/k/M/G suffix
	Decimal                // 'd' — sys: base-1000, integer, k/M suffix
	Float                  // 'f' — load/io: base-1000, decimals allowed
	Str                    // 's' — time: left-justified text
)

// Base returns the divisor used when scaling a value into a unit suffix.
func (t ColType) Base() float64 {
	if t == Byte {
		return 1024
	}
	return 1000
}

// Field is one sub-column within a group.
type Field struct{ Nick string }

// Collector is implemented by every stat group (cpu, disk, net, ...).
type Collector interface {
	// Name is the stable identifier matching the long flag (e.g. "cpu").
	Name() string
	// Title is the group banner text before dash-padding (e.g. "dsk/total").
	Title() string
	// Fields lists the sub-columns in display order.
	Fields() []Field
	// Type selects the formatter/color behavior for the whole group.
	Type() ColType
	// Width is the per-sub-column field width (3 for cpu, 4 for load, 5 default).
	Width() int
	// ColorStep returns the bucket size for value coloring. 0 means "color by
	// order of magnitude"; a non-zero value colors by int(value/step) and also
	// disables the reserved unit character (mirrors pcp-dstat's colorstep).
	ColorStep() float64
	// Collect reads /proc for this tick and returns one float64 per Field.
	Collect(now time.Time) ([]float64, error)
}

// Primer is implemented by rate collectors that can be seeded with the system
// boot time so their first emitted sample reads as an average since boot.
type Primer interface {
	Prime(boot time.Time)
}
