package main

import (
	_ "embed"
	"image"
	"math"
	"os"
	"sort"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	maxLights             = 128
	maxLightShadows       = 32
	lightShadowReachScale = 4.0
)

//go:embed data/shaders/light.kage
var lightShaderSrc []byte

var (
	lightingShader           *ebiten.Shader
	lightingTmp              *ebiten.Image
	frameLights              []lightSource
	frameDarks               []darkSource
	frameLightCasters        []lightCaster
	frameLightShadows        []lightShadow
	frameContactShadowLights []contactShadowLight
	mobileSpriteMetricsCache = make(map[mobileKey]mobileSpriteMetrics)
	// Reused shader data to avoid per-frame allocations
	lposX, lposY, lradius, lr, lg, lb, lint [maxLights]float32
	dposX, dposY, dradius, da, dint, dplane [maxLights]float32
	slightX, slightY, slightRadius          [maxLightShadows]float32
	slightR, slightG, slightB, slightInt    [maxLightShadows]float32
	scasterX, scasterY, scasterRadius       [maxLightShadows]float32
	lightingUniforms                        map[string]any
	lightingOp                              ebiten.DrawRectShaderOptions
)

// Global multipliers to make lights/darks reach farther on screen.
const (
	lightRadiusScale = 1.25
	darkRadiusScale  = 1.0
	// Stronger scaling for shader-based night attenuation. At 100% night,
	// total effective darkening approaches this factor depending on layout.
	// Increased baseline shader night strength to produce a very dark
	// overall scene at 100% night.
	// Scale for how strongly night level maps to shader darkening.
	// Lower value avoids saturating darkness at low night levels.
	shaderNightStrength = 0.96
)

// Growth factors for new lights/darks and shrink for fading items
const (
	newLightStartRadiusFactor = 0.001 // start at 10% of target radius
	newDarkStartRadiusFactor  = 0.001 // start at 10% of target radius
	fadeEndRadiusFactor       = 0.001 // shrink to 10% radius by fade end
	radiusGrowFrames          = 5.0   // grow to full radius over N game frames
)

func init() {
	if err := ReloadLightingShader(); err != nil {
		panic(err)
	}
	// Initialize reusable uniforms and options
	lightingUniforms = map[string]any{
		"LightCount":           0,
		"DarkCount":            0,
		"LightPosX":            lposX[:],
		"LightPosY":            lposY[:],
		"LightRadius":          lradius[:],
		"LightR":               lr[:],
		"LightG":               lg[:],
		"LightB":               lb[:],
		"LightIntensity":       lint[:],
		"DarkPosX":             dposX[:],
		"DarkPosY":             dposY[:],
		"DarkRadius":           dradius[:],
		"DarkAlpha":            da[:],
		"DarkIntensity":        dint[:],
		"DarkPlane":            dplane[:],
		"ShadowCount":          0,
		"ShadowLightX":         slightX[:],
		"ShadowLightY":         slightY[:],
		"ShadowLightRadius":    slightRadius[:],
		"ShadowLightR":         slightR[:],
		"ShadowLightG":         slightG[:],
		"ShadowLightB":         slightB[:],
		"ShadowLightIntensity": slightInt[:],
		"ShadowCasterX":        scasterX[:],
		"ShadowCasterY":        scasterY[:],
		"ShadowCasterRadius":   scasterRadius[:],
		"LightStrength":        float32(1),
		"GlowStrength":         float32(1),
		"NightFactor":          float32(0),
		"MaxLightPlane":        float32(32767),
	}
	lightingOp = ebiten.DrawRectShaderOptions{}
	lightingOp.Uniforms = lightingUniforms
}

// ReloadLightingShader recompiles the lighting shader from disk and swaps it in.
// Falls back to the embedded shader source if reading from disk fails.
func ReloadLightingShader() error {
	// Try to reload from the source file for live iteration
	if b, err := os.ReadFile("data/shaders/light.kage"); err == nil {
		if sh, err2 := ebiten.NewShader(b); err2 == nil {
			lightingShader = sh
			return nil
		} else {
			return err2
		}
	}
	// Fallback: use embedded shader source
	sh, err := ebiten.NewShader(lightShaderSrc)
	if err != nil {
		return err
	}
	lightingShader = sh
	return nil
}

