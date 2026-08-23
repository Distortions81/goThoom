package climg

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"testing"
)

func TestDenoiseBlendsConfirmedCheckerboard(t *testing.T) {
	a := color.RGBA{R: 120, G: 80, B: 40, A: 0xff}
	b := color.RGBA{R: 160, G: 120, B: 80, A: 0xff}
	img := image.NewRGBA(image.Rect(0, 0, 9, 9))
	for y := 0; y < 9; y++ {
		for x := 0; x < 9; x++ {
			c := a
			if (x+y)&1 != 0 {
				c = b
			}
			img.SetRGBA(x, y, c)
		}
	}

	denoiseImageWithWorkers(img, 0, 0.5, 1)
	if got := img.RGBAAt(4, 4); got == a {
		t.Fatalf("checkerboard center was not filtered: %#v", got)
	}
}

func TestDenoiseBlendsIrregularPaletteTexture(t *testing.T) {
	a := color.RGBA{R: 100, G: 80, B: 55, A: 0xff}
	b := color.RGBA{R: 145, G: 115, B: 75, A: 0xff}
	img := image.NewRGBA(image.Rect(0, 0, 13, 13))
	for y := 0; y < 13; y++ {
		for x := 0; x < 13; x++ {
			c := a
			// Deliberately non-periodic two-colour texture.
			if (x*x+3*y*y+5*x*y+7*x+11*y)%17 < 8 {
				c = b
			}
			img.SetRGBA(x, y, c)
		}
	}
	before := append([]byte(nil), img.Pix...)

	denoiseImageWithWorkers(img, 10, 0.35, 1)
	changed := 0
	for i := range img.Pix {
		if img.Pix[i] != before[i] {
			changed++
		}
	}
	if changed < 30 {
		t.Fatalf("irregular palette texture changed only %d channels; filtering is too weak", changed)
	}
}

func TestDenoisePreservesPixelArtDetails(t *testing.T) {
	background := color.RGBA{R: 70, G: 90, B: 110, A: 0xff}
	detail := color.RGBA{R: 210, G: 175, B: 55, A: 0xff}
	img := image.NewRGBA(image.Rect(0, 0, 11, 11))
	for y := 0; y < 11; y++ {
		for x := 0; x < 11; x++ {
			img.SetRGBA(x, y, background)
		}
	}
	for y := 2; y < 9; y++ {
		img.SetRGBA(5, y, detail)
	}
	img.SetRGBA(8, 8, detail)
	want := append([]byte(nil), img.Pix...)

	denoiseImageWithWorkers(img, 0, 0.5, 1)
	if !bytes.Equal(img.Pix, want) {
		for i := range img.Pix {
			if img.Pix[i] != want[i] {
				t.Fatalf("denoise changed a one-pixel line or isolated detail at (%d,%d): got %v want %v", (i/4)%11, (i/4)/11, img.Pix[i], want[i])
			}
		}
	}
}

func TestDenoisePreservesTransparencyEdges(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 9, 9))
	detail := color.RGBA{R: 140, G: 100, B: 60, A: 0xff}
	img.SetRGBA(4, 4, detail)
	want := append([]byte(nil), img.Pix...)

	denoiseImageWithWorkers(img, 0, 0.5, 1)
	if !bytes.Equal(img.Pix, want) {
		t.Fatal("denoise changed artwork at a transparency edge")
	}
}

func TestDenoisePreservesHardColorEdge(t *testing.T) {
	left := color.RGBA{R: 45, G: 70, B: 105, A: 0xff}
	right := color.RGBA{R: 190, G: 145, B: 70, A: 0xff}
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			c := left
			if x >= 5 {
				c = right
			}
			img.SetRGBA(x, y, c)
		}
	}
	want := append([]byte(nil), img.Pix...)

	denoiseImageWithWorkers(img, 0, 0.5, 1)
	if !bytes.Equal(img.Pix, want) {
		t.Fatal("denoise blurred across a hard color edge")
	}
}

func TestDenoisePreservesDiagonalColorEdge(t *testing.T) {
	a := color.RGBA{R: 40, G: 110, B: 80, A: 0xff}
	b := color.RGBA{R: 180, G: 85, B: 145, A: 0xff}
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 12; x++ {
			c := a
			if x > y {
				c = b
			}
			img.SetRGBA(x, y, c)
		}
	}
	want := append([]byte(nil), img.Pix...)

	denoiseImageWithWorkers(img, 0, 0.5, 1)
	if !bytes.Equal(img.Pix, want) {
		t.Fatal("denoise blurred across a diagonal color edge")
	}
}

func TestDenoiseParallelMatchesSerial(t *testing.T) {
	serial := denoiseTestImage(64)
	parallel := image.NewRGBA(serial.Bounds())
	copy(parallel.Pix, serial.Pix)

	denoiseImageWithWorkers(serial, 2, 0.5, 1)
	denoiseImageWithWorkers(parallel, 2, 0.5, 16)
	if !bytes.Equal(serial.Pix, parallel.Pix) {
		t.Fatal("parallel denoise output differs from serial output")
	}
}

func BenchmarkDenoiseImage(b *testing.B) {
	benchmarkDenoiseImage(b, 64)
}

func BenchmarkDenoiseImage256(b *testing.B) {
	benchmarkDenoiseImage(b, 256)
}

func BenchmarkDenoiseImageWorkers(b *testing.B) {
	for _, size := range []int{64, 256} {
		for _, workers := range []int{1, 2, 4, 8, 16, 32} {
			b.Run(fmt.Sprintf("%dx%d/%d", size, size, workers), func(b *testing.B) {
				src := denoiseTestImage(size)
				rect := src.Bounds()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					img := image.NewRGBA(rect)
					copy(img.Pix, src.Pix)
					denoiseImageWithWorkers(img, 2, 0.5, workers)
				}
			})
		}
	}
}

func benchmarkDenoiseImage(b *testing.B, size int) {
	src := denoiseTestImage(size)
	rect := src.Bounds()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		img := image.NewRGBA(rect)
		copy(img.Pix, src.Pix)
		denoiseImage(img, 2, 0.5)
	}
}

func denoiseTestImage(size int) *image.RGBA {
	src := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			off := (y*size + x) * 4
			src.Pix[off] = uint8(x * 4)
			src.Pix[off+1] = uint8(y * 4)
			src.Pix[off+2] = uint8((x + y) * 2)
			src.Pix[off+3] = 0xFF
		}
	}
	return src
}
