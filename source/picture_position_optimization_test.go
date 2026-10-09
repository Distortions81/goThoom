package main

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// Keep the all-pairs implementation as an independent reference for matching
// repeated IDs, retained pictures, and distance ties across scratch reuse.
func referencePicturePositions(prev, cur []framePicture, shiftX, shiftY, max, again int) ([]picturePositionMatch, []int) {
	matches := make([]int, len(cur))
	for i := range matches {
		matches[i] = -1
	}
	used := make([]bool, len(prev))
	for i := 0; i < again; i++ {
		used[i] = true
	}
	var candidates []picturePositionMatch
	for current := again; current < len(cur); current++ {
		for previous := range prev {
			if prev[previous].PictID != cur[current].PictID {
				continue
			}
			dh := int(cur[current].H) - int(prev[previous].H) - shiftX
			dv := int(cur[current].V) - int(prev[previous].V) - shiftY
			distance := dh*dh + dv*dv
			if prev[previous].Again && distance != 0 {
				continue
			}
			if distance <= max*max {
				candidates = append(candidates, picturePositionMatch{current, previous, distance})
			}
		}
	}
	slices.SortFunc(candidates, func(a, b picturePositionMatch) int {
		if a.distance != b.distance {
			return a.distance - b.distance
		}
		if a.current != b.current {
			return a.current - b.current
		}
		return a.previous - b.previous
	})
	for _, candidate := range candidates {
		if matches[candidate.current] < 0 && !used[candidate.previous] {
			matches[candidate.current] = candidate.previous
			used[candidate.previous] = true
		}
	}
	return candidates, matches
}

func TestPicturePositionIndexMatchesAllPairs(t *testing.T) {
	rng := rand.New(rand.NewPCG(873, 413))
	for trial := 0; trial < 500; trial++ {
		prev := make([]framePicture, rng.IntN(160))
		cur := make([]framePicture, rng.IntN(160))
		ids := rng.IntN(16) + 1
		fill := func(pictures []framePicture) {
			for i := range pictures {
				pictures[i] = framePicture{
					PictID: uint16(rng.IntN(ids)),
					H:      int16(rng.IntN(16)-8) * 4,
					V:      int16(rng.IntN(16)-8) * 4,
					Again:  rng.IntN(4) == 0,
				}
			}
		}
		fill(prev)
		fill(cur)
		again := rng.IntN(min(len(prev), len(cur)) + 1)
		copy(cur[:again], prev[:again])
		shiftX, shiftY, distance := rng.IntN(9)-4, rng.IntN(9)-4, rng.IntN(60)
		wantCandidates, wantMatches := referencePicturePositions(prev, cur, shiftX, shiftY, distance, again)
		scratch := matchPicturePositions(prev, cur, shiftX, shiftY, distance, again)
		if !slices.Equal(scratch.candidates, wantCandidates) {
			t.Fatalf("trial %d: candidates differ from all-pairs scan", trial)
		}
		if !slices.Equal(scratch.matches, wantMatches) {
			t.Fatalf("trial %d: matches = %v, want %v", trial, scratch.matches, wantMatches)
		}
		releasePicturePositionScratch(scratch)
	}
}

func BenchmarkPicturePositions(b *testing.B) {
	for _, tc := range []struct {
		name  string
		count int
		ids   int
	}{
		{"Dense", 512, 128},
		{"Repeated", 512, 8},
		{"Homogeneous", 512, 1},
		{"Small", 12, 6},
		{"Medium32", 32, 16},
		{"Medium64", 64, 32},
		{"Medium128", 128, 64},
	} {
		b.Run(tc.name, func(b *testing.B) {
			prev := make([]framePicture, tc.count)
			cur := make([]framePicture, tc.count)
			for i := range prev {
				prev[i] = framePicture{PictID: uint16(i%tc.ids + 1), H: int16((i % 32) * 20), V: int16((i / 32) * 20)}
				cur[i] = prev[i]
				cur[i].H += 3
				cur[i].V -= 2
			}
			scratch := matchPicturePositions(prev, cur, 3, -2, maxInterpPixels, 0)
			for i, previous := range scratch.matches {
				if previous != i {
					b.Fatalf("current picture %d matched previous %d", i, previous)
				}
			}
			releasePicturePositionScratch(scratch)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				releasePicturePositionScratch(matchPicturePositions(prev, cur, 3, -2, maxInterpPixels, 0))
			}
		})
	}
}
