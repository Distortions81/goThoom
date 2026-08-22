package main

import "testing"

func BenchmarkPictureShiftDense(b *testing.B) {
	const (
		dx          = 3
		dy          = -2
		numPictures = 512
	)

	prev := make([]framePicture, numPictures)
	cur := make([]framePicture, numPictures)
	for i := 0; i < numPictures; i++ {
		id := uint16(i%128 + 1)
		h := int16((i % 32) * 4)
		v := int16((i / 32) * 4)
		prev[i] = framePicture{PictID: id, H: h, V: v}
		cur[i] = framePicture{PictID: id, H: int16(int(h) + dx), V: int16(int(v) + dy)}
	}

	pixelCountMu.Lock()
	for _, p := range prev {
		pixelCountCache[p.PictID] = 2048
	}
	pixelCountMu.Unlock()

	if sx, sy, _, ok := pictureShift(prev, cur, maxInterpPixels); !ok || sx != dx || sy != dy {
		b.Fatalf("unexpected warm-up shift (%d,%d) ok=%v", sx, sy, ok)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sx, sy, idxs, ok := pictureShift(prev, cur, maxInterpPixels)
		if !ok {
			b.Fatal("expected successful pictureShift")
		}
		if sx != dx || sy != dy {
			b.Fatalf("unexpected shift (%d,%d)", sx, sy)
		}
		if len(idxs) == 0 {
			b.Fatal("expected background indexes")
		}
	}
}
