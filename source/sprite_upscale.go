package main

import (
	"image"
	"math"
)

type colorThresholds struct {
	hue        float64
	saturation float64
	brightness float64
}

const (
	spriteUpscaleEdgeHueThreshold        = 4.5
	spriteUpscaleEdgeSaturationThreshold = 0.05
	spriteUpscaleEdgeBrightnessThreshold = 0.05

	spriteUpscaleBlendHueThreshold        = 14.0
	spriteUpscaleBlendSaturationThreshold = 0.14
	spriteUpscaleBlendBrightnessThreshold = 0.14

	spriteUpscaleIsolatedCornerWeight = 0.25
	spriteUpscaleIsolatedEdgeWeight   = 0.20

	spriteUpscaleAntiAliasWeight = 1.0
)

var (
	spriteUpscaleEdgeThresholds = colorThresholds{
		hue:        spriteUpscaleEdgeHueThreshold,
		saturation: spriteUpscaleEdgeSaturationThreshold,
		brightness: spriteUpscaleEdgeBrightnessThreshold,
	}
	spriteUpscaleBlendThresholds = colorThresholds{
		hue:        spriteUpscaleBlendHueThreshold,
		saturation: spriteUpscaleBlendSaturationThreshold,
		brightness: spriteUpscaleBlendBrightnessThreshold,
	}
)

type rgbaPixel struct {
	r uint8
	g uint8
	b uint8
	a uint8
}

func sampleRGBA(src *image.RGBA, x, y int) rgbaPixel {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w == 0 || h == 0 {
		return rgbaPixel{}
	}
	if x < 0 {
		x = 0
	} else if x >= w {
		x = w - 1
	}
	if y < 0 {
		y = 0
	} else if y >= h {
		y = h - 1
	}
	ax := b.Min.X + x
	ay := b.Min.Y + y
	i := src.PixOffset(ax, ay)
	pix := src.Pix
	return rgbaPixel{r: pix[i+0], g: pix[i+1], b: pix[i+2], a: pix[i+3]}
}

func setRGBA(dst *image.RGBA, x, y int, c rgbaPixel) {
	b := dst.Bounds()
	ax := b.Min.X + x
	ay := b.Min.Y + y
	i := dst.PixOffset(ax, ay)
	pix := dst.Pix
	pix[i+0] = c.r
	pix[i+1] = c.g
	pix[i+2] = c.b
	pix[i+3] = c.a
}

func colorsSimilarWithThresholds(a, b rgbaPixel, t colorThresholds) bool {
	if a == b {
		return true
	}
	if a.a == 0 && b.a == 0 {
		return true
	}
	if a.a != b.a {
		return false
	}

	ah, as, av := rgbaToHSV(a)
	bh, bs, bv := rgbaToHSV(b)

	if math.Abs(av-bv) > t.brightness {
		return false
	}
	if math.Abs(as-bs) > t.saturation {
		return false
	}

	if as < 1e-6 && bs < 1e-6 {
		return true
	}

	dh := math.Abs(ah - bh)
	if dh > 180 {
		dh = 360 - dh
	}
	return dh <= t.hue
}

func colorsBlendSimilar(a, b rgbaPixel) bool {
	return colorsSimilarWithThresholds(a, b, spriteUpscaleBlendThresholds)
}

func colorsEdgeSimilar(a, b rgbaPixel) bool {
	return colorsSimilarWithThresholds(a, b, spriteUpscaleEdgeThresholds)
}

func colorsSignificantlyDifferent(a, b rgbaPixel) bool {
	return !colorsEdgeSimilar(a, b)
}

func similarColor(a, b rgbaPixel) bool {
	return colorsBlendSimilar(a, b)
}

