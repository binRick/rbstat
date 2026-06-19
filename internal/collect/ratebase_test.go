package collect

import (
	"testing"
	"time"
)

var epoch = time.Unix(1_700_000_000, 0)

func TestRatesDtDivision(t *testing.T) {
	var b RateBase
	// First call with no prior sample returns zeros and seeds prev.
	if got := b.Rates(epoch, 100, 200); got[0] != 0 || got[1] != 0 {
		t.Fatalf("first call = %v, want [0 0]", got)
	}
	// 2s later, counters advanced by 20 and 60 -> 10/s and 30/s.
	got := b.Rates(epoch.Add(2*time.Second), 120, 260)
	if got[0] != 10 || got[1] != 30 {
		t.Errorf("rates = %v, want [10 30]", got)
	}
}

func TestRatesWrapYieldsZero(t *testing.T) {
	var b RateBase
	b.Rates(epoch, 500)
	got := b.Rates(epoch.Add(time.Second), 100) // counter reset: cur < prev
	if got[0] != 0 {
		t.Errorf("wrap rate = %v, want 0", got[0])
	}
}

func TestRatesZeroDt(t *testing.T) {
	var b RateBase
	b.Rates(epoch, 100)
	if got := b.Rates(epoch, 200); got[0] != 0 { // dt == 0 guard
		t.Errorf("zero-dt rate = %v, want 0", got[0])
	}
}

func TestPrimeSinceBoot(t *testing.T) {
	var b RateBase
	b.Prime(epoch)                          // boot time
	got := b.Rates(epoch.Add(10*time.Second), 1000) // uptime 10s
	if got[0] != 100 {                       // since-boot avg = cur/uptime
		t.Errorf("since-boot rate = %v, want 100", got[0])
	}
}
