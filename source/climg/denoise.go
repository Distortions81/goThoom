//go:build !nodenoise

package climg

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sync"
)

// denoiseImage applies an edge-directed bilateral filter to palette artwork.
// Nearby, perceptually similar colours can blend even when the dither is
// irregular, while alpha boundaries, strong corners, isolated pixels, and
// one-pixel lines are protected.
func denoiseImage(img *image.RGBA, sharpness, maxPercent float64) {
	rows := img.Bounds().Dy() - 4
	workers := (rows + denoiseRowsPerWorker - 1) / denoiseRowsPerWorker
	if workers > maxDenoiseWorkers {
		workers = maxDenoiseWorkers
	}
	if maxWorkers := runtime.GOMAXPROCS(0); workers > maxWorkers {
		workers = maxWorkers
	}
	denoiseImageWithWorkers(img, sharpness, maxPercent, workers)
}

// DenoiseRGBA applies the same palette-dithering blend used by decoded game
// artwork. It is exported for deterministic preview and diagnostic tools.
func DenoiseRGBA(img *image.RGBA, sharpness, maxPercent float64) {
	denoiseImage(img, sharpness, maxPercent)
}

// DenoiseRGBASerial applies denoise without starting another worker group.
// It is intended for callers that already distribute independent images over
// a shared worker pool.
func DenoiseRGBASerial(img *image.RGBA, sharpness, maxPercent float64) {
	denoiseImageWithWorkers(img, sharpness, maxPercent, 1)
}

