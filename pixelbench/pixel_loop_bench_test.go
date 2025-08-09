package pixelbench

import (
	"image"
	"image/color"
	"testing"
)

// BenchmarkDecodeSetRGBA measures the old pixel loop that used SetRGBA.
func BenchmarkDecodeSetRGBA(b *testing.B) {
	const (
		width  = 64
		height = 64
	)
	pixelCount := width * height
	data := make([]byte, pixelCount)
	for i := range data {
		data[i] = byte(i)
	}
	pal := make([]byte, 256*3)
	for i := range pal {
		pal[i] = byte(i)
	}
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		img := image.NewRGBA(image.Rect(0, 0, width+2, height+2))
		for i := 0; i < pixelCount; i++ {
			idx := int(data[i])
			r := pal[idx*3]
			g := pal[idx*3+1]
			bl := pal[idx*3+2]
			a := byte(0x80)
			r = byte(int(r) * int(a) / 255)
			g = byte(int(g) * int(a) / 255)
			bl = byte(int(bl) * int(a) / 255)
			x := i%width + 1
			y := i/width + 1
			img.SetRGBA(x, y, color.RGBA{r, g, bl, a})
		}
	}
}

// BenchmarkDecodeWritePix measures writing pixels directly to the Pix slice.
func BenchmarkDecodeWritePix(b *testing.B) {
	const (
		width  = 64
		height = 64
	)
	pixelCount := width * height
	data := make([]byte, pixelCount)
	for i := range data {
		data[i] = byte(i)
	}
	pal := make([]byte, 256*3)
	for i := range pal {
		pal[i] = byte(i)
	}
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		img := image.NewRGBA(image.Rect(0, 0, width+2, height+2))
		pix := img.Pix
		stride := img.Stride
		for i := 0; i < pixelCount; i++ {
			idx := int(data[i])
			r := pal[idx*3]
			g := pal[idx*3+1]
			bl := pal[idx*3+2]
			a := byte(0x80)
			r = byte(int(r) * int(a) / 255)
			g = byte(int(g) * int(a) / 255)
			bl = byte(int(bl) * int(a) / 255)
			off := (i/width+1)*stride + (i%width+1)*4
			pix[off+0] = r
			pix[off+1] = g
			pix[off+2] = bl
			pix[off+3] = a
		}
	}
}
