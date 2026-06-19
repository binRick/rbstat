// Package render builds dstat-style header and data lines: dash-padded group
// bars, colored "|" separators, and width-fitted, colored numeric cells.
package render

import (
	"io"
	"math"
	"strings"
	"time"

	"github.com/binRick/rbstat/internal/collect"
	"github.com/binRick/rbstat/internal/color"
	"github.com/binRick/rbstat/internal/format"
)

// timeLayout matches pcp-dstat's default DSTAT_TIMEFMT "%d-%m %H:%M:%S".
const timeLayout = "02-01 15:04:05"

// Renderer prints the header and data rows for a fixed set of collectors.
type Renderer struct {
	cols        []collect.Collector
	w           io.Writer
	headerEvery int // 0 => print the header once at the top only
	sinceHeader int
	started     bool
}

// New builds a Renderer. When stdout is an interactive terminal at least 6
// rows tall (and headers are not disabled), the header is reprinted every
// rows-2 data lines; otherwise it is printed once.
func New(cols []collect.Collector, w io.Writer, noheaders bool, ttyFd uintptr, isTTY bool) *Renderer {
	r := &Renderer{cols: cols, w: w}
	if !noheaders && isTTY {
		if rows, ok := termRows(ttyFd); ok && rows >= 6 {
			r.headerEvery = rows - 2
		}
	}
	r.started = noheaders // when headers disabled, never print one
	return r
}

// statWidth is a group's total width: sum of sub-column widths plus the single
// spaces joining them.
func statWidth(c collect.Collector) int {
	n := len(c.Fields())
	if n == 0 {
		return 0
	}
	return n*c.Width() + (n - 1)
}

// Tick renders one sample: a header when due, then the data row.
func (r *Renderer) Tick(values [][]float64) {
	if !r.started || (r.headerEvery > 0 && r.sinceHeader >= r.headerEvery) {
		io.WriteString(r.w, r.header())
		r.sinceHeader = 0
		r.started = true
	}
	io.WriteString(r.w, r.dataRow(values)+"\n")
	r.sinceHeader++
}

func (r *Renderer) header() string {
	var b strings.Builder
	// Title row: group bars joined by a frame-colored space.
	for i, c := range r.cols {
		if i > 0 {
			b.WriteString(color.Frame() + " ")
		}
		b.WriteString(color.Title() + format.CenterDash(c.Title(), statWidth(c)) + color.Reset())
	}
	b.WriteByte('\n')
	// Sub-title row: nicks joined by spaces within a group, groups by "|".
	for i, c := range r.cols {
		if i > 0 {
			b.WriteString(color.Frame() + "|")
		}
		for j, f := range c.Fields() {
			if j > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(color.Subtitle() + format.Center(f.Nick, c.Width()) + color.Reset())
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func (r *Renderer) dataRow(values [][]float64) string {
	var b strings.Builder
	for i, c := range r.cols {
		if i > 0 {
			b.WriteString(color.Frame() + "|")
		}
		var vals []float64
		if i < len(values) {
			vals = values[i]
		}
		for j := range c.Fields() {
			if j > 0 {
				b.WriteByte(' ')
			}
			v := math.NaN()
			if j < len(vals) {
				v = vals[j]
			}
			b.WriteString(cell(c, v))
		}
	}
	b.WriteString(color.Reset())
	return b.String()
}

// cell renders one value into its column, matching pcp-dstat's cprint.
func cell(c collect.Collector, v float64) string {
	w := c.Width()
	typ := c.Type()

	if typ == collect.Str {
		s := time.Unix(int64(v), 0).Format(timeLayout)
		if len(s) > w {
			s = s[:w]
		}
		return color.TimeText() + s + strings.Repeat(" ", w-len(s))
	}

	// Missing value (e.g. collector error): blank the full-width column.
	if math.IsNaN(v) {
		return strings.Repeat(" ", w)
	}

	// showunit reserves one character for the unit suffix, only when coloring
	// by magnitude (colorStep == 0) and the column is wide enough.
	colorStep := c.ColorStep()
	showunit := colorStep == 0 && w >= 4
	valWidth := w
	if showunit {
		valWidth = w - 1
	}

	if v < 0 {
		out := color.Err() + rjust("-", valWidth)
		if showunit {
			out += " "
		}
		return out
	}

	var text string
	var div int
	base := typ.Base()
	if typ == collect.Float {
		text, div = format.Fchg(v, base, valWidth)
	} else { // Percent, Byte, Decimal
		text, div = format.Dchg(v, base, valWidth)
	}

	out := color.Value(text, v, div, colorStep, typ == collect.Percent) + rjust(text, valWidth)
	if showunit {
		if math.Round(v) != 0 {
			units := format.DecUnits
			if typ == collect.Byte {
				units = format.ByteUnits
			}
			if div >= len(units) {
				div = len(units) - 1
			}
			out += color.Unit() + units[div]
		} else {
			out += " "
		}
	}
	return out
}

func rjust(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}
