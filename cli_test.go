package main

import "testing"

func names(cfg *config) []string {
	var out []string
	for _, c := range cfg.cols {
		out = append(out, c.Name())
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseActions(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want action
	}{
		{[]string{"--version"}, actionVersion},
		{[]string{"--help"}, actionHelp},
		{[]string{"-h"}, actionHelp},
		{[]string{"--list"}, actionList},
		{[]string{"-c"}, actionRun},
	} {
		_, act, err := parseArgs(tc.args)
		if err != nil {
			t.Errorf("parseArgs(%v) error: %v", tc.args, err)
		}
		if act != tc.want {
			t.Errorf("parseArgs(%v) action = %d, want %d", tc.args, act, tc.want)
		}
	}
}

func TestParseSelection(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{nil, []string{"cpu", "disk", "net", "page", "sys"}},               // default
		{[]string{"-cdn"}, []string{"cpu", "disk", "net"}},                 // clustered
		{[]string{"-c", "-d"}, []string{"cpu", "disk"}},                    // separate
		{[]string{"-cc", "--cpu"}, []string{"cpu"}},                        // dedup
		{[]string{"--mem", "--load"}, []string{"mem", "load"}},             // long
		{[]string{"-a"}, []string{"cpu", "disk", "net", "page", "sys"}},    // all
		{[]string{"-t"}, []string{"time", "cpu", "disk", "net", "page", "sys"}}, // time is not a stat
		{[]string{"-tc"}, []string{"time", "cpu"}},                         // explicit stat: no default
	} {
		cfg, _, err := parseArgs(tc.args)
		if err != nil {
			t.Fatalf("parseArgs(%v) error: %v", tc.args, err)
		}
		if got := names(cfg); !eq(got, tc.want) {
			t.Errorf("parseArgs(%v) cols = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestParsePositionals(t *testing.T) {
	cfg, _, err := parseArgs([]string{"-c", "2", "5"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if cfg.delay.Seconds() != 2 || cfg.count != 5 {
		t.Errorf("delay=%v count=%d, want 2s/5", cfg.delay, cfg.count)
	}
	// "--" forces the rest to be positionals.
	cfg, _, err = parseArgs([]string{"--", "3"})
	if err != nil || cfg.delay.Seconds() != 3 {
		t.Errorf("'-- 3' => delay=%v err=%v, want 3s", cfg.delay, err)
	}
}

func TestParseErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--bogus"},
		{"-z"},
		{"abc"},      // bad delay
		{"1", "-2"},  // negative count parsed as flag cluster? "-2" is a number => count -2 invalid
		{"1", "x"},   // bad count
		{"1", "2", "3"}, // too many
	} {
		if _, _, err := parseArgs(args); err == nil {
			t.Errorf("parseArgs(%v) expected error, got nil", args)
		}
	}
}
