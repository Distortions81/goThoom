//go:build !nodenoise

package climg

import (
	"crypto/sha256"
	"fmt"
	"image"
	"testing"
)

func TestDenoiseArtworkOutputRegression(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			offset := img.PixOffset(x, y)
			if (x*x+3*y*y+5*x*y+7*x+11*y)%19 < 9 {
				img.Pix[offset] = 112
				img.Pix[offset+1] = 78
				img.Pix[offset+2] = 46
			} else {
				img.Pix[offset] = 154
				img.Pix[offset+1] = 116
				img.Pix[offset+2] = 72
			}
			img.Pix[offset+3] = 0xff
		}
	}
	DenoiseRGBASerial(img, 10, 0.35)
	got := fmt.Sprintf("%x", sha256.Sum256(img.Pix))
	const want = "4abe3407608dc231a000d44c2d4ad6092c3a6e3720d430aab3aa6f82375defdb"
	if got != want {
		t.Fatalf("denoise output hash = %s, want %s", got, want)
	}
}