type lightSource struct {
	X, Y    float32
	Radius  float32
	R, G, B float32
	Plane   int16
	// Intensity is a scalar multiplier for this light's contribution
	// used for temporal fades. 1 = full, 0 = none.
	Intensity float32
	// AgeFrames: how many full game frames this light persisted
	// Used to grow radius across multiple frames
	AgeFrames float32
}

type darkSource struct {
	X, Y   float32
	Radius float32
	Alpha  float32
	Plane  int16
	// Intensity is a scalar multiplier applied to Alpha for fades.
	Intensity float32
	// AgeFrames: how many full game frames this dark persisted
	AgeFrames float32
}

type lightCaster struct {
	X, Y                 float32
	Radius               float32
	LightExclusionRadius float32
}

type contactShadowLight struct {
	X, Y       float32
	Radius     float32
	OwnerIndex uint8
	HasOwner   bool
}

type lightShadow struct {
	LightX, LightY, LightRadius float32
	LightR, LightG, LightB      float32
	LightIntensity              float32
	CasterX, CasterY            float32
	CasterRadius                float32
}

func ensureLightingTmp(w, h int) {
	if lightingTmp == nil || lightingTmp.Bounds().Dx() != w || lightingTmp.Bounds().Dy() != h {
		// Allocate the intermediate image, respecting potato mode for unmanaged images.
		lightingTmp = newImage(w, h)
	}
}

func applyLightingShader(dst *ebiten.Image, lights []lightSource, darks []darkSource, t float32) {
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	ensureLightingTmp(w, h)
	lightingTmp.DrawImage(dst, nil)

	// Use already-interpolated positions and smooth attributes
	il := interpolateLights(lights, t)
	id := interpolateDarks(darks, t)
	frameLightShadows = buildLightShadows(il, frameLightCasters, frameLightShadows[:0])

	// Update counts
	lightingUniforms["LightCount"] = len(il)
	lightingUniforms["DarkCount"] = len(id)
	lightingUniforms["ShadowCount"] = len(frameLightShadows)

	// Fill light arrays
	for i := 0; i < len(il) && i < maxLights; i++ {
		ls := il[i]
		lposX[i] = ls.X
		lposY[i] = ls.Y
		lradius[i] = ls.Radius * float32(lightRadiusScale)
		lr[i] = ls.R
		lg[i] = ls.G
		lb[i] = ls.B
		if ls.Intensity <= 0 {
			lint[i] = 0
		} else if ls.Intensity >= 1 {
			lint[i] = 1
		} else {
			lint[i] = ls.Intensity
		}
	}
	// Fill dark arrays
	for i := 0; i < len(id) && i < maxLights; i++ {
		ds := id[i]
		dposX[i] = ds.X
		dposY[i] = ds.Y
		dradius[i] = ds.Radius * float32(darkRadiusScale)
		da[i] = ds.Alpha
		dplane[i] = float32(ds.Plane)
		if ds.Intensity <= 0 {
			dint[i] = 0
		} else if ds.Intensity >= 1 {
			dint[i] = 1
		} else {
			dint[i] = ds.Intensity
		}
	}
	for i := 0; i < len(frameLightShadows); i++ {
		shadow := frameLightShadows[i]
		slightX[i] = shadow.LightX
		slightY[i] = shadow.LightY
		slightRadius[i] = shadow.LightRadius
		slightR[i] = shadow.LightR
		slightG[i] = shadow.LightG
		slightB[i] = shadow.LightB
		slightInt[i] = shadow.LightIntensity
		scasterX[i] = shadow.CasterX
		scasterY[i] = shadow.CasterY
		scasterRadius[i] = shadow.CasterRadius
	}

	// Scalars
	lightingUniforms["LightStrength"] = float32(gs.ShaderLightStrength)
	lightingUniforms["GlowStrength"] = float32(gs.ShaderGlowStrength)
	lightingUniforms["MaxLightPlane"] = highestLightPlane(il)

	// Smoothed night factor (0..1)
	nightFactor := float32(0)
	if nightAlphaInited {
		nf := lerpf(nightPrevTarget, nightCurTarget, ease(t)) / float32(shaderNightStrength)
		if nf < 0 {
			nf = 0
		} else if nf > 1 {
			nf = 1
		}
		nightFactor = nf
	} else {
		lvl := currentNightLevel()
		nightFactor = float32(lvl) / 100
	}
	lightingUniforms["NightFactor"] = nightFactor

	// Bind source and draw
	lightingOp.Images[0] = lightingTmp
	dst.DrawRectShader(w, h, lightingShader, &lightingOp)
}

