package render

import (
	"bytes"
	"testing"

	"github.com/binRick/rbstat/internal/collect"
	"github.com/binRick/rbstat/internal/collectors"
	"github.com/binRick/rbstat/internal/color"
)

func TestRenderLayout(t *testing.T) {
	color.Enabled = false // deterministic, no ANSI

	cols := []collect.Collector{collectors.NewCPU(), collectors.NewDisk(), collectors.NewNet()}

	var buf bytes.Buffer
	r := New(cols, &buf, false, 0, false)
	// 124928 = 122 KiB, 57344 = 56 KiB, 507904 = 496 KiB.
	r.Tick([][]float64{
		{2, 2, 97, 0, 0},
		{0, 124928},
		{57344, 507904},
	})

	want := "----total-usage---- -dsk/total- -net/total-\n" +
		"usr sys idl wai stl| read  writ| recv  send\n" +
		"  2   2  97   0   0|   0   122k|  56k  496k\n"
	if got := buf.String(); got != want {
		t.Errorf("layout mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}
