package climg

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sync"
)

// denoiseImage softens pixels by blending with neighbours. Pixels with
// similar hue and brightness are blended more strongly while dissimilar
// pixels are blended less. The sharpness parameter controls how quickly the
// blend amount falls off as colours become more different. Only the immediate
// horizontal and vertical neighbours are considered.
func denoiseImage(img *image.RGBA, sharpness, maxPercent float64) {
	rows := img.Bounds().Dy() - 2
	workers := (rows + denoiseRowsPerWorker - 1) / denoiseRowsPerWorker
	if workers > maxDenoiseWorkers {
		workers = maxDenoiseWorkers
	}
	if maxWorkers := runtime.GOMAXPROCS(0); workers > maxWorkers {
		workers = maxWorkers
	}
	denoiseImageWithWorkers(img, sharpness, maxPercent, workers)
}

const (
	denoiseRowsPerWorker = 4
	maxDenoiseWorkers    = 16
)

func denoiseImageWithWorkers(img *image.RGBA, sharpness, maxPercent float64, workers int) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Work on a copy so neighbour checks aren't affected by in-place writes.
	src := getTempRGBA(bounds)
	copy(src.Pix, img.Pix)

	hsvBuf := hsvPool.Get().(*hsvBuffer)
	need := w * h
	if cap(hsvBuf.values) < need {
		hsvBuf.values = make([]hsv, need)
	}
	hsvs := hsvBuf.values[:need]
	for y := 0; y < h; y++ {
		yoff := y * src.Stride
		idx := y * w
		for x := 0; x < w; x++ {
			off := yoff + x*4
			r := float64(src.Pix[off]) / 255
			g := float64(src.Pix[off+1]) / 255
			b := float64(src.Pix[off+2]) / 255
			h, s, v := rgbToHSV(r, g, b)
			hsvs[idx+x] = hsv{h: h, s: s, v: v}
		}
	}

	rows := h - 2
	if rows > 0 {
		if workers < 1 {
			workers = 1
		}
		if workers > rows {
			workers = rows
		}
		if workers == 1 {
			denoiseRows(img, src, hsvs, w, 1, h-1, sharpness, maxPercent)
		} else {
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				start := 1 + i*rows/workers
				end := 1 + (i+1)*rows/workers
				wg.Add(1)
				go func(start, end int) {
					defer wg.Done()
					denoiseRows(img, src, hsvs, w, start, end, sharpness, maxPercent)
				}(start, end)
			}
			wg.Wait()
		}
	}
	hsvBuf.values = hsvs[:0]
	hsvPool.Put(hsvBuf)
	putTempRGBA(src)
}

var rgbaPool = sync.Pool{New: func() any { return &image.RGBA{} }}

type hsvBuffer struct{ values []hsv }

var hsvPool = sync.Pool{New: func() any { return &hsvBuffer{} }}

type hsv struct{ h, s, v float64 }

var denoiseNeighbours = [...]image.Point{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

func denoiseRows(img, src *image.RGBA, hsvs []hsv, w, start, end int, sharpness, maxPercent float64) {
	for y := start; y < end; y++ {
		yoff := y * src.Stride
		idx := y * w
		for x := 1; x < w-1; x++ {
			off := yoff + x*4
			c := color.RGBA{src.Pix[off], src.Pix[off+1], src.Pix[off+2], src.Pix[off+3]}
			chsv := hsvs[idx+x]

			// If this pixel is opaque and all direct neighbours are
			// non-opaque, blur it unless it is full black.
			if c.A == 0xFF && (c.R != 0 || c.G != 0 || c.B != 0) {
				isolated := true
				for _, n := range denoiseNeighbours {
					nOff := (y+n.Y)*src.Stride + (x+n.X)*4
					if src.Pix[nOff+3] == 0xFF {
						isolated = false
						break
					}
				}
				if isolated {
					c = mixColour(c, color.RGBA{}, float32(maxPercent))
				}
			}

			for _, n := range denoiseNeighbours {
				nOff := (y+n.Y)*src.Stride + (x+n.X)*4
				nIdx := (y+n.Y)*w + (x + n.X)
				ncol := color.RGBA{src.Pix[nOff], src.Pix[nOff+1], src.Pix[nOff+2], src.Pix[nOff+3]}
				dist := colourDist(c, ncol, chsv, hsvs[nIdx])
				if dist < 1 {
					blend := maxPercent * math.Pow(1-dist, sharpness)
					if blend > 0 {
						c = mixColour(c, ncol, float32(blend))
					}
				}
			}

			dstOff := y*img.Stride + x*4
			img.Pix[dstOff] = c.R
			img.Pix[dstOff+1] = c.G
			img.Pix[dstOff+2] = c.B
			img.Pix[dstOff+3] = c.A
		}
	}
}

func getTempRGBA(bounds image.Rectangle) *image.RGBA {
	img := rgbaPool.Get().(*image.RGBA)
	w, h := bounds.Dx(), bounds.Dy()
	need := w * h * 4
	if cap(img.Pix) < need {
		img.Pix = make([]uint8, need)
	}
	img.Pix = img.Pix[:need]
	img.Stride = w * 4
	img.Rect = bounds
	return img
}

func putTempRGBA(img *image.RGBA) { rgbaPool.Put(img) }

// colourDist returns a normalised distance [0,1] based on hue and brightness
// differences between two colours. Values >= 1 indicate colours that should
// not be blended.
const dt = 0

func colourDist(a, b color.RGBA, ahsv, bhsv hsv) float64 {
	if a.A < 0xFF || b.A < 0xFF ||
		(a.R == dt && a.G == dt && a.B == dt) ||
		(b.R == dt && b.G == dt && b.B == dt) {
		return 2 // sentinel > 1
	}

	dh := math.Abs(ahsv.h - bhsv.h)
	if dh > 180 {
		dh = 360 - dh
	}
	dh /= 360
	dv := math.Abs(ahsv.v - bhsv.v)
	avgSat := (ahsv.s + bhsv.s) / 2

	d := dh*avgSat + dv*(1-avgSat)
	if d > 1 {
		return 1
	}
	return d
}

func rgbToHSV(r, g, b float64) (h, s, v float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	v = max
	d := max - min
	if max != 0 {
		s = d / max
	} else {
		return 0, 0, 0
	}
	if d == 0 {
		return 0, s, v
	}
	switch {
	case r == max:
		h = (g - b) / d
	case g == max:
		h = 2 + (b-r)/d
	default:
		h = 4 + (r-g)/d
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return
}

// mixColour blends two colours together by the provided percentage.
func mixColour(a, b color.RGBA, p float32) color.RGBA {
	inv := 1 - p
	return color.RGBA{
		R: uint8(float32(a.R)*inv + float32(b.R)*p),
		G: uint8(float32(a.G)*inv + float32(b.G)*p),
		B: uint8(float32(a.B)*inv + float32(b.B)*p),
		A: uint8(float32(a.A)*inv + float32(b.A)*p),
	}
}
