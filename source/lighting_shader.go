package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"math"
	"os"
	"sort"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	maxLights        = 128
	maxLightShadows  = 32
	lightCutoffStart = 3.0
	lightCutoffEnd   = 4.0
)

//go:embed data/shaders/light.kage
var lightShaderSrc []byte

var (
	lightingShader           *ebiten.Shader
	lightingShaderVariants   []lightingShaderVariant
	lightingTmp              *ebiten.Image
	frameLights              []lightSource
	frameDarks               []darkSource
	frameLightCasters        []lightCaster
	frameLightShadows        []lightShadow
	mobileSpriteMetricsCache = make(map[mobileKey]mobileSpriteMetrics)
	// Reused shader data to avoid per-frame allocations
	lposX, lposY, linvRadiusSquared, lr, lg, lb, lint [maxLights]float32
	dposX, dposY, dinvRadiusSquared, da, dint, dplane [maxLights]float32
	slightX, slightY, slightInvRadiusSquared          [maxLightShadows]float32
	slightR, slightG, slightB, slightInt              [maxLightShadows]float32
	scasterX, scasterY, scasterRadius                 [maxLightShadows]float32
	shadowAxisX, shadowAxisY, shadowInvDistance       [maxLightShadows]float32
	characterShadowMin, characterShadowMax            [2]float32
	lightingIndices                                   = []uint16{0, 1, 2, 1, 2, 3}
)

type lightingShaderVariant struct {
	maxLights  int
	maxShadows int
	shader     *ebiten.Shader
	uniforms   map[string]any
	op         ebiten.DrawTrianglesShaderOptions
}

var sceneLightingScan struct {
	worldGeneration uint64
	archive         *climg.CLImages
	hasEmitters     bool
	valid           bool
}

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

func newLightingShaderVariant(shader *ebiten.Shader, lightLimit, shadowLimit int) lightingShaderVariant {
	uniforms := map[string]any{
		"LightCount":                  0,
		"DarkCount":                   0,
		"LightPosX":                   lposX[:lightLimit],
		"LightPosY":                   lposY[:lightLimit],
		"LightInvRadiusSquared":       linvRadiusSquared[:lightLimit],
		"LightR":                      lr[:lightLimit],
		"LightG":                      lg[:lightLimit],
		"LightB":                      lb[:lightLimit],
		"LightIntensity":              lint[:lightLimit],
		"DarkPosX":                    dposX[:lightLimit],
		"DarkPosY":                    dposY[:lightLimit],
		"DarkInvRadiusSquared":        dinvRadiusSquared[:lightLimit],
		"DarkAlpha":                   da[:lightLimit],
		"DarkIntensity":               dint[:lightLimit],
		"DarkPlane":                   dplane[:lightLimit],
		"ShadowCount":                 0,
		"ShadowLightX":                slightX[:shadowLimit],
		"ShadowLightY":                slightY[:shadowLimit],
		"ShadowLightInvRadiusSquared": slightInvRadiusSquared[:shadowLimit],
		"ShadowLightR":                slightR[:shadowLimit],
		"ShadowLightG":                slightG[:shadowLimit],
		"ShadowLightB":                slightB[:shadowLimit],
		"ShadowLightIntensity":        slightInt[:shadowLimit],
		"ShadowCasterX":               scasterX[:shadowLimit],
		"ShadowCasterY":               scasterY[:shadowLimit],
		"ShadowCasterRadius":          scasterRadius[:shadowLimit],
		"ShadowAxisX":                 shadowAxisX[:shadowLimit],
		"ShadowAxisY":                 shadowAxisY[:shadowLimit],
		"ShadowInvDistance":           shadowInvDistance[:shadowLimit],
		"LightStrength":               float32(1),
		"GlowStrength":                float32(1),
		"NightFactor":                 float32(0),
		"MaxLightPlane":               float32(32767),
		"HasCharacterShadowMask":      float32(0),
		"CharacterShadowMin":          characterShadowMin[:],
		"CharacterShadowMax":          characterShadowMax[:],
	}
	return lightingShaderVariant{
		maxLights: lightLimit, maxShadows: shadowLimit, shader: shader, uniforms: uniforms,
		op: ebiten.DrawTrianglesShaderOptions{Uniforms: uniforms},
	}
}