// DenoiseRGBAWorkers applies denoise with an explicit worker ceiling. It lets
// callers that already distribute independent images divide the available CPU
// budget between outer image jobs and inner row jobs without oversubscribing.
func DenoiseRGBAWorkers(img *image.RGBA, sharpness, maxPercent float64, workers int) {
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
			// Transparent and black pixels are rejected by colourDist before
			// their HSV value is observed. Avoid the relatively expensive colour
			// conversion across the empty area surrounding most sprite poses.
			if src.Pix[off+3] != 0xff || (src.Pix[off] == 0 && src.Pix[off+1] == 0 && src.Pix[off+2] == 0) {
				hsvs[idx+x] = hsv{}
				continue
			}
			r := float64(src.Pix[off]) / 255
			g := float64(src.Pix[off+1]) / 255
			b := float64(src.Pix[off+2]) / 255
			h, s, v := rgbToHSV(r, g, b)
			hsvs[idx+x] = hsv{h: h, s: s, v: v}
		}
	}

	rows := h - 4
	if rows > 0 {
		if workers < 1 {
			workers = 1
		}
		if workers > rows {
			workers = rows
		}
		if workers == 1 {
			denoiseRows(img, src, hsvs, w, 2, h-2, sharpness, maxPercent)
		} else {
			var wg sync.WaitGroup
			for i := 0; i < workers; i++ {
				start := 2 + i*rows/workers
				end := 2 + (i+1)*rows/workers
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

const denoiseLUTSize = 256

type denoiseRangeWeightEntry struct {
	key   uint64
	value float64
}

type denoiseRangeWeightLUT [denoiseLUTSize]denoiseRangeWeightEntry

func denoiseLUTIndex(key uint64) uint64 {
	key ^= key >> 33
	key *= 0xff51afd7ed558ccd
	return key & (denoiseLUTSize - 1)
}

func (lut *denoiseRangeWeightLUT) weight(distance, sharpness float64) float64 {
	key := math.Float64bits(distance)
	index := denoiseLUTIndex(key)
	entry := &lut[index]
	if entry.key == key && entry.value != 0 {
		return entry.value
	}
	return lut.missedWeight(index, key, distance, sharpness)
}

func (lut *denoiseRangeWeightLUT) missedWeight(index, key uint64, distance, sharpness float64) float64 {
	value := math.Pow(1-distance, sharpness)
	lut[index] = denoiseRangeWeightEntry{key: key, value: value}
	return value
}

func denoiseRows(img, src *image.RGBA, hsvs []hsv, w, start, end int, sharpness, maxPercent float64) {
	if maxPercent <= 0 {
		return
	}
	if maxPercent > 0.5 {
		maxPercent = 0.5
	}
	if sharpness < 0 {
		sharpness = 0
	}
	var rangeWeightLUT denoiseRangeWeightLUT
	for y := start; y < end; y++ {
		yoff := y * src.Stride
		idx := y * w
		for x := 2; x < w-2; x++ {
			off := yoff + x*4
			c := color.RGBA{src.Pix[off], src.Pix[off+1], src.Pix[off+2], src.Pix[off+3]}
			if c.A != 0xFF || (c.R == 0 && c.G == 0 && c.B == 0) {
				continue
			}
			centerIdx := idx + x
			chsv := hsvs[centerIdx]

			left, leftIdx := rgbaAt(src, w, x-1, y)
			right, rightIdx := rgbaAt(src, w, x+1, y)
			top, topIdx := rgbaAt(src, w, x, y-1)
			bottom, bottomIdx := rgbaAt(src, w, x, y+1)
			// Never pull transparent background into a sprite silhouette.
			if left.A != c.A || right.A != c.A || top.A != c.A || bottom.A != c.A {
				continue
			}

			leftDist := colourDist(c, left, chsv, hsvs[leftIdx])
			rightDist := colourDist(c, right, chsv, hsvs[rightIdx])
			topDist := colourDist(c, top, chsv, hsvs[topIdx])
			bottomDist := colourDist(c, bottom, chsv, hsvs[bottomIdx])
			const (
				detailThreshold       = 0.08
				strongDetailThreshold = 0.20
			)
			distinctCardinals := 0
			strongDistinctCardinals := 0
			if leftDist > detailThreshold {
				distinctCardinals++
			}
			if rightDist > detailThreshold {
				distinctCardinals++
			}
			if topDist > detailThreshold {
				distinctCardinals++
			}
			if bottomDist > detailThreshold {
				distinctCardinals++
			}
			if leftDist > strongDetailThreshold {
				strongDistinctCardinals++
			}
			if rightDist > strongDetailThreshold {
				strongDistinctCardinals++
			}
			if topDist > strongDetailThreshold {
				strongDistinctCardinals++
			}
			if bottomDist > strongDetailThreshold {
				strongDistinctCardinals++
			}
			// Three or four different sides describes an isolated pixel or the
			// end of a one-pixel stroke, not a broad colour texture.
			if distinctCardinals >= 3 && strongDistinctCardinals >= 3 {
				continue
			}
			verticalLine := topDist < detailThreshold && bottomDist < detailThreshold && leftDist > strongDetailThreshold && rightDist > strongDetailThreshold
			horizontalLine := leftDist < detailThreshold && rightDist < detailThreshold && topDist > strongDetailThreshold && bottomDist > strongDetailThreshold

			edgeX, edgeY, edgeStrength := sobelColorEdge(src, x, y)
			const edgeThreshold = 0.08
			protectedEdge := coherentSobelEdge(src, x, y, edgeX, edgeY, edgeStrength, edgeThreshold)

			totalWeight := 1.0
			rSum := float64(c.R)
			gSum := float64(c.G)
			bSum := float64(c.B)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					if verticalLine && dx != 0 {
						continue
					}
					if horizontalLine && dy != 0 {
						continue
					}
					if protectedEdge {
						// Sobel points across the edge. Keep samples approximately on
						// its tangent, including for diagonal and chromatic edges.
						sampleLength := math.Hypot(float64(dx), float64(dy))
						crossing := math.Abs(float64(dx)*edgeX+float64(dy)*edgeY) / (edgeStrength * sampleLength)
						if crossing > 0.35 {
							continue
						}
					}
					sample, sampleIdx := rgbaAt(src, w, x+dx, y+dy)
					if sample.A != c.A || (sample.R == 0 && sample.G == 0 && sample.B == 0) {
						continue
					}
					dist := colourDist(c, sample, chsv, hsvs[sampleIdx])
					if dist >= strongDetailThreshold {
						continue
					}
					rangeWeight := rangeWeightLUT.weight(dist, sharpness)
					spatialWeight := 1 / float64(1+dx*dx+dy*dy)
					weight := rangeWeight * spatialWeight
					totalWeight += weight
					rSum += float64(sample.R) * weight
					gSum += float64(sample.G) * weight
					bSum += float64(sample.B) * weight
				}
			}
			if totalWeight <= 1 {
				continue
			}
			filtered := color.RGBA{
				R: uint8(math.Round(rSum / totalWeight)),
				G: uint8(math.Round(gSum / totalWeight)),
				B: uint8(math.Round(bSum / totalWeight)),
				A: c.A,
			}
			// The public strength is 0..0.5. Map that to 0..1 here because this
			// symmetric pass applies only once rather than repeatedly per neighbour.
			out := mixColour(c, filtered, float32(maxPercent*2))

			dstOff := y*img.Stride + x*4
			img.Pix[dstOff] = out.R
			img.Pix[dstOff+1] = out.G
			img.Pix[dstOff+2] = out.B
			img.Pix[dstOff+3] = c.A
		}
	}
}

func rgbaAt(img *image.RGBA, width, x, y int) (color.RGBA, int) {
	off := y*img.Stride + x*4
	return color.RGBA{img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3]}, y*width + x
}

