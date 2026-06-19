// Package color holds rbstat's ANSI theme and the value-coloring rule,
// matching pcp-dstat's default dark-background theme.
package color

import "math"

// Raw ANSI codes (from pcp-dstat's COLOR table).
const (
	cReset    = "\033[0;0m"
	cTitle    = "\033[0;34m"          // darkblue — group dash bars
	cFrame    = "\033[0;34m"          // darkblue — "|" separators
	cSubtitle = "\033[1;34m\033[4m"   // blue + underline — sub-column nicks
	cError    = "\033[1;37m\033[41m"  // white on red — negative values
	cDone     = "\033[1;37m"          // white — cpu at >= 100%
	cText     = "\033[0;37m"          // gray — string (time) values
	cUnit     = "\033[90m"            // darkgray — unit suffixes and zero values
)

// values is THEME['colors_lo']: the bright bucket palette used for live rows.
var values = []string{
	"\033[1;31m", // red
	"\033[1;33m", // yellow
	"\033[1;32m", // green
	"\033[1;34m", // blue
	"\033[1;36m", // cyan
	"\033[1;37m", // white
	"\033[0;31m", // darkred
	"\033[0;32m", // darkgreen
}

// Enabled gates all color output; the engine clears it for --nocolor or a
// non-tty stdout.
var Enabled = true

func code(s string) string {
	if !Enabled {
		return ""
	}
	return s
}

// Theme accessors return the code, or "" when color is disabled.
func Reset() string    { return code(cReset) }
func Title() string    { return code(cTitle) }
func Frame() string    { return code(cFrame) }
func Subtitle() string { return code(cSubtitle) }
func Unit() string     { return code(cUnit) }
func Err() string      { return code(cError) }
func TimeText() string { return code(cText) }

// Value returns the color code for one numeric cell. text is the already
// formatted number, v the raw value, c the magnitude division count,
// colorStep the per-group bucket size (0 = color by magnitude), and percent
// whether the group is a percentage type (for the >=100% "done" highlight).
func Value(text string, v float64, c int, colorStep float64, percent bool) string {
	if !Enabled {
		return ""
	}
	switch {
	case text == "0":
		return cUnit
	case v < 0:
		return cError
	case percent && math.Round(v) >= 100:
		return cDone
	case colorStep != 0:
		return values[int(v/colorStep)%len(values)]
	default:
		if c < 0 {
			c = 0
		}
		return values[c%len(values)]
	}
}
