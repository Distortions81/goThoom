package main

import (
	"slices"
	"testing"
)

func TestPictureShiftPreservesWeightedMovementAndUniqueBackgrounds(t *testing.T) {
	pixelCountMu.Lock()
	oldPixels := pixelCountCache
	pixelCountCache = map[uint16]int{1: 3000, 2: 3000, 3: 7000, 4: 200, 5: 200000, 6: 75000, 7: 75000}
	pixelCountMu.Unlock()
	t.Cleanup(func() {
		pixelCountMu.Lock()
		pixelCountCache = oldPixels
		pixelCountMu.Unlock()
	})
	for _, test := range []struct {
		name        string
		prev, cur   []framePicture
		dx, dy      int
		backgrounds []int
		ok          bool
	}{
		{
			name: "duplicate previous matches",
			prev: []framePicture{{PictID: 1}, {PictID: 1}, {PictID: 2, H: 20}},
			cur:  []framePicture{{PictID: 1, H: 5, V: -2}, {PictID: 2, H: 25, V: -2}},
			dx:   5, dy: -2, backgrounds: []int{0, 1}, ok: true,
		},
		{
			name: "negative horizontal movement",
			prev: []framePicture{{PictID: 1, H: 20}},
			cur:  []framePicture{{PictID: 1, H: 15}},
			dx:   -5, backgrounds: []int{0}, ok: true,
		},
		{
			name: "equal distance keeps first current picture",
			prev: []framePicture{{PictID: 1}},
			cur:  []framePicture{{PictID: 1, H: 5}, {PictID: 1, H: -5}},
			dx:   5, backgrounds: []int{0}, ok: true,
		},
		{
			name: "unmatched pictures do not vote",
			prev: []framePicture{{PictID: 1}, {PictID: 3, H: 20}},
			cur:  []framePicture{{PictID: 2, H: 20}, {PictID: 1, H: 5}},
			dx:   5, backgrounds: []int{1}, ok: true,
		},
		{
			name: "moving second choice behind stationary artwork",
			prev: []framePicture{{PictID: 3, H: 100}, {PictID: 1}, {PictID: 2, H: 20}},
			cur:  []framePicture{{PictID: 3, H: 100}, {PictID: 1, H: 5}, {PictID: 2, H: 25}},
			dx:   5, backgrounds: []int{1, 2}, ok: true,
		},
		{
			name: "small artwork excluded from background pinning",
			prev: []framePicture{{PictID: 1}, {PictID: 4, H: 20}},
			cur:  []framePicture{{PictID: 1, H: 5}, {PictID: 4, H: 25}},
			dx:   5, backgrounds: []int{0}, ok: true,
		},
		{
			name: "small artwork still votes",
			prev: []framePicture{{PictID: 4}},
			cur:  []framePicture{{PictID: 4, H: 5}},
			dx:   5, ok: true,
		},
		{
			name: "large artwork weight is capped",
			prev: []framePicture{{PictID: 5}, {PictID: 6}, {PictID: 7}},
			cur:  []framePicture{{PictID: 5, H: -5}, {PictID: 6, H: 5}, {PictID: 7, H: 5}},
			dx:   5, backgrounds: []int{1, 2}, ok: true,
		},
		{
			name: "excluded artwork cannot vote",
			prev: []framePicture{{PictID: 1}, {PictID: 3037, H: 20}},
			cur:  []framePicture{{PictID: 1, H: 5}, {PictID: 3037, H: 20}},
			dx:   5, backgrounds: []int{0}, ok: true,
		},
		{
			name: "no majority",
			prev: []framePicture{{PictID: 1}, {PictID: 2, H: 20}},
			cur:  []framePicture{{PictID: 1, H: 5}, {PictID: 2, H: 15}},
		},
		{
			name: "movement exceeds limit",
			prev: []framePicture{{PictID: 1}}, cur: []framePicture{{PictID: 1, H: 21}},
		},
		{name: "empty scene"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Repeat to exercise reused scratch storage with different scene sizes.
			for range 3 {
				dx, dy, indices, ok := pictureShift(test.prev, test.cur, 20)
				slices.Sort(indices)
				if dx != test.dx || dy != test.dy || ok != test.ok || !slices.Equal(indices, test.backgrounds) {
					t.Fatalf("shift = (%d,%d), backgrounds = %v, ok = %v; want (%d,%d), %v, %v", dx, dy, indices, ok, test.dx, test.dy, test.backgrounds, test.ok)
				}
			}
		})
	}
}

func TestPictureShiftRefreshesWeightsAndIndices(t *testing.T) {
	pixelCountMu.Lock()
	oldPixels := pixelCountCache
	pixelCountCache = map[uint16]int{1: 3000, 2: 0}
	pixelCountMu.Unlock()
	t.Cleanup(func() {
		pixelCountMu.Lock()
		pixelCountCache = oldPixels
		pixelCountMu.Unlock()
	})
	prev := []framePicture{{PictID: 1}, {PictID: 2}}
	cur := []framePicture{{PictID: 1, H: 5}, {PictID: 2, H: -5}, {PictID: 1, H: 10}}
	dx, dy, indices, ok := pictureShift(prev, cur, 20)
	if !ok || dx != 5 || dy != 0 || !slices.Equal(indices, []int{0}) {
		t.Fatalf("initial shift = (%d,%d), %v, %v", dx, dy, indices, ok)
	}
	pixelCountMu.Lock()
	pixelCountCache[1], pixelCountCache[2] = 0, 3000
	pixelCountMu.Unlock()
	dx, dy, indices, ok = pictureShiftInto(prev, cur[:2], 20, indices)
	if !ok || dx != -5 || dy != 0 || !slices.Equal(indices, []int{1}) {
		t.Fatalf("updated shift = (%d,%d), %v, %v", dx, dy, indices, ok)
	}
	// The reused current-picture index must not retain the previous ID 2.
	dx, dy, indices, ok = pictureShiftInto(prev, cur[:1], 20, indices)
	if ok || dx != 0 || dy != 0 || len(indices) != 0 {
		t.Fatalf("unmatched weighted picture voted: (%d,%d), %v, %v", dx, dy, indices, ok)
	}
}