// sobelColorEdge returns the normal and strength of the strongest RGB Sobel
// response. Choosing the strongest channel also catches chromatic edges whose
// two sides have nearly identical luminance.
func sobelColorEdge(img *image.RGBA, x, y int) (edgeX, edgeY, strength float64) {
	center := y*img.Stride + x*4
	topLeft := center - img.Stride - 4
	top := center - img.Stride
	topRight := center - img.Stride + 4
	left := center - 4
	right := center + 4
	bottomLeft := center + img.Stride - 4
	bottom := center + img.Stride
	bottomRight := center + img.Stride + 4
	var gx, gy [3]int
	for channel := range 3 {
		gx[channel] = int(img.Pix[topRight+channel]) - int(img.Pix[topLeft+channel]) +
			2*(int(img.Pix[right+channel])-int(img.Pix[left+channel])) +
			int(img.Pix[bottomRight+channel]) - int(img.Pix[bottomLeft+channel])
		gy[channel] = int(img.Pix[bottomLeft+channel]) + 2*int(img.Pix[bottom+channel]) + int(img.Pix[bottomRight+channel]) -
			int(img.Pix[topLeft+channel]) - 2*int(img.Pix[top+channel]) - int(img.Pix[topRight+channel])
	}
	const sobelScale = 1.0 / (4 * 255)
	var strengthSquared float64
	for channel := range 3 {
		x := float64(gx[channel]) * sobelScale
		y := float64(gy[channel]) * sobelScale
		magnitudeSquared := x*x + y*y
		if magnitudeSquared > strengthSquared {
			edgeX = x
			edgeY = y
			strengthSquared = magnitudeSquared
		}
	}
	strength = math.Sqrt(strengthSquared)
	return edgeX, edgeY, strength
}

func coherentSobelEdge(img *image.RGBA, x, y int, edgeX, edgeY, strength, threshold float64) bool {
	if strength <= threshold {
		return false
	}
	// Follow the nearest pixel step along the edge tangent. Real boundaries
	// retain a similar normal there; dither gradients usually change direction.
	tangentX := 0
	tangentY := 0
	if math.Abs(edgeX) >= math.Abs(edgeY) {
		tangentY = 1
	} else {
		tangentX = 1
	}
	for _, sign := range [...]int{-1, 1} {
		nx, ny, neighbourStrength := sobelColorEdge(img, x+tangentX*sign, y+tangentY*sign)
		if neighbourStrength <= threshold {
			continue
		}
		alignment := math.Abs(edgeX*nx+edgeY*ny) / (strength * neighbourStrength)
		if alignment >= 0.8 {
			return true
		}
	}
	return false
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
