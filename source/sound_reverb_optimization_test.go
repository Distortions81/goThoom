package main

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	math32 "github.com/chewxy/math32"
)

func TestMicroAmbienceInPlaceMatchesSeparateOutput(t *testing.T) {
	rng := rand.New(rand.NewPCG(25, 39))
	for _, rate := range []int{0, 1, 8000, 44100, 48000} {
		for _, length := range []int{0, 1, 64, 2048, 8192} {
			input := make([]float32, length)
			for i := range input {
				input[i] = float32(rng.IntN(65536) - 32768)
			}
			for _, offset := range []int{-200, 0, 120} {
				want := buildMicroAmbience(input, rate, offset)
				inPlace := slices.Clone(input)
				buildMicroAmbienceInto(inPlace, inPlace, rate, offset)
				if !slices.Equal(inPlace, want) {
					t.Fatalf("in-place ambience differs at rate=%d length=%d offset=%d", rate, length, offset)
				}
			}
		}
	}
}

func TestGameSoundReverbBufferReusePreservesSamples(t *testing.T) {
	originalContext := audioContext
	audioContext = nil
	t.Cleanup(func() { audioContext = originalContext })
	for _, amount := range []float64{0, 0.25, 1, 2} {
		for _, length := range []int{0, 1, 220, 1500, 44100} {
			for _, shape := range []string{"silence", "impulse", "noise", "loud"} {
				t.Run(fmt.Sprintf("%s/%d/%g", shape, length, amount), func(t *testing.T) {
					rng := rand.New(rand.NewPCG(53, 71))
					samples := make([]int32, length)
					for i := range samples {
						switch shape {
						case "impulse":
							if i == 0 {
								samples[i] = 30000
							}
						case "noise":
							samples[i] = int32(rng.IntN(65536) - 32768)
						case "loud":
							samples[i] = int32(rng.IntN(1<<30) - 1<<29)
						}
					}
					want := slices.Clone(samples)
					referenceGameSoundReverb(want, amount)
					applyGameSoundReverb(samples, amount)
					if !slices.Equal(samples, want) {
						for i := range samples {
							if samples[i] != want[i] {
								t.Fatalf("sample %d = %d, want %d", i, samples[i], want[i])
							}
						}
					}
				})
			}
		}
	}
}

// Preserve the original three-buffer mixing order as a reference. Shared DSP
// helpers keep this comparison independent of platform-specific SIMD rounding.
func referenceGameSoundReverb(samples []int32, enhancementAmount float64) {
	if len(samples) == 0 {
		return
	}

	rate := sampleRate
	if audioContext != nil {
		if r := audioContext.SampleRate(); r > 0 {
			rate = r
		}
	}
	if rate <= 0 {
		return
	}

	floatSamples := make([]float32, len(samples))
	for i, v := range samples {
		floatSamples[i] = float32(v)
	}

	ambienceA := buildMicroAmbience(floatSamples, rate, 0)
	offsetSamples := int(math32.Round(float32(rate) * 0.0025))
	ambienceB := buildMicroAmbience(floatSamples, rate, offsetSamples)

	for i := range ambienceA {
		ambienceA[i] += ambienceB[i]
	}
	wetBuffer := ambienceA

	amount := float32(clampSoundEnhancementAmount(enhancementAmount))
	mixScale := math32.Sqrt(amount)

	delayFeedback := 0.42 * mixScale
	if delayFeedback > 0.9 {
		delayFeedback = 0.9
	} else if delayFeedback < 0 {
		delayFeedback = 0
	}

	delayMix := 0.58 * mixScale
	if delayMix > 0.95 {
		delayMix = 0.95
	} else if delayMix < 0 {
		delayMix = 0
	}

	applySlapDelay(wetBuffer, rate, 0.085, delayFeedback, delayMix)

	wetMix := 0.14 * amount
	if wetMix > 0.85 {
		wetMix = 0.85
	} else if wetMix < 0 {
		wetMix = 0
	}
	const scatterRatio = 0.4
	combMix := wetMix * (1 - scatterRatio)
	if combMix < 0 {
		combMix = 0
	}
	scatterMix := wetMix - combMix
	if scatterMix < 0 {
		scatterMix = 0
	}
	dryMix := 1 - wetMix
	if dryMix < 0 {
		dryMix = 0
	}
	const wetLowpass = 0.3
	scatterFeedback := 0.1 * amount
	if scatterFeedback > 0.9 {
		scatterFeedback = 0.9
	} else if scatterFeedback < 0 {
		scatterFeedback = 0
	}
	const maxInt32 = float32(1<<31 - 1)
	const minInt32 = -float32(1 << 31)

	scatterDelay := int(math32.Round(float32(rate) * 0.03))
	var scatter []float32
	if scatterDelay > 0 && scatterMix > 0 {
		scatter = make([]float32, scatterDelay)
	}

	scatterIdx := 0
	var wetState float32
	for i, dry := range floatSamples {
		var wet float32
		if combMix > 0 && i < len(wetBuffer) {
			wet += wetBuffer[i] * combMix
		}
		if len(scatter) > 0 {
			delayed := scatter[scatterIdx]
			wet += delayed * scatterMix
			scatter[scatterIdx] = dry + delayed*scatterFeedback
			scatterIdx++
			if scatterIdx >= len(scatter) {
				scatterIdx = 0
			}
		}

		wetState += (wet - wetState) * wetLowpass
		val := dry*dryMix + wetState
		if val > maxInt32 {
			val = maxInt32
		} else if val < minInt32 {
			val = minInt32
		}
		floatSamples[i] = val
	}

	applySaturation(floatSamples, ambienceB, 1.15, 0.2)

	for i, v := range floatSamples {
		if v > maxInt32 {
			v = maxInt32
		} else if v < minInt32 {
			v = minInt32
		}
		samples[i] = int32(math32.Round(v))
	}
}
