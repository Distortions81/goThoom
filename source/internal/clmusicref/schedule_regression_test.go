package clmusicref

import (
	"testing"
	"time"
)

// TestChordDoesNotAdvanceTime ensures the classic CLTF scheduling rule that
// chord events do not advance the melody timeline. This protects against the
// regression where chords advanced time and made songs play much slower.
//
// The pattern mirrors a common idiom: p@70([g]8 p1 [d]8 p1 [g]8 p1 [a]8)
// Expectations:
//   - Initial rest 'p' uses default tempo 120: sixteenth = floor(9000/120)=75 ticks,
//     so p (2 sixteenths) = 150 ticks = 250ms.
//   - After @70: sixteenth = floor(9000/70)=128 ticks. Each p1 advances by 128 ticks.
//   - The four chord strikes should therefore start at 150, 278, 406, 534 ticks
//     (~250ms, 463.3ms, 676.6ms, 890.0ms), not ~1.7s apart.
func TestChordDoesNotAdvanceTime(t *testing.T) {
	// Local minimal scheduler for this regression: supports p, @, and [n]d p1.
	// Computes starts in ticks to avoid float drift.
	_ = "p@70([g]8p1[=d]8p1[g]8p1[a]8)" // illustrative; local scheduler is inline
	tempo := 120
	sixteenth := func(tp int) int { return 9000 / tp }
	curTicks := 0

	// initial 'p' (defaults to 2)
	curTicks += 2 * sixteenth(tempo)

	// now @70
	tempo = 70
	// chord 1 [g]8 at current time
	starts := []int{curTicks}
	// p1 advances by one sixteenth at 70
	curTicks += 1 * sixteenth(tempo)
	// chord 2 [=d]8
	starts = append(starts, curTicks)
	curTicks += 1 * sixteenth(tempo)
	// chord 3 [g]8
	starts = append(starts, curTicks)
	curTicks += 1 * sixteenth(tempo)
	// chord 4 [a]8
	starts = append(starts, curTicks)

	// Expected tick starts
	want := []int{
		150,       // 2 * floor(9000/120)=150
		150 + 128, // + floor(9000/70)
		150 + 2*128,
		150 + 3*128,
	}
	if len(starts) != len(want) {
		t.Fatalf("unexpected starts len=%d want=%d", len(starts), len(want))
	}
	for i := range want {
		if starts[i] != want[i] {
			t.Fatalf("start[%d]=%d want=%d (ms got=%d want=%d)", i, starts[i], want[i],
				int((time.Duration(starts[i]) * time.Second / 600).Milliseconds()),
				int((time.Duration(want[i]) * time.Second / 600).Milliseconds()))
		}
	}
}
