package main

import (
	"fmt"
	"image"
	"image/color"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	graphicsBenchmarkWarmups = 2
	graphicsBenchmarkSamples = 7
	graphicsBenchmarkLimit   = 25 * time.Millisecond
)

type graphicsBenchmarkResult struct {
	Median           time.Duration
	Slowest          time.Duration
	RecommendLowVRAM bool
}

func recommendLowVRAMMode(median time.Duration) bool {
	return median >= graphicsBenchmarkLimit
}

func graphicsBenchmarkRecommendedPreset(result graphicsBenchmarkResult) string {
	if result.RecommendLowVRAM {
		return "iGPU / Low-VRAM (Potato GPU)"
	}
	return "High"
}

func graphicsBenchmarkRecommendedLabel(result graphicsBenchmarkResult) string {
	if result.RecommendLowVRAM {
		return "Low-VRAM (Recommended)"
	}
	return "Full Quality (Recommended)"
}

// runGraphicsBenchmark measures synchronized work from the same lighting and
// artwork-upscale shaders used by the client. It must be
// called from Ebitengine's main goroutine after the game has started.
func runGraphicsBenchmark() (graphicsBenchmarkResult, error) {
	if isWASM {
		return graphicsBenchmarkResult{}, fmt.Errorf("the graphics test is unavailable in browser builds")
	}
	if lightingShader == nil || spriteUpscaleShader == nil {
		return graphicsBenchmarkResult{}, fmt.Errorf("graphics shaders are not ready")
	}

	pattern := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			a := uint8(255)
			if (x/8+y/8)%5 == 0 {
				a = 0
			}
			pattern.SetRGBA(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: uint8((x + y) * 2), A: a})
		}
	}
	patternImage := ebiten.NewImageFromImage(pattern)
	defer patternImage.Deallocate()

	nearest := ebiten.NewImage(512, 512)
	defer nearest.Deallocate()
	nearestOp := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	nearestOp.GeoM.Scale(4, 4)
	nearest.DrawImage(patternImage, nearestOp)

	upscaled := ebiten.NewImage(512, 512)
	defer upscaled.Deallocate()
	upscaleOp := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
		"Scale":         float32(4),
		"CornerReach":   float32(2.75),
		"BlendStrength": float32(0.82),
	}}
	upscaleOp.Images[0] = nearest

	const lightingWidth, lightingHeight = 960, 640
	lightingSource := ebiten.NewImage(lightingWidth, lightingHeight)
	defer lightingSource.Deallocate()
	lightingSource.Fill(color.RGBA{R: 120, G: 110, B: 90, A: 255})
	lightingOutput := ebiten.NewImage(lightingWidth, lightingHeight)
	defer lightingOutput.Deallocate()

	lightX := make([]float32, maxLights)
	lightY := make([]float32, maxLights)
	lightRadius := make([]float32, maxLights)
	lightR := make([]float32, maxLights)
	lightG := make([]float32, maxLights)
	lightB := make([]float32, maxLights)
	lightIntensity := make([]float32, maxLights)
	for i := 0; i < 12; i++ {
		lightX[i] = float32((i%4 + 1) * lightingWidth / 5)
		lightY[i] = float32((i/4 + 1) * lightingHeight / 4)
		lightRadius[i] = 180
		lightR[i], lightG[i], lightB[i], lightIntensity[i] = 1, 0.75, 0.45, 1
	}
	darkX := make([]float32, maxLights)
	darkY := make([]float32, maxLights)
	darkRadius := make([]float32, maxLights)
	darkAlpha := make([]float32, maxLights)
	darkIntensity := make([]float32, maxLights)
	darkPlane := make([]float32, maxLights)
	for i := 0; i < 5; i++ {
		darkX[i], darkY[i] = lightingWidth/2, lightingHeight/2
		darkRadius[i], darkAlpha[i], darkIntensity[i] = 700, 0.15, 1
	}
	shadowLightX := make([]float32, maxLightShadows)
	shadowLightY := make([]float32, maxLightShadows)
	shadowLightRadius := make([]float32, maxLightShadows)
	shadowLightR := make([]float32, maxLightShadows)
	shadowLightG := make([]float32, maxLightShadows)
	shadowLightB := make([]float32, maxLightShadows)
	shadowLightIntensity := make([]float32, maxLightShadows)
	shadowCasterX := make([]float32, maxLightShadows)
	shadowCasterY := make([]float32, maxLightShadows)
	shadowCasterRadius := make([]float32, maxLightShadows)
	for i := 0; i < 12; i++ {
		shadowLightX[i], shadowLightY[i] = lightX[i], lightY[i]
		shadowLightRadius[i] = lightRadius[i]
		shadowLightR[i], shadowLightG[i], shadowLightB[i] = lightR[i], lightG[i], lightB[i]
		shadowLightIntensity[i] = 1
		shadowCasterX[i], shadowCasterY[i], shadowCasterRadius[i] = lightX[i]+60, lightY[i], 10
	}
	lightOp := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
		"LightCount":           12,
		"DarkCount":            5,
		"LightPosX":            lightX,
		"LightPosY":            lightY,
		"LightRadius":          lightRadius,
		"LightR":               lightR,
		"LightG":               lightG,
		"LightB":               lightB,
		"LightIntensity":       lightIntensity,
		"DarkPosX":             darkX,
		"DarkPosY":             darkY,
		"DarkRadius":           darkRadius,
		"DarkAlpha":            darkAlpha,
		"DarkIntensity":        darkIntensity,
		"DarkPlane":            darkPlane,
		"ShadowCount":          12,
		"ShadowLightX":         shadowLightX,
		"ShadowLightY":         shadowLightY,
		"ShadowLightRadius":    shadowLightRadius,
		"ShadowLightR":         shadowLightR,
		"ShadowLightG":         shadowLightG,
		"ShadowLightB":         shadowLightB,
		"ShadowLightIntensity": shadowLightIntensity,
		"ShadowCasterX":        shadowCasterX,
		"ShadowCasterY":        shadowCasterY,
		"ShadowCasterRadius":   shadowCasterRadius,
		"LightStrength":        float32(1),
		"GlowStrength":         float32(1),
		"NightFactor":          float32(0.75),
		"MaxLightPlane":        float32(32767),
	}}
	lightOp.Images[0] = lightingSource
	sentinel := lightingOutput.SubImage(image.Rect(0, 0, 1, 1)).(*ebiten.Image)
	pixel := make([]byte, 4)

	pass := func() time.Duration {
		start := time.Now()
		upscaled.DrawRectShader(512, 512, spriteUpscaleShader, upscaleOp)

		lightingOutput.DrawRectShader(lightingWidth, lightingHeight, lightingShader, lightOp)
		sentinel.ReadPixels(pixel)
		return time.Since(start)
	}

	for i := 0; i < graphicsBenchmarkWarmups; i++ {
		pass()
	}
	samples := make([]time.Duration, graphicsBenchmarkSamples)
	for i := range samples {
		samples[i] = pass()
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	median := samples[len(samples)/2]
	return graphicsBenchmarkResult{
		Median:           median,
		Slowest:          samples[len(samples)-1],
		RecommendLowVRAM: recommendLowVRAMMode(median),
	}, nil
}
