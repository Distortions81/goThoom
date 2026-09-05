package main

import (
	"fmt"
	"testing"
)

func BenchmarkCompletionWordBoundaries(b *testing.B) {
	candidates := make([]string, 500)
	for i := range candidates {
		candidates[i] = fmt.Sprintf("Player %03d", i)
	}
	candidates = append(candidates, "Healing Potion")
	for b.Loop() {
		completionAtWordBoundary("I would like a healing p", candidates)
	}
}

var benchmarkSoundKey soundPlaybackKey

func BenchmarkSoundKey(b *testing.B) {
	for _, count := range []int{1, 4, 32} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			ids := make([]uint16, count)
			for i := range ids {
				ids[i] = uint16(count - i)
			}
			b.ReportAllocs()
			for b.Loop() {
				benchmarkSoundKey = soundPlaybackCacheKey(ids, nil, true, 1, true)
			}
		})
	}
}
