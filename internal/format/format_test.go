package format

import "testing"

func TestCenterDash(t *testing.T) {
	// Verified against pcp-dstat 7.1.5 output on Linux.
	cases := []struct {
		s    string
		w    int
		want string
	}{
		{"total usage", 19, "----total-usage----"},
		{"dsk/total", 11, "-dsk/total-"},
		{"net/total", 11, "-net/total-"},
		{"paging", 11, "---paging--"},   // odd margin → extra dash on the left
		{"system", 11, "---system--"},
		{"memory usage", 23, "------memory-usage-----"},
		{"load avg", 14, "---load-avg---"},
		{"total", 11, "---total---"},
		{"io/total", 11, "--io/total-"},
		{"system", 14, "----system----"},
	}
	for _, c := range cases {
		if got := CenterDash(c.s, c.w); got != c.want {
			t.Errorf("CenterDash(%q,%d) = %q, want %q", c.s, c.w, got, c.want)
		}
	}
}

func TestCenterSubcols(t *testing.T) {
	cases := []struct {
		s    string
		w    int
		want string
	}{
		{"read", 5, " read"},
		{"writ", 5, " writ"},
		{"in", 5, "  in "},  // odd margin → extra space on the left
		{"out", 5, " out "},
		{"int", 5, " int "},
		{"buf", 5, " buf "},
		{"1m", 4, " 1m "},
		{"15m", 4, "15m "},
		{"time", 14, "     time     "},
	}
	for _, c := range cases {
		if got := Center(c.s, c.w); got != c.want {
			t.Errorf("Center(%q,%d) = %q, want %q", c.s, c.w, got, c.want)
		}
	}
}

func TestDchg(t *testing.T) {
	// printtype 'b'/'d'/'p' use dchg: integer only, no decimals.
	cases := []struct {
		v     float64
		base  float64
		width int
		want  string
		wantC int
	}{
		{0, 1024, 4, "0", 0},
		{66, 1024, 4, "66", 0},
		{486, 1024, 4, "486", 0},
		{3023, 1024, 4, "3023", 0},
		{56291, 1024, 4, "55", 1},          // 56291/1024 ≈ 55 -> "55k"
		{10485760, 1024, 4, "10", 2},        // 10 MiB -> "10M"
		{274, 1000, 4, "274", 0},            // sys decimal
		{1845493760, 1024, 4, "1760", 2},    // 1760 MiB -> "1760M"
	}
	for _, c := range cases {
		got, gotC := Dchg(c.v, c.base, c.width)
		if got != c.want || gotC != c.wantC {
			t.Errorf("Dchg(%v,%v,%d) = (%q,%d), want (%q,%d)", c.v, c.base, c.width, got, gotC, c.want, c.wantC)
		}
	}
}

func TestFchg(t *testing.T) {
	// printtype 'f' uses fchg: decimals where they fit.
	cases := []struct {
		v     float64
		base  float64
		width int
		want  string
	}{
		{0, 1000, 4, "0"},
		{0.43, 1000, 4, "0.43"},
		{0.18, 1000, 4, "0.18"},
		{12, 1000, 4, "12.0"},
	}
	for _, c := range cases {
		got, _ := Fchg(c.v, c.base, c.width)
		if got != c.want {
			t.Errorf("Fchg(%v,%v,%d) = %q, want %q", c.v, c.base, c.width, got, c.want)
		}
	}
}
