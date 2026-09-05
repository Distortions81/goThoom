package main

import (
	"encoding/binary"
	"slices"
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

func TestSoundPlaybackCacheKeyBufferSizesAndInputOwnership(t *testing.T) {
	for _, count := range []int{0, 1, 4, 16, 17, 32} {
		ids := make([]uint16, count)
		for i := range ids {
			ids[i] = uint16(count - i)
		}
		original := slices.Clone(ids)
		key := soundPlaybackCacheKey(ids, nil, false, 0, false)
		if !slices.Equal(ids, original) {
			t.Fatalf("%d IDs: modified caller slice", count)
		}
		if len(key.ids) != count*2 {
			t.Fatalf("%d IDs: incorrect key length", count)
		}
		for i := range count {
			if got := binary.LittleEndian.Uint16([]byte(key.ids)[2*i:]); got != uint16(i+1) {
				t.Fatalf("%d IDs: key entry %d = %d", count, i, got)
			}
		}
	}
}
