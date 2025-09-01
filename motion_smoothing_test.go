//go:build integration
// +build integration

package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestMotionSmoothingFailure(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	moviePath := filepath.Join(filepath.Dir(file), "clmovFiles", "2025a.clMov")
	resetState()
	initFont()
	frames, err := parseMovie(moviePath, 200)
	if err != nil {
		t.Fatalf("parseMovie: %v", err)
	}
	parsed := 0
	for _, f := range frames {
		if _, _, err := parseDrawState(f.data, false); err != nil {
			continue
		}
		parsed++
		if parsed == 2 {
			break
		}
	}
	if parsed < 2 {
		t.Fatalf("parsed %d frames", parsed)
	}
	if dx, dy, _, ok := pictureShift(state.prevPictures, state.pictures, maxInterpPixels); ok {
		t.Fatalf("pictureShift succeeded unexpectedly: (%d,%d)", dx, dy)
	}
}

func TestPictureShiftWithStaticPictures(t *testing.T) {
	gs.NoCaching = false

	prev := []framePicture{
		{PictID: 1, H: 0, V: 0},
		{PictID: 2, H: 10, V: 10},
		{PictID: 3, H: 20, V: 20},
		{PictID: 4, H: 30, V: 30},
	}
	const dx, dy = 5, 7
	cur := []framePicture{
		{PictID: 1, H: 0 + dx, V: 0 + dy},
		{PictID: 2, H: 10 + dx, V: 10 + dy},
		{PictID: 3, H: 20 + dx, V: 20 + dy},
		{PictID: 4, H: 30, V: 30}, // stationary heavy sprite
	}

	tests := []struct {
		name    string
		weights map[uint16]int
	}{
		{
			name: "weightCap",
			// Moving pictures outweigh the capped static sprite.
			weights: map[uint16]int{1: 5000, 2: 5000, 3: 5000, 4: 200000},
		},
		{
			name: "countFallback",
			// Static sprite remains heavier, rely on count fallback.
			weights: map[uint16]int{1: 10, 2: 10, 3: 10, 4: 200000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pixelCountMu.Lock()
			pixelCountCache = make(map[uint16]int)
			for id, w := range tt.weights {
				pixelCountCache[id] = w
			}
			pixelCountMu.Unlock()

			dxGot, dyGot, _, ok := pictureShift(prev, cur, maxInterpPixels)
			if !ok || dxGot != dx || dyGot != dy {
				t.Fatalf("pictureShift = (%d,%d) ok=%v", dxGot, dyGot, ok)
			}
		})
	}
}
