package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

func TestSoundPlaybackCacheKeyCanonicalizesSoundIDs(t *testing.T) {
	context := (*audio.Context)(nil)
	first := soundPlaybackCacheKey([]uint16{9, 2, 9, 5}, context, true, 1.25, true)
	second := soundPlaybackCacheKey([]uint16{5, 9, 2, 9}, context, true, 1.25, true)
	if first != second {
		t.Fatalf("equivalent sound mixes produced different cache keys: %+v != %+v", first, second)
	}
	if first == soundPlaybackCacheKey([]uint16{5, 9, 2}, context, true, 1.25, true) {
		t.Fatal("cache key discarded a duplicate sound ID")
	}
	if first == soundPlaybackCacheKey([]uint16{5, 9, 2, 9}, context, true, 1.5, true) {
		t.Fatal("cache key ignored enhancement amount")
	}
	if first == soundPlaybackCacheKey([]uint16{5, 9, 2, 9}, context, true, 1.25, false) {
		t.Fatal("cache key ignored resampling quality")
	}
}

func TestEffectiveAudioVolumeCanMuteTestOutput(t *testing.T) {
	original := muteAudioOutputForTests
	t.Cleanup(func() { muteAudioOutputForTests = original })

	muteAudioOutputForTests = true
	if got := effectiveAudioVolume(0.75); got != 0 {
		t.Fatalf("muted audio volume = %v, want 0", got)
	}
	muteAudioOutputForTests = false
	if got := effectiveAudioVolume(0.75); got != 0.75 {
		t.Fatalf("normal audio volume = %v, want 0.75", got)
	}
}
