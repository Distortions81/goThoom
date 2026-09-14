package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	meltysynth "github.com/Distortions81/go-meltysynth/meltysynth"
)

var benchmarkSynthSample float32

func BenchmarkMeltySynthRender64(b *testing.B) {
	font := installedBenchmarkSoundFont(b)
	for _, voices := range []int{1, 16} {
		b.Run(benchmarkVoiceLabel(voices), func(b *testing.B) {
			settings := meltysynth.NewSynthesizerSettings(sampleRate)
			settings.EnableReverbAndChorus = false
			settings.BlockSize = block
			synth, err := meltysynth.NewSynthesizer(font, settings)
			if err != nil {
				b.Fatal(err)
			}
			for voice := 0; voice < voices; voice++ {
				synth.NoteOn(0, int32(48+voice), 100)
			}
			left := make([]float32, block)
			right := make([]float32, block)
			b.ReportAllocs()
			b.ResetTimer()
			for render := range b.N {
				// Retrigger before short, non-looping instrument samples can
				// decay to silence and turn this into an idle-synth benchmark.
				if render > 0 && render%256 == 0 {
					synth.NoteOffAll(true)
					for voice := 0; voice < voices; voice++ {
						key := int32(48 + voice)
						synth.NoteOn(0, key, 100)
					}
				}
				synth.Render(left, right)
			}
			b.StopTimer()
			benchmarkSynthSample = left[0] + right[0]
			renderedSeconds := float64(b.N*block) / sampleRate
			b.ReportMetric(renderedSeconds/b.Elapsed().Seconds(), "x_realtime")
		})
	}
}

func BenchmarkMusicGroupWorkerPool(b *testing.B) {
	font := installedBenchmarkSoundFont(b)
	settings := newSynthSettings()
	for _, partCount := range []int{1, 4, 8} {
		b.Run(fmt.Sprintf("%d_parts", partCount), func(b *testing.B) {
			group := &musicGroupRenderer{
				parts:        make([]*songRenderer, partCount),
				totalSamples: (b.N + 1) * musicRenderBatchFrames,
			}
			for part := range partCount {
				synth, err := meltysynth.NewSynthesizer(font, settings)
				if err != nil {
					b.Fatal(err)
				}
				program := instruments[part%len(instruments)].program
				synth.ProcessMidiMessage(0, 0xC0, int32(program), 0)
				synth.NoteOn(0, int32(48+part), 100)
				group.parts[part] = &songRenderer{
					syn:          synth,
					gain:         1,
					active:       make(map[int]bool),
					totalSamples: group.totalSamples,
				}
			}

			b.ReportAllocs()
			b.SetBytes(musicRenderBatchFrames * 4)
			b.ResetTimer()
			for render := range b.N {
				if render > 0 && render%64 == 0 {
					for part, renderer := range group.parts {
						renderer.syn.NoteOff(0, int32(48+part))
						renderer.syn.NoteOn(0, int32(48+part), 100)
					}
				}
				left, right, err := group.render(musicRenderBatchFrames)
				if err != nil {
					b.Fatal(err)
				}
				benchmarkSynthSample = left[0] + right[0]
			}
			b.StopTimer()
			renderedSeconds := float64(b.N*musicRenderBatchFrames) / sampleRate
			b.ReportMetric(renderedSeconds/b.Elapsed().Seconds(), "x_realtime")
		})
	}
}

func installedBenchmarkSoundFont(b *testing.B) *meltysynth.SoundFont {
	b.Helper()
	root := platformDataDir(runtime.GOOS, runtime.GOARCH, os.Getenv, os.UserHomeDir, os.Executable)
	fontPath := filepath.Join(root, soundFontFile)
	if info, err := os.Stat(fontPath); err != nil || !info.Mode().IsRegular() {
		b.Skipf("normal installed SoundFont is unavailable: %s", fontPath)
	}
	b.Logf("SoundFont: %s", fontPath)
	font, err := loadSoundFont(fontPath)
	if err != nil {
		b.Fatal(err)
	}
	return font
}

func benchmarkVoiceLabel(voices int) string {
	if voices == 1 {
		return "1_voice"
	}
	return "16_voices"
}
