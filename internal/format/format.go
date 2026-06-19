// Package format implements dstat's number and header-label formatting:
// the str.center dash padding and the dchg/fchg width-fitting algorithms.
package format

import (
	"math"
	"strconv"
	"strings"
)

// Unit suffix tables. Byte values (base 1024) start at "B"; decimal/float
// values (base 1000) use a space for the ones bucket.
var (
	ByteUnits = []string{"B", "k", "M", "G", "T", "P", "E", "Z", "Y"}
	DecUnits  = []string{" ", "k", "M", "G", "T", "P", "E", "Z", "Y"}
)

// pad reproduces Python's str.center(width): when the margin is odd the extra
// space goes on the LEFT (the marg//2 + (marg & width & 1) quirk). Strings
// longer than width are truncated.
func pad(s string, w int) string {
	if len(s) > w {
		s = s[:w]
	}
	if len(s) >= w {
		return s
	}
	marg := w - len(s)
	left := marg/2 + (marg & w & 1)
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", marg-left)
}

// Center centers s within width w using Python str.center semantics.
func Center(s string, w int) string { return pad(s, w) }

// CenterDash centers s and replaces every space with a dash (dstat group bars).
func CenterDash(s string, w int) string { return strings.ReplaceAll(pad(s, w), " ", "-") }

// Dchg fits an integer value into width by rounding and scaling down by base
// until it fits, returning the text and the number of divisions performed
// (used to pick the unit suffix and color). Mirrors pcp-dstat's dchg.
func Dchg(v, base float64, width int) (string, int) {
	c := 0
	for {
		ret := strconv.FormatInt(int64(math.Round(v)), 10)
		if len(ret) <= width {
			return ret, c
		}
		v /= base
		c++
	}
}

// Fchg fits a float value into width, adding as many decimal places as fit
// once the integer part fits, scaling down by base otherwise. Mirrors
// pcp-dstat's fchg.
func Fchg(v, base float64, width int) (string, int) {
	c := 0
	for {
		if v == 0 {
			return "0", 0
		}
		intStr := strconv.FormatInt(int64(v), 10) // truncate toward zero
		if len(intStr) <= width {
			for i := width - len(intStr) - 1; i > 0; i-- {
				cand := strconv.FormatFloat(v, 'f', i, 64)
				if len(cand) <= width && cand != intStr {
					return cand, c
				}
			}
			return intStr, c
		}
		v /= base
		c++
	}
}
