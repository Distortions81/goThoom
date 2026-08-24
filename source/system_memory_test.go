package main

import "testing"

func TestPrecacheDefaultsBySystemMemory(t *testing.T) {
	tests := []struct {
		name       string
		memory     uint64
		wasm       bool
		wantSounds bool
	}{
		{name: "unknown", memory: 0},
		{name: "low memory", memory: 3 * gibibyte},
		{name: "sound tier", memory: 4 * gibibyte, wantSounds: true},
		{name: "high memory", memory: 8 * gibibyte, wantSounds: true},
		{name: "wasm stays off", memory: 64 * gibibyte, wasm: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sounds := precacheSoundsDefault(tt.memory, tt.wasm)
			if sounds != tt.wantSounds {
				t.Fatalf("precacheSoundsDefault(%d, %v) = %v, want %v", tt.memory, tt.wasm, sounds, tt.wantSounds)
			}
		})
	}
}
