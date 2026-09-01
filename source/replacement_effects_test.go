package main

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestConcurrentReplacementShaderInitializationDoesNotRepeatShaders(t *testing.T) {
	replacementEffectsShaderInitMu.Lock()
	originalReady := replacementEffectsShadersReady
	originalAttempted := replacementEffectsShaderInitAttempted
	originalIndex := replacementEffectsShaderInitIndex
	originalCompile := compileReplacementEffectShaderForInit
	replacementEffectsShadersReady = false
	replacementEffectsShaderInitAttempted = false
	replacementEffectsShaderInitIndex = 0
	var compileCount atomic.Int32
	compileReplacementEffectShaderForInit = func(string, []byte) (*ebiten.Shader, error) {
		compileCount.Add(1)
		return nil, nil
	}
	replacementEffectsShaderInitMu.Unlock()

	t.Cleanup(func() {
		replacementEffectsShaderInitMu.Lock()
		replacementEffectsShadersReady = originalReady
		replacementEffectsShaderInitAttempted = originalAttempted
		replacementEffectsShaderInitIndex = originalIndex
		compileReplacementEffectShaderForInit = originalCompile
		replacementEffectsShaderInitMu.Unlock()
	})

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := loadNextReplacementEffectShader(); err != nil {
				t.Errorf("loadNextReplacementEffectShader: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := compileCount.Load(); got != replacementEffectsShaderCount {
		t.Fatalf("compiled %d replacement shaders, want %d", got, replacementEffectsShaderCount)
	}
	if !replacementEffectsShadersReady || !replacementEffectsShaderInitAttempted {
		t.Fatal("replacement shader initialization did not finish")
	}
	if replacementEffectsShaderInitIndex != replacementEffectsShaderCount {
		t.Fatalf("replacement shader index = %d, want %d", replacementEffectsShaderInitIndex, replacementEffectsShaderCount)
	}
}

func TestReplacementEffectKindLookup(t *testing.T) {
	tests := []struct {
		id   uint16
		kind replacementEffectKind
		ok   bool
	}{
		{id: 1759, kind: replacementEffectHealing, ok: true},
		{id: 1286, kind: replacementEffectMysticWard, ok: true},
		{id: 445, kind: replacementEffectMysticFade, ok: true},
		{id: 2976, kind: replacementEffectTeleportGold, ok: true},
		{id: 2977, kind: replacementEffectTeleportBlue, ok: true},
		{id: 2978, kind: replacementEffectTeleportPrismatic, ok: true},
		{id: 3125, kind: replacementEffectStoneForm, ok: true},
		{id: coinRewardFirstPictID, kind: replacementEffectCoinReward, ok: true},
		{id: coinRewardLastPictID, kind: replacementEffectCoinReward, ok: true},
		{id: 1, ok: false},
	}
	for _, test := range tests {
		kind, ok := replacementEffectKindForPict(test.id)
		if kind != test.kind || ok != test.ok {
			t.Errorf("replacementEffectKindForPict(%d) = (%d, %v), want (%d, %v)", test.id, kind, ok, test.kind, test.ok)
		}
	}
}

var benchmarkReplacementEffectKind replacementEffectKind

func BenchmarkReplacementEffectKindLookup(b *testing.B) {
	ids := [...]uint16{1, 445, 1286, 1759, 1847, 2976, 2977, 2978, 3125, 5000}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		kind, _ := replacementEffectKindForPict(ids[i%len(ids)])
		benchmarkReplacementEffectKind = kind
	}
}
