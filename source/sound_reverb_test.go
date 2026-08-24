package main

import (
	"math"
	"testing"

	math32 "github.com/chewxy/math32"
)

func TestSIMDSaturationMatchesScalarFloat32(t *testing.T) {
	samples := []float32{-65536, -32768, -12000, -1, 0, 1, 12000, 32767, 65536}
	want := append([]float32(nil), samples...)
	const drive = float32(1.15)
	const mix = float32(0.2)
	const toFloat = float32(1.0 / 32768.0)
	const fromFloat = float32(32768.0)
	norm := math32.Tanh(drive)
	for i, sample := range want {
		normalized := sample * toFloat
		saturated := math32.Tanh(normalized*drive) / norm
		want[i] = ((1-mix)*normalized + mix*saturated) * fromFloat
	}

	applySaturation(samples, make([]float32, len(samples)), drive, mix)
	for i := range samples {
		if delta := math.Abs(float64(samples[i] - want[i])); delta > 0.1 {
			t.Errorf("sample %d differs from scalar float32 by %.6f: got %f want %f", i, delta, samples[i], want[i])
		}
	}
}