func compileLightingShaderVariants(source []byte) ([]lightingShaderVariant, error) {
	if !bytes.Contains(source, []byte("const MaxLights = 128")) || !bytes.Contains(source, []byte("const MaxLightShadows = 32")) {
		return nil, fmt.Errorf("lighting shader capacity constants are missing")
	}
	tiers := [...]struct{ lights, shadows int }{{8, 8}, {32, 16}, {64, 32}, {128, 32}}
	variants := make([]lightingShaderVariant, 0, len(tiers))
	for _, tier := range tiers {
		variantSource := bytes.Replace(source, []byte("const MaxLights = 128"), []byte(fmt.Sprintf("const MaxLights = %d", tier.lights)), 1)
		variantSource = bytes.Replace(variantSource, []byte("const MaxLightShadows = 32"), []byte(fmt.Sprintf("const MaxLightShadows = %d", tier.shadows)), 1)
		shader, err := ebiten.NewShader(variantSource)
		if err != nil {
			for _, variant := range variants {
				variant.shader.Deallocate()
			}
			return nil, fmt.Errorf("compile %d-light/%d-shadow variant: %w", tier.lights, tier.shadows, err)
		}
		variants = append(variants, newLightingShaderVariant(shader, tier.lights, tier.shadows))
	}
	return variants, nil
}

func sceneMayNeedLighting(snap drawSnapshot) bool {
	if clImages == nil || !shaderLightingEnabled() {
		return false
	}
	if currentNightLevel() > 0 {
		return true
	}
	if snap.worldGeneration != 0 && sceneLightingScan.valid &&
		sceneLightingScan.worldGeneration == snap.worldGeneration &&
		sceneLightingScan.archive == clImages {
		return sceneLightingScan.hasEmitters
	}
	hasEmitters := false
	for _, pictures := range [...][]framePicture{snap.picsNeg, snap.picsZero, snap.picsPos} {
		for _, picture := range pictures {
			if clImages.Flags(uint32(picture.PictID))&climg.PictDefFlagEmitsLight != 0 {
				hasEmitters = true
				break
			}
		}
		if hasEmitters {
			break
		}
	}
	if !hasEmitters {
		for _, mobile := range snap.mobiles {
			descriptor, ok := snap.descriptors[mobile.Index]
			if !ok {
				continue
			}
			flags := clImages.Flags(uint32(descriptor.PictID))
			if flags&climg.PictDefFlagEmitsLight != 0 && mobileLightEnabled(flags, mobile.State) {
				hasEmitters = true
				break
			}
		}
	}
	if snap.worldGeneration != 0 {
		sceneLightingScan.worldGeneration = snap.worldGeneration
		sceneLightingScan.archive = clImages
		sceneLightingScan.hasEmitters = hasEmitters
		sceneLightingScan.valid = true
	}
	return hasEmitters
}

func installLightingShaderVariants(source []byte) error {
	variants, err := compileLightingShaderVariants(source)
	if err != nil {
		return err
	}
	previous := lightingShaderVariants
	lightingShaderVariants = variants
	lightingShader = variants[len(variants)-1].shader
	for _, variant := range previous {
		variant.shader.Deallocate()
	}
	return nil
}

func selectLightingShaderVariant(lightCount, darkCount, shadowCount int) *lightingShaderVariant {
	neededLights := max(lightCount, darkCount)
	for index := range lightingShaderVariants {
		variant := &lightingShaderVariants[index]
		if neededLights <= variant.maxLights && shadowCount <= variant.maxShadows {
			return variant
		}
	}
	return nil
}

