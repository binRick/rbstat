package collect

import "time"

// RateBase centralizes counter-diffing for rate collectors. Embed it and call
// Rates(now, cur...) each tick with the current cumulative counters; it stores
// the previous sample and divides deltas by the measured wall-clock elapsed
// time. Counter wrap (cur < prev) yields 0 for that counter rather than a spike.
type RateBase struct {
	prev   []uint64
	prevAt time.Time
	have   bool
}

// Prime seeds the previous sample as all-zero at the given boot time, so the
// first Rates call returns the average since boot (cur / uptime).
func (b *RateBase) Prime(boot time.Time) {
	b.prev = b.prev[:0]
	b.prevAt = boot
	b.have = true
}

// Rates returns (cur[i]-prev[i]) / dt for each counter.
func (b *RateBase) Rates(now time.Time, cur ...uint64) []float64 {
	out := make([]float64, len(cur))
	if b.have {
		if dt := now.Sub(b.prevAt).Seconds(); dt > 0 {
			for i := range cur {
				var p uint64
				if i < len(b.prev) {
					p = b.prev[i]
				}
				if cur[i] >= p {
					out[i] = float64(cur[i]-p) / dt
				}
			}
		}
	}
	b.prev = append(b.prev[:0], cur...)
	b.prevAt = now
	b.have = true
	return out
}