func rgbaToHSV(p rgbaPixel) (h, s, v float64) {
	r := float64(p.r) / 255
	g := float64(p.g) / 255
	b := float64(p.b) / 255

	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	v = max

	d := max - min
	if max != 0 {
		s = d / max
	}
	if d == 0 {
		return 0, s, v
	}

	switch max {
	case r:
		h = math.Mod((g-b)/d, 6) * 60
	case g:
		h = ((b-r)/d + 2) * 60
	default:
		h = ((r-g)/d + 4) * 60
	}
	if h < 0 {
		h += 360
	}
	return
}

func scale2xRGBA(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w == 0 || h == 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w*2, h*2))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			bPix := sampleRGBA(src, x, y-1)
			d := sampleRGBA(src, x-1, y)
			e := sampleRGBA(src, x, y)
			f := sampleRGBA(src, x+1, y)
			hPix := sampleRGBA(src, x, y+1)

			e0, e1, e2, e3 := e, e, e, e
			verticalDifferent := colorsSignificantlyDifferent(bPix, hPix)
			horizontalDifferent := colorsSignificantlyDifferent(d, f)
			if verticalDifferent && horizontalDifferent {
				if colorsBlendSimilar(d, bPix) {
					e0 = d
				}
				if colorsBlendSimilar(bPix, f) {
					e1 = f
				}
				if colorsBlendSimilar(d, hPix) {
					e2 = d
				}
				if colorsBlendSimilar(hPix, f) {
					e3 = f
				}
			}
			if e0 == e && e1 == e && e2 == e && e3 == e {
				if tl, tr, bl, br, ok := softenIsolated2x(e, bPix, d, f, hPix); ok {
					e0, e1, e2, e3 = tl, tr, bl, br
				}
			}
			setRGBA(dst, x*2+0, y*2+0, e0)
			setRGBA(dst, x*2+1, y*2+0, e1)
			setRGBA(dst, x*2+0, y*2+1, e2)
			setRGBA(dst, x*2+1, y*2+1, e3)
		}
	}
	return dst
}

func scale3xRGBA(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w == 0 || h == 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w*3, h*3))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := sampleRGBA(src, x-1, y-1)
			bPix := sampleRGBA(src, x, y-1)
			c := sampleRGBA(src, x+1, y-1)
			d := sampleRGBA(src, x-1, y)
			e := sampleRGBA(src, x, y)
			f := sampleRGBA(src, x+1, y)
			g := sampleRGBA(src, x-1, y+1)
			hPix := sampleRGBA(src, x, y+1)
			i := sampleRGBA(src, x+1, y+1)

			e0, e1, e2, e3, e4, e5, e6, e7, e8 := e, e, e, e, e, e, e, e, e
			verticalDifferent := colorsSignificantlyDifferent(bPix, hPix)
			horizontalDifferent := colorsSignificantlyDifferent(d, f)
			if verticalDifferent && horizontalDifferent {
				if colorsBlendSimilar(d, bPix) {
					e0 = d
				}
				if (colorsBlendSimilar(d, bPix) && colorsSignificantlyDifferent(e, c)) || (colorsBlendSimilar(bPix, f) && colorsSignificantlyDifferent(e, a)) {
					e1 = bPix
				}
				if colorsBlendSimilar(bPix, f) {
					e2 = f
				}
				if (colorsBlendSimilar(d, bPix) && colorsSignificantlyDifferent(e, g)) || (colorsBlendSimilar(d, hPix) && colorsSignificantlyDifferent(e, a)) {
					e3 = d
				}
				if (colorsBlendSimilar(bPix, f) && colorsSignificantlyDifferent(e, i)) || (colorsBlendSimilar(hPix, f) && colorsSignificantlyDifferent(e, c)) {
					e5 = f
				}
				if colorsBlendSimilar(d, hPix) {
					e6 = d
				}
				if (colorsBlendSimilar(d, hPix) && colorsSignificantlyDifferent(e, i)) || (colorsBlendSimilar(hPix, f) && colorsSignificantlyDifferent(e, g)) {
					e7 = hPix
				}
				if colorsBlendSimilar(hPix, f) {
					e8 = f
				}
			}
			if e0 == e && e1 == e && e2 == e && e3 == e && e4 == e && e5 == e && e6 == e && e7 == e && e8 == e {
				if block, ok := softenIsolated3x(e, a, bPix, c, d, f, g, hPix, i); ok {
					e0, e1, e2 = block[0], block[1], block[2]
					e3, e4, e5 = block[3], block[4], block[5]
					e6, e7, e8 = block[6], block[7], block[8]
				}
			}
			ox := x * 3
			oy := y * 3
			setRGBA(dst, ox+0, oy+0, e0)
			setRGBA(dst, ox+1, oy+0, e1)
			setRGBA(dst, ox+2, oy+0, e2)
			setRGBA(dst, ox+0, oy+1, e3)
			setRGBA(dst, ox+1, oy+1, e4)
			setRGBA(dst, ox+2, oy+1, e5)
			setRGBA(dst, ox+0, oy+2, e6)
			setRGBA(dst, ox+1, oy+2, e7)
			setRGBA(dst, ox+2, oy+2, e8)
		}
	}
	return dst
}