// ReloadLightingShader recompiles the lighting shader from disk and swaps it in.
// Falls back to the embedded shader source if reading from disk fails.
func ReloadLightingShader() error {
	// Try to reload from the source file for live iteration
	source := lightShaderSrc
	if b, err := os.ReadFile("data/shaders/light.kage"); err == nil {
		source = b
	}
	shadowComposite, err := ebiten.NewShader(layeredShadowCompositeShaderSource)
	if err != nil {
		return fmt.Errorf("compile layered character shadow composite: %w", err)
	}
	if err := installLightingShaderVariants(source); err != nil {
		shadowComposite.Deallocate()
		return err
	}
	previous := layeredShadowCompositeShader
	layeredShadowCompositeShader = shadowComposite
	if previous != nil {
		previous.Deallocate()
	}
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

type lightShadow struct {
	LightX, LightY, LightRadius float32
	LightR, LightG, LightB      float32
	LightIntensity              float32
	CasterX, CasterY            float32
	CasterRadius                float32
}

func ensureLightingTmp(bounds image.Rectangle) *ebiten.Image {
	if lightingTmp == nil || lightingTmp.Bounds() != bounds {
		if lightingTmp != nil {
			lightingTmp.Deallocate()
		}
		lightingTmp = ebiten.NewImageWithOptions(bounds, &ebiten.NewImageOptions{Unmanaged: true})
	}
	return lightingTmp
}

func localLightingPosition(x, y float32, bounds image.Rectangle) (float32, float32) {
	return x - float32(bounds.Min.X), y - float32(bounds.Min.Y)
}

func applyLightingShader(dst *ebiten.Image, lights []lightSource, darks []darkSource, t float32) {
	if lightingShader == nil {
		return
	}
	ensureLightingTmp(dst.Bounds())
	lightingTmp.DrawImage(dst, nil)
	applyWorldComposite(dst, lightingTmp, lights, darks, t, true)
}

func applyWorldComposite(dst, source *ebiten.Image, lights []lightSource, darks []darkSource, t float32, useLighting bool) {
	if lightingShader == nil || dst == nil || source == nil {
		return
	}
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()

	// Scene positions and flicker are already interpolated. Use only this draw's
	// sources so stale positions cannot feed back and accumulate across frames.
	if !useLighting {
		lights = nil
		darks = nil
	}
	il := lights[:min(len(lights), maxLights)]
	id := darks[:min(len(darks), maxLights)]
	frameLightShadows = buildLightShadows(il, frameLightCasters, frameLightShadows[:0])
	variant := selectLightingShaderVariant(len(il), len(id), len(frameLightShadows))
	if variant == nil {
		return
	}
	uniforms := variant.uniforms
	op := &variant.op

	// Update counts
	uniforms["LightCount"] = len(il)
	uniforms["DarkCount"] = len(id)
	uniforms["ShadowCount"] = len(frameLightShadows)

	// Shader distance calculations use source-pixel coordinates, which are local
	// to the temporary image. Sources are stored in destination-image coordinates.
	dstBounds := source.Bounds()
	for i := 0; i < len(il) && i < maxLights; i++ {
		ls := il[i]
		lposX[i], lposY[i] = localLightingPosition(ls.X, ls.Y, dstBounds)
		effectiveRadius := ls.Radius * float32(lightRadiusScale)
		linvRadiusSquared[i] = 1 / (effectiveRadius * effectiveRadius)
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
		dposX[i], dposY[i] = localLightingPosition(ds.X, ds.Y, dstBounds)
		effectiveRadius := ds.Radius * float32(darkRadiusScale)
		dinvRadiusSquared[i] = 1 / (effectiveRadius * effectiveRadius)
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
		slightX[i], slightY[i] = localLightingPosition(shadow.LightX, shadow.LightY, dstBounds)
		slightInvRadiusSquared[i] = 1 / (shadow.LightRadius * shadow.LightRadius)
		slightR[i] = shadow.LightR
		slightG[i] = shadow.LightG
		slightB[i] = shadow.LightB
		slightInt[i] = shadow.LightIntensity
		scasterX[i], scasterY[i] = localLightingPosition(shadow.CasterX, shadow.CasterY, dstBounds)
		scasterRadius[i] = shadow.CasterRadius
		dx := shadow.CasterX - shadow.LightX
		dy := shadow.CasterY - shadow.LightY
		distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if distance < 1 {
			distance = 1
		}
		shadowAxisX[i] = dx / distance
		shadowAxisY[i] = dy / distance
		shadowInvDistance[i] = 1 / distance
	}

	// Scalars
	uniforms["LightStrength"] = float32(gs.ShaderLightStrength)
	uniforms["GlowStrength"] = float32(gs.ShaderGlowStrength)
	uniforms["MaxLightPlane"] = highestLightPlane(il)

	// Smoothed night factor (0..1)
	nightFactor := float32(0)
	if !useLighting {
		nightFactor = 0
	} else if nightAlphaInited {
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
	uniforms["NightFactor"] = nightFactor
	uniforms["HasCharacterShadowMask"] = float32(0)
	op.Images[1] = whiteImage
	if frameDetailedShadowMask != nil && !frameDetailedShadowBounds.Empty() {
		characterShadowMin[0] = float32(frameDetailedShadowBounds.Min.X - dstBounds.Min.X)
		characterShadowMin[1] = float32(frameDetailedShadowBounds.Min.Y - dstBounds.Min.Y)
		characterShadowMax[0] = float32(frameDetailedShadowBounds.Max.X - dstBounds.Min.X)
		characterShadowMax[1] = float32(frameDetailedShadowBounds.Max.Y - dstBounds.Min.Y)
		uniforms["HasCharacterShadowMask"] = float32(1)
		op.Images[1] = frameDetailedShadowMask
	}

	// Bind the full scene and the grow-only cropped shadow target. Pixel-unit
	// triangle shaders permit differently sized sources, unlike DrawRectShader.
	op.Images[0] = source
	left, top := float32(dstBounds.Min.X), float32(dstBounds.Min.Y)
	right, bottom := left+float32(w), top+float32(h)
	vertices := [...]ebiten.Vertex{
		{DstX: left, DstY: top, SrcX: left, SrcY: top, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		{DstX: right, DstY: top, SrcX: right, SrcY: top, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		{DstX: left, DstY: bottom, SrcX: left, SrcY: bottom, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		{DstX: right, DstY: bottom, SrcX: right, SrcY: bottom, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
	}
	dst.DrawTrianglesShader(vertices[:], lightingIndices, variant.shader, op)
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

func addNightDarkSources(bounds image.Rectangle, t float32) {
	lvl := currentNightLevel()
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
	w, h := bounds.Dx(), bounds.Dy()
	diag := float32(math.Hypot(float64(w), float64(h)))
	// Center dark: provide near-total ambient darkening across the scene.
	centerRadius := diag * 1.5
	centerAlpha := alpha * 1.0
	frameDarks = append(frameDarks, darkSource{
		X:         float32(bounds.Min.X+bounds.Max.X) / 2,
		Y:         float32(bounds.Min.Y+bounds.Max.Y) / 2,
		Radius:    centerRadius,
		Alpha:     centerAlpha,
		Intensity: 1,
	})

	// Corner vignettes: minimal edge emphasis
	cornerRadius := diag * 1.1
	cornerAlpha := alpha * 0.02 / 4
	corners := [][2]float32{
		{float32(bounds.Min.X), float32(bounds.Min.Y)},
		{float32(bounds.Max.X), float32(bounds.Min.Y)},
		{float32(bounds.Min.X), float32(bounds.Max.Y)},
		{float32(bounds.Max.X), float32(bounds.Max.Y)},
	}
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
	nearestX := x
	if nearestX < float32(bounds.Min.X) {
		nearestX = float32(bounds.Min.X)
	} else if nearestX > float32(bounds.Max.X) {
		nearestX = float32(bounds.Max.X)
	}
	nearestY := y
	if nearestY < float32(bounds.Min.Y) {
		nearestY = float32(bounds.Min.Y)
	} else if nearestY > float32(bounds.Max.Y) {
		nearestY = float32(bounds.Max.Y)
	}
	return dist2(x, y, nearestX, nearestY) <= radius*radius
}

func lightInfluenceRadius(radius float32) float32 {
	return radius * float32(lightRadiusScale*lightCutoffEnd)
}

type mobileSpriteMetrics struct {
	widthFraction float32
	footFraction  float32
}

func addMobileLightCaster(x, y float64, size int, metrics mobileSpriteMetrics) {
	widthFraction := metrics.widthFraction
	if !shaderLightingEnabled() || !gs.MobileLightConeShadows || size <= 0 || widthFraction <= 0 {
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

	// Production artwork preparation records these values while the pose is
	// already available as CPU RGBA. A conservative fallback keeps injected or
	// synthetic images usable without synchronously reading pixels back from the
	// GPU on the render goroutine.
	metrics = mobileSpriteMetrics{widthFraction: 0.5, footFraction: 0.9}
	imageMu.Lock()
	mobileSpriteMetricsCache[key] = metrics
	imageMu.Unlock()
	return metrics
}

func mobileSpriteMetricsFromRGBA(img *image.RGBA) mobileSpriteMetrics {
	if img == nil || img.Bounds().Empty() {
		return mobileSpriteMetrics{widthFraction: 0.5, footFraction: 0.9}
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	median := medianOccupiedRowWidth(img.Pix, width, height)
	return mobileSpriteMetrics{
		widthFraction: float32(median) / float32(width),
		footFraction:  float32(opaqueFootY(img.Pix, width, height)) / float32(height),
	}
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
		shadowReach := effectiveRadius * float32(lightCutoffEnd)
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
	if !shaderLightingEnabled() || clImages == nil {
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

func addPictureLightSource(pictID uint32, h, v int16, x, y float64, width, height, logicalFrame int, interpolation float64, bounds image.Rectangle) {
	if !shaderLightingEnabled() || clImages == nil {
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
		if !lightIntersectsViewport(cx, cy, lightInfluenceRadius(radius), bounds) {
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

// squared distance
func dist2(ax, ay, bx, by float32) float32 {
	dx := ax - bx
	dy := ay - by
	return dx*dx + dy*dy
}