func highestLightPlane(lights []lightSource) float32 {
	if len(lights) == 0 {
		// With no lights there is no ordering boundary, so keep every darkcaster
		// on the existing pre-light path.
		return 32767
	}
	maxPlane := lights[0].Plane
	for _, light := range lights[1:] {
		if light.Plane > maxPlane {
			maxPlane = light.Plane
		}
	}
	return float32(maxPlane)
}

// min helper to avoid importing math just for ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// addNightDarkSources appends dark sources to produce a smooth inverse-square
// vignette-like darkening using the shader path. The overall strength scales
// with the current/effective night level and ambientNightStrength.
// Night smoothing state
var (
	nightAlphaInited bool
	nightLastT       float32
	nightPrevTarget  float32
	nightCurTarget   float32
)

func addNightDarkSources(w, h int, t float32) {
	lvl := currentNightLevel()
	if lvl <= 0 {
		return
	}
	// Convert to [0..1] strength; reuse ambientNightStrength as baseline.
	// Use a higher strength specifically for shader night so 100% looks dark.
	// Apply a gamma curve so low night levels are much gentler than high.
	// This avoids cases where 25% appears darker than 100% due to reveal interplay.
	frac := float64(lvl) / 100.0
	// Photometric-like response; tweak exponent if needed (2.2 is typical)
	gamma := 2.2
	target := float32(math.Pow(frac, gamma) * float64(shaderNightStrength))
	if nightAlphaInited {
		if t < nightLastT { // new frame
			nightPrevTarget = nightCurTarget
			nightCurTarget = target
		} else {
			nightCurTarget = target
		}
	} else {
		nightAlphaInited = true
		nightPrevTarget = target
		nightCurTarget = target
	}
	nightLastT = t
	alpha := lerpf(nightPrevTarget, nightCurTarget, ease(t))
	if alpha <= 0 {
		return
	}
	// Use four corner dark sources with shared alpha to bias edges darker.
	// Radius based on screen diagonal yields gentle center falloff.
	diag := float32(math.Hypot(float64(w), float64(h)))
	// Center dark: provide near-total ambient darkening across the scene.
	centerRadius := diag * 1.5
	centerAlpha := alpha * 1.0
	frameDarks = append(frameDarks, darkSource{X: float32(w) / 2, Y: float32(h) / 2, Radius: centerRadius, Alpha: centerAlpha, Intensity: 1})

	// Corner vignettes: minimal edge emphasis
	cornerRadius := diag * 1.1
	cornerAlpha := alpha * 0.02 / 4
	corners := [][2]float32{{0, 0}, {float32(w), 0}, {0, float32(h)}, {float32(w), float32(h)}}
	for _, c := range corners {
		frameDarks = append(frameDarks, darkSource{X: c[0], Y: c[1], Radius: cornerRadius, Alpha: cornerAlpha, Intensity: 1})
	}
}

func mobileLightEnabled(flags uint32, state uint8) bool {
	if flags&climg.PictDefFlagOnlyAttackPosesLit == 0 {
		return true
	}
	return state < poseDead && state%4 == 3
}

type lightGeometry struct {
	radius    float32
	intensity float32
}

