package main

import "testing"

func TestPrecacheDefaultsBySystemMemory(t *testing.T) {
	tests := []struct {
		name       string
		memory     uint64
		wasm       bool
		wantSounds bool
		wantImages bool
	}{
		{name: "unknown", memory: 0},
		{name: "low memory", memory: 3 * gibibyte},
		{name: "sound tier", memory: 4 * gibibyte, wantSounds: true},
		{name: "below image tier", memory: 8*gibibyte - 1, wantSounds: true},
		{name: "image tier", memory: 8 * gibibyte, wantSounds: true, wantImages: true},
		{name: "wasm stays off", memory: 64 * gibibyte, wasm: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sounds, images := precacheDefaults(tt.memory, tt.wasm)
			if sounds != tt.wantSounds || images != tt.wantImages {
				t.Fatalf("precacheDefaults(%d, %v) = (%v, %v), want (%v, %v)", tt.memory, tt.wasm, sounds, images, tt.wantSounds, tt.wantImages)
			}
		})
	}
}
