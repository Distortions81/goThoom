package main

import (
	"math"
	"testing"
)

func benchmarkSoundSamples(count int) []int32 {
	samples := make([]int32, count)
	for i := range samples {
		samples[i] = int32(math.Sin(float64(i)*0.031) * 1_000_000)
	}
	return samples
}

func BenchmarkApplyGameSoundReverb(b *testing.B) {
	originalContext := audioContext
	b.Cleanup(func() {
		audioContext = originalContext
	})

	audioContext = nil
	input := benchmarkSoundSamples(sampleRate)
	work := make([]int32, len(input))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		copy(work, input)
		applyGameSoundReverb(work, 1)
	}
}

func BenchmarkBuildMicroAmbience(b *testing.B) {
	input := make([]float32, sampleRate)
	for i := range input {
		input[i] = float32(math.Sin(float64(i) * 0.031))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = buildMicroAmbience(input, sampleRate, 0)
	}
}
