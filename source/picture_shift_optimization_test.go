package main

import (
	"slices"
	"testing"
)

func TestPictureShiftPreservesWeightedMovementAndUniqueBackgrounds(t *testing.T) {
	pixelCountMu.Lock()
	oldPixels := pixelCountCache
	pixelCountCache = map[uint16]int{1: 3000, 2: 3000, 3: 7000, 4: 200}
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