func pictureLightGeometry(metadataRadius uint16, flags uint32, width, height int) lightGeometry {
	radius := float32(metadataRadius)
	if flags&climg.PictDefFlagLightDarkcaster != 0 {
		combinedSize := width + height
		if radius == 0 {
			radius = float32(combinedSize)
		}
		radius *= 4
		minimum := float32(combinedSize) / 2
		if radius < minimum {
			radius = minimum
		}
	} else if radius == 0 {
		// Use the pre-classic-tuning fallback for emitted light. Enlarging this
		// to the combined sprite dimensions makes nearby artwork look blurred.
		radius = float32(width)
	}
	return lightGeometry{radius: radius, intensity: 1}
}

func mobileLightGeometry(metadataRadius uint16, flags uint32, size int, state uint8) lightGeometry {
	radius := float32(metadataRadius)
	if flags&climg.PictDefFlagLightDarkcaster != 0 {
		if radius == 0 {
			radius = float32(size * 2)
		}
		radius *= 4
		intensity := float32(1)
		if state == poseDead {
			radius /= 2
			intensity = 0.5
		}
		minimum := float32(size)
		if radius < minimum {
			radius = minimum
		}
		return lightGeometry{radius: radius, intensity: intensity}
	}
	if radius == 0 {
		// Before the classic-radius pass, mobile emitters used their sprite size.
		radius = float32(size)
	}
	return lightGeometry{radius: radius, intensity: 1}
}

func lightIntersectsViewport(x, y float32, radius float32, bounds image.Rectangle) bool {
	if radius <= 0 || bounds.Empty() {
		return false
	}
	return x+radius >= float32(bounds.Min.X) &&
		x-radius <= float32(bounds.Max.X) &&
		y+radius >= float32(bounds.Min.Y) &&
		y-radius <= float32(bounds.Max.Y)
}

type mobileSpriteMetrics struct {
	widthFraction float32
	footFraction  float32
}

func addMobileLightCaster(x, y float64, size int, metrics mobileSpriteMetrics) {
	widthFraction := metrics.widthFraction
	if !gs.ShaderLighting || size <= 0 || widthFraction <= 0 {
		return
	}
	scaledSize := float32(float64(size) * gs.GameScale)
	radius := scaledSize * widthFraction / 2
	minimumRadius := scaledSize * 0.10
	maximumRadius := scaledSize * 0.28
	if radius < minimumRadius {
		radius = minimumRadius
	} else if radius > maximumRadius {
		radius = maximumRadius
	}
	frameLightCasters = append(frameLightCasters, lightCaster{
		X:                    float32(x),
		Y:                    float32(y) - scaledSize/2 + scaledSize*metrics.footFraction,
		Radius:               radius,
		LightExclusionRadius: scaledSize * 0.60,
	})
}

func mobileSpriteMetricsFor(key mobileKey, img *ebiten.Image) mobileSpriteMetrics {
	if img == nil || img.Bounds().Empty() {
		return mobileSpriteMetrics{}
	}
	imageMu.Lock()
	metrics, ok := mobileSpriteMetricsCache[key]
	imageMu.Unlock()
	if ok {
		return metrics
	}

	bounds := img.Bounds()
	pixels := make([]byte, 4*bounds.Dx()*bounds.Dy())
	img.ReadPixels(pixels)
	median := medianOccupiedRowWidth(pixels, bounds.Dx(), bounds.Dy())
	metrics = mobileSpriteMetrics{
		widthFraction: float32(median) / float32(bounds.Dx()),
		footFraction:  float32(opaqueFootY(pixels, bounds.Dx(), bounds.Dy())) / float32(bounds.Dy()),
	}
	imageMu.Lock()
	mobileSpriteMetricsCache[key] = metrics
	imageMu.Unlock()
	return metrics
}

func opaqueFootY(pixels []byte, width, height int) int {
	if width <= 0 || height <= 0 || len(pixels) < width*height*4 {
		return height
	}
	for y := height - 1; y >= 0; y-- {
		rowStart := y * width * 4
		for x := 0; x < width; x++ {
			if pixels[rowStart+x*4+3] > 16 {
				return y + 1
			}
		}
	}
	return height
}

