package collectors

import "testing"

func TestCpuPercent(t *testing.T) {
	// Deltas chosen to sum to 100 jiffies:
	// user10 nice5 system5 idle60 iowait12 irq2 softirq1 steal5.
	prev := [8]uint64{}
	cur := [8]uint64{10, 5, 5, 60, 12, 2, 1, 5}
	got := cpuPercent(prev, cur)
	want := []float64{15, 8, 60, 12, 5} // usr=u+n, sys=s+irq+soft, idl, wai, stl
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %v, want %v (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestCpuPercentWrapGuard(t *testing.T) {
	// Every counter goes backwards -> total 0 -> all zeros (no negatives).
	got := cpuPercent([8]uint64{100, 100, 100, 100, 100, 100, 100, 100}, [8]uint64{})
	for i, v := range got {
		if v != 0 {
			t.Errorf("field %d = %v, want 0", i, v)
		}
	}
}