func averagePixels(a, b rgbaPixel) rgbaPixel {
	return rgbaPixel{
		r: uint8((int(a.r) + int(b.r) + 1) / 2),
		g: uint8((int(a.g) + int(b.g) + 1) / 2),
		b: uint8((int(a.b) + int(b.b) + 1) / 2),
		a: uint8((int(a.a) + int(b.a) + 1) / 2),
	}
}

func averageThreePixels(a, b, c rgbaPixel) rgbaPixel {
	return rgbaPixel{
		r: uint8((int(a.r) + int(b.r) + int(c.r) + 1) / 3),
		g: uint8((int(a.g) + int(b.g) + int(c.g) + 1) / 3),
		b: uint8((int(a.b) + int(b.b) + int(c.b) + 1) / 3),
		a: uint8((int(a.a) + int(b.a) + int(c.a) + 1) / 3),
	}
}

func averagePixelsSlice(pixels []rgbaPixel) rgbaPixel {
	if len(pixels) == 0 {
		return rgbaPixel{}
	}
	if len(pixels) == 1 {
		return pixels[0]
	}
	sumR, sumG, sumB, sumA := 0, 0, 0, 0
	for _, p := range pixels {
		sumR += int(p.r)
		sumG += int(p.g)
		sumB += int(p.b)
		sumA += int(p.a)
	}
	count := len(pixels)
	return rgbaPixel{
		r: uint8((sumR + count/2) / count),
		g: uint8((sumG + count/2) / count),
		b: uint8((sumB + count/2) / count),
		a: uint8((sumA + count/2) / count),
	}
}

func blendTowards(src, target rgbaPixel, weight float64) rgbaPixel {
	if weight <= 0 {
		return src
	}
	if weight >= 1 {
		return target
	}
	inv := 1 - weight
	return rgbaPixel{
		r: uint8(math.Round(float64(src.r)*inv + float64(target.r)*weight)),
		g: uint8(math.Round(float64(src.g)*inv + float64(target.g)*weight)),
		b: uint8(math.Round(float64(src.b)*inv + float64(target.b)*weight)),
		a: uint8(math.Round(float64(src.a)*inv + float64(target.a)*weight)),
	}
}

func shouldSmoothIsolated(center, top, left, right, bottom rgbaPixel) bool {
	if center.a == 0 {
		return false
	}
	neighbors := [4]rgbaPixel{top, left, right, bottom}
	similarPairs := 0
	for i := 0; i < len(neighbors); i++ {
		for j := i + 1; j < len(neighbors); j++ {
			if colorsBlendSimilar(neighbors[i], neighbors[j]) {
				similarPairs++
			}
		}
	}
	if similarPairs < 5 { // require most neighbor pairs to agree
		return false
	}
	diffCount := 0
	for _, neighbor := range neighbors {
		if colorsSignificantlyDifferent(center, neighbor) {
			diffCount++
		}
	}
	return diffCount >= 3
}