func medianOccupiedRowWidth(pixels []byte, width, height int) int {
	if width <= 0 || height <= 0 || len(pixels) < width*height*4 {
		return 0
	}
	rowWidths := make([]int, 0, height)
	for y := 0; y < height; y++ {
		count := 0
		rowStart := y * width * 4
		for x := 0; x < width; x++ {
			if pixels[rowStart+x*4+3] > 16 {
				count++
			}
		}
		if count > 0 {
			rowWidths = append(rowWidths, count)
		}
	}
	if len(rowWidths) == 0 {
		return 0
	}
	sort.Ints(rowWidths)
	middle := len(rowWidths) / 2
	if len(rowWidths)%2 == 0 {
		return (rowWidths[middle-1] + rowWidths[middle]) / 2
	}
	return rowWidths[middle]
}

func buildLightShadows(lights []lightSource, casters []lightCaster, dst []lightShadow) []lightShadow {
	for _, light := range lights {
		effectiveRadius := light.Radius * float32(lightRadiusScale)
		shadowReach := effectiveRadius * float32(lightShadowReachScale)
		for _, caster := range casters {
			distanceSquared := dist2(light.X, light.Y, caster.X, caster.Y)
			minimumDistance := caster.Radius * 1.25
			if caster.LightExclusionRadius > minimumDistance {
				minimumDistance = caster.LightExclusionRadius
			}
			if distanceSquared <= minimumDistance*minimumDistance || distanceSquared >= shadowReach*shadowReach {
				continue
			}
			shadow := lightShadow{
				LightX:         light.X,
				LightY:         light.Y,
				LightRadius:    effectiveRadius,
				LightR:         light.R,
				LightG:         light.G,
				LightB:         light.B,
				LightIntensity: light.Intensity,
				CasterX:        caster.X,
				CasterY:        caster.Y,
				CasterRadius:   caster.Radius,
			}
			dst = append(dst, shadow)
		}
	}
	if len(dst) > maxLightShadows {
		// Prefer the interactions deepest inside their light's glow when a
		// crowded scene exceeds the shader's fixed shadow budget.
		sort.Slice(dst, func(i, j int) bool {
			di := dist2(dst[i].LightX, dst[i].LightY, dst[i].CasterX, dst[i].CasterY)
			dj := dist2(dst[j].LightX, dst[j].LightY, dst[j].CasterX, dst[j].CasterY)
			return di/(dst[i].LightRadius*dst[i].LightRadius) < dj/(dst[j].LightRadius*dst[j].LightRadius)
		})
		dst = dst[:maxLightShadows]
	}
	return dst
}

func mixLightFlicker(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}

func lightFlickerTarget(pictID uint32, instanceKey uint64, logicalFrame int) (float64, float64) {
	seed := uint64(pictID)<<32 ^ instanceKey ^ uint64(int64(logicalFrame))*0x9e3779b97f4a7c15
	const hashToSignedUnit = 2.0 / (1 << 53)
	dx := float64(mixLightFlicker(seed)>>11)*hashToSignedUnit - 1
	dy := float64(mixLightFlicker(seed+0x517cc1b727220a95)>>11)*hashToSignedUnit - 1
	return dx, dy
}

func lightFlickerOffset(pictID uint32, instanceKey uint64, logicalFrame int, interpolation float64) (float64, float64) {
	prevX, prevY := lightFlickerTarget(pictID, instanceKey, logicalFrame-1)
	curX, curY := lightFlickerTarget(pictID, instanceKey, logicalFrame)
	u := float64(ease(float32(interpolation)))
	return prevX + (curX-prevX)*u, prevY + (curY-prevY)*u
}

func lightFlickerEnergyTarget(pictID uint32, instanceKey uint64, logicalFrame int) float64 {
	seed := uint64(pictID)<<32 ^ instanceKey ^ uint64(int64(logicalFrame))*0x9e3779b97f4a7c15
	const hashToSignedUnit = 2.0 / (1 << 53)
	return float64(mixLightFlicker(seed+0xd1b54a32d192ed03)>>11)*hashToSignedUnit - 1
}

type flameLightModulation struct {
	offsetX, offsetY float64
	radius           float32
	brightness       float32
}

