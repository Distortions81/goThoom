package main

import (
	_ "embed"
	"gothoom/climg"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
)

const maxLights = 128

//go:embed data/shaders/light.kage
var lightShaderSrc []byte

var (
	lightingShader *ebiten.Shader
	lightingTmp    *ebiten.Image
	frameLights    []lightSource
	frameDarks     []darkSource
	// Reused shader data to avoid per-frame allocations
	lposX, lposY, lradius, lr, lg, lb, lint [maxLights]float32
	dposX, dposY, dradius, da, dint         [maxLights]float32
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
		"LightCount":     0,
		"DarkCount":      0,
		"LightPosX":      lposX[:],
		"LightPosY":      lposY[:],
		"LightRadius":    lradius[:],
		"LightR":         lr[:],
		"LightG":         lg[:],
		"LightB":         lb[:],
		"LightIntensity": lint[:],
		"DarkPosX":       dposX[:],
		"DarkPosY":       dposY[:],
		"DarkRadius":     dradius[:],
		"DarkAlpha":      da[:],
		"DarkIntensity":  dint[:],
		"LightStrength":  float32(1),
		"GlowStrength":   float32(1),
		"NightFactor":    float32(0),
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
	// Intensity is a scalar multiplier applied to Alpha for fades.
	Intensity float32
	// AgeFrames: how many full game frames this dark persisted
	AgeFrames float32
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

	// Update counts
	lightingUniforms["LightCount"] = len(il)
	lightingUniforms["DarkCount"] = len(id)

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
		if ds.Intensity <= 0 {
			dint[i] = 0
		} else if ds.Intensity >= 1 {
			dint[i] = 1
		} else {
			dint[i] = ds.Intensity
		}
	}

	// Scalars
	lightingUniforms["LightStrength"] = float32(gs.ShaderLightStrength)
	lightingUniforms["GlowStrength"] = float32(gs.ShaderGlowStrength)

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

func addLightSource(pictID uint32, x, y float64, size int) {
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
	radius := float32(li.Radius)
	if radius == 0 {
		radius = float32(size)
	}
	radius *= float32(gs.GameScale)
	cx := float32(x)
	cy := float32(y)
	if flags&climg.PictDefFlagLightDarkcaster != 0 {
		if len(frameDarks) < maxLights {
			alpha := float32(li.Color[3]) / 255
			frameDarks = append(frameDarks, darkSource{X: cx, Y: cy, Radius: radius, Alpha: alpha, Intensity: 1})
		}
	} else {
		if len(frameLights) < maxLights {
			r := float32(li.Color[0]) / 255
			g := float32(li.Color[1]) / 255
			b := float32(li.Color[2]) / 255
			frameLights = append(frameLights, lightSource{X: cx, Y: cy, Radius: radius, R: r, G: g, B: b, Intensity: 1})
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