func softenIsolated2x(center, top, left, right, bottom rgbaPixel) (rgbaPixel, rgbaPixel, rgbaPixel, rgbaPixel, bool) {
	if !shouldSmoothIsolated(center, top, left, right, bottom) {
		return center, center, center, center, false
	}
	topLeftTarget := averagePixels(top, left)
	topRightTarget := averagePixels(top, right)
	bottomLeftTarget := averagePixels(bottom, left)
	bottomRightTarget := averagePixels(bottom, right)
	return blendTowards(center, topLeftTarget, spriteUpscaleIsolatedCornerWeight),
		blendTowards(center, topRightTarget, spriteUpscaleIsolatedCornerWeight),
		blendTowards(center, bottomLeftTarget, spriteUpscaleIsolatedCornerWeight),
		blendTowards(center, bottomRightTarget, spriteUpscaleIsolatedCornerWeight),
		true
}

func softenIsolated3x(center, tl, top, tr, left, right, bl, bottom, br rgbaPixel) ([9]rgbaPixel, bool) {
	if !shouldSmoothIsolated(center, top, left, right, bottom) {
		return [9]rgbaPixel{}, false
	}
	var block [9]rgbaPixel
	block[0] = blendTowards(center, averageThreePixels(tl, top, left), spriteUpscaleIsolatedCornerWeight)
	block[1] = blendTowards(center, top, spriteUpscaleIsolatedEdgeWeight)
	block[2] = blendTowards(center, averageThreePixels(tr, top, right), spriteUpscaleIsolatedCornerWeight)
	block[3] = blendTowards(center, left, spriteUpscaleIsolatedEdgeWeight)
	block[4] = center
	block[5] = blendTowards(center, right, spriteUpscaleIsolatedEdgeWeight)
	block[6] = blendTowards(center, averageThreePixels(bl, bottom, left), spriteUpscaleIsolatedCornerWeight)
	block[7] = blendTowards(center, bottom, spriteUpscaleIsolatedEdgeWeight)
	block[8] = blendTowards(center, averageThreePixels(br, bottom, right), spriteUpscaleIsolatedCornerWeight)
	return block, true
}

func applyUpscaleAntialias(src *image.RGBA) *image.RGBA {
	if src == nil {
		return nil
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return src
	}
	dst := image.NewRGBA(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			center := sampleRGBA(src, x, y)
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				setRGBA(dst, x, y, center)
				continue
			}
			if center.a == 0 {
				setRGBA(dst, x, y, center)
				continue
			}
			top := sampleRGBA(src, x, y-1)
			bottom := sampleRGBA(src, x, y+1)
			left := sampleRGBA(src, x-1, y)
			right := sampleRGBA(src, x+1, y)
			topLeft := sampleRGBA(src, x-1, y-1)
			topRight := sampleRGBA(src, x+1, y-1)
			bottomLeft := sampleRGBA(src, x-1, y+1)
			bottomRight := sampleRGBA(src, x+1, y+1)

			targets := make([]rgbaPixel, 0, 4)
			addPair := func(a, b rgbaPixel) {
				if colorsEdgeSimilar(a, b) && colorsSignificantlyDifferent(center, a) {
					targets = append(targets, averagePixels(a, b))
				}
			}
			addPair(top, bottom)
			addPair(left, right)
			addPair(topLeft, bottomRight)
			addPair(topRight, bottomLeft)

			if len(targets) > 0 {
				blendTarget := averagePixelsSlice(targets)
				center = blendTowards(center, blendTarget, spriteUpscaleAntiAliasWeight)
			}
			setRGBA(dst, x, y, center)
		}
	}
	return dst
}

func scale4xRGBA(src *image.RGBA) *image.RGBA {
	if src == nil {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	first := scale2xRGBA(src)
	return scale2xRGBA(first)
}