func flameLightFlicker(flags uint32, pictID uint32, instanceKey uint64, logicalFrame int, interpolation, strength float64) flameLightModulation {
	modulation := flameLightModulation{radius: 1, brightness: 1}
	if flags&climg.PictDefFlagLightFlicker == 0 || strength <= 0 {
		return modulation
	}
	if strength > 2 {
		strength = 2
	}
	modulation.offsetX, modulation.offsetY = lightFlickerOffset(pictID, instanceKey, logicalFrame, interpolation)
	modulation.offsetX *= strength
	modulation.offsetY *= strength
	previous := lightFlickerEnergyTarget(pictID, instanceKey, logicalFrame-1)
	current := lightFlickerEnergyTarget(pictID, instanceKey, logicalFrame)
	u := float64(ease(float32(interpolation)))
	energy := previous + (current-previous)*u
	// Flame light usually dips more noticeably than it flares. Radius follows
	// brightness so the pool of light breathes instead of merely blinking.
	modulation.radius = float32(1 + (-0.03+0.05*energy)*strength)
	modulation.brightness = float32(1 + (-0.06+0.10*energy)*strength)
	return modulation
}

func addMobileLightSource(pictID uint32, state, index uint8, x, y float64, size, logicalFrame int, interpolation float64, bounds image.Rectangle) {
	if !gs.ShaderLighting || clImages == nil {
		return
	}
	flags := clImages.Flags(pictID)
	if flags&climg.PictDefFlagEmitsLight == 0 {
		return
	}
	if !mobileLightEnabled(flags, state) {
		return
	}
	li, ok := clImages.Lighting(pictID)
	if !ok {
		return
	}
	const mobileKeyTag = uint64(1) << 63
	geometry := mobileLightGeometry(li.Radius, flags, size, state)
	addLightSource(pictID, flags, li, geometry, mobileKeyTag|uint64(index), x, y, logicalFrame, interpolation, bounds)
}

func addMobileContactShadowLight(pictID uint32, state, index uint8, x, y float64, size int) {
	if !gs.ShaderLighting || clImages == nil {
		return
	}
	flags := clImages.Flags(pictID)
	if flags&climg.PictDefFlagEmitsLight == 0 || flags&climg.PictDefFlagLightDarkcaster != 0 || !mobileLightEnabled(flags, state) {
		return
	}
	li, ok := clImages.Lighting(pictID)
	if !ok {
		return
	}
	geometry := mobileLightGeometry(li.Radius, flags, size, state)
	frameContactShadowLights = append(frameContactShadowLights, contactShadowLight{
		X:          float32(x),
		Y:          float32(y),
		Radius:     geometry.radius * float32(gs.GameScale*lightRadiusScale),
		OwnerIndex: index,
		HasOwner:   true,
	})
}

func addPictureLightSource(pictID uint32, h, v int16, x, y float64, width, height, logicalFrame int, interpolation float64, bounds image.Rectangle) {
	if !gs.ShaderLighting || clImages == nil {
		return
	}
	flags := clImages.Flags(pictID)
	if flags&climg.PictDefFlagEmitsLight == 0 {
		return
	}
	li, ok := clImages.Lighting(pictID)
	if !ok {
		return
	}
	instanceKey := uint64(uint16(h))<<16 | uint64(uint16(v))
	geometry := pictureLightGeometry(li.Radius, flags, width, height)
	addLightSource(pictID, flags, li, geometry, instanceKey, x, y, logicalFrame, interpolation, bounds)
}

func addPictureContactShadowLight(pictID uint32, x, y float64, width, height int) {
	if !gs.ShaderLighting || clImages == nil {
		return
	}
	flags := clImages.Flags(pictID)
	if flags&climg.PictDefFlagEmitsLight == 0 || flags&climg.PictDefFlagLightDarkcaster != 0 {
		return
	}
	li, ok := clImages.Lighting(pictID)
	if !ok {
		return
	}
	geometry := pictureLightGeometry(li.Radius, flags, width, height)
	frameContactShadowLights = append(frameContactShadowLights, contactShadowLight{
		X:      float32(x),
		Y:      float32(y),
		Radius: geometry.radius * float32(gs.GameScale*lightRadiusScale),
	})
}

