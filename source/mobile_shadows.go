package main

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"gothoom/climg"
)

const (
	poseLie                = 41
	shadowDropOffset       = 5.0
	minimumShadowSunHeight = 12.0
	nominalNoonSunHeight   = 55.0
	minimumShadowContrast  = 0.70
	detailedCoreOpacity    = 0.45
	detailedEdgeOpacity    = 0.10
	detailedEdgeBaseRadius = 0.50
	detailedEdgeLengthGain = 0.20
	detailedEdgeMaxRadius  = 2.00
)

// shadowDarkenBlend directly attenuates the scene beneath the silhouette while
// preserving the destination alpha. This keeps ground color and detail visible.
var shadowDarkenBlend = ebiten.Blend{
	BlendFactorSourceRGB:        ebiten.BlendFactorZero,
	BlendFactorSourceAlpha:      ebiten.BlendFactorZero,
	BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceAlpha,
	BlendFactorDestinationAlpha: ebiten.BlendFactorOne,
	BlendOperationRGB:           ebiten.BlendOperationAdd,
	BlendOperationAlpha:         ebiten.BlendOperationAdd,
}

type characterShadowProjection struct {
	angle       float64
	length      float64
	dropOffsetX float64
	dropOffsetY float64
	contrast    float32
}

func normalizeShadowAzimuth(azimuth int) int {
	azimuth %= 360
	if azimuth < 0 {
		azimuth += 360
	}
	return azimuth
}

func chooseUprightShadowPose(state uint8, azimuth int) (uint8, bool) {
	if state < poseDead {
		facing := int(state) / 4
		sunDirection := (normalizeShadowAzimuth(azimuth) + 23) / 45
		shadowFacing := (facing + sunDirection + 6) & 7
		return uint8(shadowFacing*4 + int(state&3)), true
	}
	if state == poseDead || state == poseLie {
		return 0, false
	}
	return state, true
}

func characterShadowSunHeight(azimuth int) float64 {
	angle := float64(normalizeShadowAzimuth(azimuth))
	if angle > 180 {
		angle -= 180
	}
	latitude := 35 * math.Pi / 180
	hourAngle := angle * math.Pi / 180
	elevation := math.Asin(math.Cos(latitude)*math.Sin(hourAngle)) * 180 / math.Pi
	if elevation < minimumShadowSunHeight {
		elevation = minimumShadowSunHeight
	} else if elevation > 85 {
		elevation = 85
	}
	return elevation
}

// uprightShadowLength returns the ground-shadow length for unit height. The
// minimum sun height prevents very long, heavily stretched silhouettes.
func uprightShadowLength(azimuth int) float64 {
	elevation := characterShadowSunHeight(azimuth)
	return math.Tan((90 - elevation) * math.Pi / 180)
}

func newCharacterShadowProjection(azimuth int) characterShadowProjection {
	angle := float64(azimuth) * math.Pi / 180
	elevation := characterShadowSunHeight(azimuth)
	daylight := elevation / nominalNoonSunHeight
	if daylight > 1 {
		daylight = 1
	}
	return characterShadowProjection{
		angle:       angle,
		length:      math.Tan((90 - elevation) * math.Pi / 180),
		dropOffsetX: -shadowDropOffset * math.Cos(angle) * gs.GameScale,
		dropOffsetY: shadowDropOffset * math.Sin(angle) * gs.GameScale,
		contrast:    float32(minimumShadowContrast + (1-minimumShadowContrast)*daylight),
	}
}

func currentCharacterShadowState() (float32, int, bool) {
	if !gs.CharacterShadows || gs.MaxNightLevel == 0 {
		return 0, 0, false
	}
	gNight.mu.Lock()
	level := gNight.Shadows
	azimuth := gNight.Azimuth
	gNight.mu.Unlock()
	if level <= 0 {
		return 0, 0, false
	}
	if level > 100 {
		level = 100
	}
	return float32(level) / 100, normalizeShadowAzimuth(azimuth), true
}

