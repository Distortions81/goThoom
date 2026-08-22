package climg

import (
	"bytes"
	"fmt"
	"image"
	"testing"
)

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