func contactShadowNearOtherLight(index uint8, x, y float32) bool {
	for _, light := range frameContactShadowLights {
		if light.HasOwner && light.OwnerIndex == index {
			continue
		}
		if dist2(x, y, light.X, light.Y) <= light.Radius*light.Radius {
			return true
		}
	}
	return false
}

func addLightSource(pictID, flags uint32, li climg.LightInfo, geometry lightGeometry, instanceKey uint64, x, y float64, logicalFrame int, interpolation float64, bounds image.Rectangle) {
	radius := geometry.radius
	radius *= float32(gs.GameScale)
	strength := gs.FlameFlickerStrength
	if !gs.FlameLightFlicker {
		strength = 0
	}
	flame := flameLightFlicker(flags, pictID, instanceKey, logicalFrame, interpolation, strength)
	x += flame.offsetX * gs.GameScale
	y += flame.offsetY * gs.GameScale
	cx := float32(x)
	cy := float32(y)
	if flags&climg.PictDefFlagLightDarkcaster != 0 {
		if !lightIntersectsViewport(cx, cy, radius, bounds) {
			return
		}
		if len(frameDarks) < maxLights {
			alpha := float32(li.Color[3]) / 255 * geometry.intensity
			frameDarks = append(frameDarks, darkSource{X: cx, Y: cy, Radius: radius, Alpha: alpha, Plane: li.Plane, Intensity: 1})
		}
	} else {
		radius *= flame.radius
		if !lightIntersectsViewport(cx, cy, radius, bounds) {
			return
		}
		if len(frameLights) < maxLights {
			brightness := flame.brightness * geometry.intensity
			r := float32(li.Color[0]) / 255 * brightness
			g := float32(li.Color[1]) / 255 * brightness
			b := float32(li.Color[2]) / 255 * brightness
			frameLights = append(frameLights, lightSource{X: cx, Y: cy, Radius: radius, R: r, G: g, B: b, Plane: li.Plane, Intensity: 1})
		}
	}
}

// Previous frame lighting state for temporal blending
var (
	prevLights []lightSource
	prevDarks  []darkSource
	havePrev   bool
)