func detailedCharacterShadowRadius(projection characterShadowProjection) float64 {
	radius := (detailedEdgeBaseRadius + detailedEdgeLengthGain*projection.length) * gs.GameScale
	return math.Max(1, math.Min(radius, detailedEdgeMaxRadius*gs.GameScale))
}

func drawMobileShadows(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, maxDist int) {
	shadowAlpha, azimuth, ok := currentCharacterShadowState()
	if !ok || clImages == nil {
		return
	}
	projection := newCharacterShadowProjection(azimuth)

	for _, mobile := range mobiles {
		desc, ok := descMap[mobile.Index]
		if !ok {
			continue
		}
		state := mobile.State
		upright := clImages.Flags(uint32(desc.PictID))&climg.PictDefFlagUprightShadow != 0
		if upright {
			var casts bool
			state, casts = chooseUprightShadowPose(state, azimuth)
			if !casts {
				continue
			}
		}

		colors := playerColorsForDescriptor(desc)
		img := loadMobileFrame(desc.PictID, state, colors)
		key := makeMobileKey(desc.PictID, state, colors)
		img = getScaledMobileFrame(key, img)
		if img == nil {
			continue
		}

		size := mobileSize(desc.PictID)
		if size == 0 {
			size = img.Bounds().Dx()
		}
		x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxDist)
		drawCharacterShadow(screen, img, size, x, y, shadowAlpha, projection, upright)
	}
}

func drawCharacterShadow(screen, img *ebiten.Image, size, x, y int, alpha float32, projection characterShadowProjection, upright bool) {
	drawSize := img.Bounds().Dx()
	if drawSize <= 0 || size <= 0 {
		return
	}
	shadowAlpha := alpha * projection.contrast
	if gs.DetailedCharacterShadows {
		radius := detailedCharacterShadowRadius(projection)
		edgeAlpha := shadowAlpha * detailedEdgeOpacity
		for _, offset := range [][2]float64{{-radius, 0}, {radius, 0}, {0, -radius}, {0, radius}} {
			drawCharacterShadowLayer(screen, img, drawSize, size, x, y, edgeAlpha, projection, upright, offset[0], offset[1])
		}
		shadowAlpha *= detailedCoreOpacity
	}
	drawCharacterShadowLayer(screen, img, drawSize, size, x, y, shadowAlpha, projection, upright, 0, 0)
}

func drawCharacterShadowLayer(screen, img *ebiten.Image, drawSize, size, x, y int, alpha float32, projection characterShadowProjection, upright bool, offsetX, offsetY float64) {
	target := float64(roundToInt(float64(size) * gs.GameScale))
	baseScale := target / float64(drawSize)
	op := acquireDrawOpts()
	op.Filter = ebiten.FilterLinear
	op.DisableMipmaps = true
	op.Blend = shadowDarkenBlend
	op.ColorScale.Scale(0, 0, 0, alpha)
	if upright {
		op.GeoM = uprightShadowGeoM(drawSize, size, x, y, projection)
	} else {
		op.GeoM.Scale(baseScale, baseScale)
		op.GeoM.Translate(float64(x)-target/2+projection.dropOffsetX, float64(y)-target/2+projection.dropOffsetY)
	}
	op.GeoM.Translate(offsetX, offsetY)

	screen.DrawImage(img, op)
	releaseDrawOpts(op)
}

func uprightShadowGeoM(drawSize, size, x, y int, projection characterShadowProjection) ebiten.GeoM {
	target := float64(roundToInt(float64(size) * gs.GameScale))
	baseScale := target / float64(drawSize)

	var geo ebiten.GeoM
	geo.Translate(-float64(drawSize)/2, -float64(drawSize))
	geo.Scale(baseScale, baseScale*projection.length)
	geo.Rotate(-(projection.angle + math.Pi/2))
	geo.Translate(float64(x), float64(y)+target/2)
	return geo
}