// smoothstep easing for temporal interpolation
func ease(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func lerpf(a, b, t float32) float32 { return a + (b-a)*t }

// Faster drop for items that are removed: starts dimming immediately.
func fadeOut(u float32) float32 {
	x := 1 - u
	return x * x // quadratic falloff
}

// squared distance
func dist2(ax, ay, bx, by float32) float32 {
	dx := ax - bx
	dy := ay - by
	return dx*dx + dy*dy
}

// interpolateLights blends current lights with previous for smoother fades.
func interpolateLights(curr []lightSource, t float32) []lightSource {
	if len(curr) == 0 && !havePrev {
		return curr
	}
	u := ease(t)
	// If we have no previous, start small radius and grow during first interval.
	if !havePrev {
		out := make([]lightSource, min(len(curr), maxLights))
		for i := 0; i < len(out); i++ {
			out[i] = curr[i]
			out[i].Intensity = 1
			// start small and grow to desired radius over the interval
			out[i].Radius = lerpf(curr[i].Radius*newLightStartRadiusFactor, curr[i].Radius, u)
		}
		// store prev for next frame (persist grown radius)
		prevLights = cloneLights(out)
		havePrev = true
		return out
	}

	// Track matches
	matchedPrev := make([]bool, len(prevLights))
	out := make([]lightSource, 0, min(len(curr)+len(prevLights), maxLights))

	// Greedy nearest match by position
	for _, c := range curr {
		best := -1
		bestD2 := float32(1e12)
		// position threshold scales with radius
		thresh := c.Radius * 0.6
		if thresh < 12 {
			thresh = 12
		} else if thresh > 96 {
			thresh = 96
		}
		thresh2 := thresh * thresh
		for j, p := range prevLights {
			if matchedPrev[j] {
				continue
			}
			d2 := dist2(c.X, c.Y, p.X, p.Y)
			if d2 <= thresh2 && d2 < bestD2 {
				bestD2 = d2
				best = j
			}
		}
		if best >= 0 {
			p := prevLights[best]
			matchedPrev[best] = true
			// Positions already interpolated elsewhere; use current.
			o := c
			// Blend color/radius non-linearly
			o.R = lerpf(p.R, c.R, u)
			o.G = lerpf(p.G, c.G, u)
			o.B = lerpf(p.B, c.B, u)
			o.Radius = lerpf(p.Radius, c.Radius, u)
			o.Intensity = 1
			out = append(out, o)
		} else {
			// New light: start small radius and grow
			o := c
			o.Intensity = 1
			o.Radius = lerpf(c.Radius*newLightStartRadiusFactor, c.Radius, u)
			out = append(out, o)
		}
		if len(out) >= maxLights {
			break
		}
	}
	// Unmatched previous lights: fade out
	if len(out) < maxLights {
		for j, p := range prevLights {
			if matchedPrev[j] {
				continue
			}
			o := p
			o.Intensity = fadeOut(u)
			// shrink radius as it fades out
			o.Radius = lerpf(p.Radius, p.Radius*fadeEndRadiusFactor, u)
			out = append(out, o)
			if len(out) >= maxLights {
				break
			}
		}
	}

	// store blended result as previous for next frame
	prevLights = cloneLights(out)
	havePrev = true
	return out
}

func interpolateDarks(curr []darkSource, t float32) []darkSource {
	if len(curr) == 0 && !havePrev {
		return curr
	}
	u := ease(t)
	if !havePrev {
		out := make([]darkSource, min(len(curr), maxLights))
		for i := 0; i < len(out); i++ {
			out[i] = curr[i]
			out[i].Intensity = 1
			out[i].Radius = lerpf(curr[i].Radius*newDarkStartRadiusFactor, curr[i].Radius, u)
		}
		prevDarks = cloneDarks(out)
		havePrev = true
		return out
	}
	matchedPrev := make([]bool, len(prevDarks))
	out := make([]darkSource, 0, min(len(curr)+len(prevDarks), maxLights))
	for _, c := range curr {
		best := -1
		bestD2 := float32(1e12)
		thresh := c.Radius * 0.6
		if thresh < 16 {
			thresh = 16
		} else if thresh > 128 {
			thresh = 128
		}
		thresh2 := thresh * thresh
		for j, p := range prevDarks {
			if matchedPrev[j] {
				continue
			}
			d2 := dist2(c.X, c.Y, p.X, p.Y)
			if d2 <= thresh2 && d2 < bestD2 {
				bestD2 = d2
				best = j
			}
		}
		if best >= 0 {
			p := prevDarks[best]
			matchedPrev[best] = true
			o := c
			o.Alpha = lerpf(p.Alpha, c.Alpha, u)
			o.Radius = lerpf(p.Radius, c.Radius, u)
			o.Intensity = 1
			out = append(out, o)
		} else {
			o := c
			o.Intensity = 1
			o.Radius = lerpf(c.Radius*newDarkStartRadiusFactor, c.Radius, u)
			out = append(out, o)
		}
		if len(out) >= maxLights {
			break
		}
	}
	if len(out) < maxLights {
		for j, p := range prevDarks {
			if matchedPrev[j] {
				continue
			}
			o := p
			o.Intensity = fadeOut(u)
			o.Radius = lerpf(p.Radius, p.Radius*fadeEndRadiusFactor, u)
			out = append(out, o)
			if len(out) >= maxLights {
				break
			}
		}
	}
	prevDarks = cloneDarks(out)
	havePrev = true
	return out
}

func cloneLights(in []lightSource) []lightSource {
	if len(in) == 0 {
		return nil
	}
	out := make([]lightSource, len(in))
	copy(out, in)
	// stored prev state should be full intensity values
	for i := range out {
		out[i].Intensity = 1
	}
	return out
}

func cloneDarks(in []darkSource) []darkSource {
	if len(in) == 0 {
		return nil
	}
	out := make([]darkSource, len(in))
	copy(out, in)
	for i := range out {
		out[i].Intensity = 1
	}
	return out
}
