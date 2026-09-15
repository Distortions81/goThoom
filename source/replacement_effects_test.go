package main

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestHealingBurstShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(healingBurstShaderSource)
	if err != nil {
		t.Fatalf("compile healing burst shader: %v", err)
	}
	shader.Deallocate()
}

func TestReplacementShaderReloadUsesSourceTreeNotUserData(t *testing.T) {
	originalSourceDir, originalDataDir := replacementEffectsShaderSourceDir, dataDirPath
	replacementEffectsShaderSourceDir = t.TempDir()
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		replacementEffectsShaderSourceDir, dataDirPath = originalSourceDir, originalDataDir
	})

	const source = "package main\nfunc Fragment(_ vec4, _ vec2, _ vec4) vec4 { return vec4(1) }\n"
	if err := os.WriteFile(filepath.Join(replacementEffectsShaderSourceDir, "reload-test.kage"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDirPath, "shaders"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDirPath, "shaders", "reload-test.kage"), []byte("not valid kage"), 0o644); err != nil {
		t.Fatal(err)
	}

	shader, err := compileReplacementEffectShader("reload-test.kage", []byte("not valid kage"))
	if err != nil {
		t.Fatalf("reload should use the source-tree shader: %v", err)
	}
	shader.Deallocate()
}

func TestFirePlumeShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(firePlumeShaderSource)
	if err != nil {
		t.Fatalf("compile fire plume shader: %v", err)
	}
	shader.Deallocate()
}

func TestReplacementEffectPreviewSelection(t *testing.T) {
	originalSelection, originalMode, originalScale, originalUPS := replacementEffectsPreviewSelection, replacementEffectsPreviewMode, replacementEffectsPreviewScale, replacementEffectsPreviewUPS
	t.Cleanup(func() {
		replacementEffectsPreviewSelection, replacementEffectsPreviewMode, replacementEffectsPreviewScale, replacementEffectsPreviewUPS = originalSelection, originalMode, originalScale, originalUPS
	})

	replacementEffectsPreviewSelection = 1
	items := replacementEffectPreviewItems()
	if len(items) != 1 || items[0].kind != replacementEffectFirePlume || items[0].pictID != 481 {
		t.Fatalf("selected preview = %#v, want fire plume 481", items)
	}
	replacementEffectsPreviewMode = replacementEffectPreviewBoth
	if got := replacementEffectPreviewLabel(replacementEffectsPreviewMode); got != "Original + new effect" {
		t.Fatalf("both preview label = %q", got)
	}
	replacementEffectsPreviewScale = replacementEffectPreviewDoubleSize
	if replacementEffectsPreviewScale != replacementEffectPreviewDoubleSize {
		t.Fatal("preview scale did not retain the selected 200% mode")
	}
	replacementEffectsPreviewUPS = 2
	if replacementEffectsPreviewUPS != 2 {
		t.Fatal("preview did not retain the selected animation rate")
	}

	replacementEffectsPreviewSelection = -1
	if got := len(replacementEffectPreviewItems()); got != len(replacementEffectsPreviews) {
		t.Fatalf("gallery has %d effects, want %d", got, len(replacementEffectsPreviews))
	}
}

func TestFirePlumeIsAnOverscannedOneShot(t *testing.T) {
	if !replacementEffectIsOneShot(replacementEffectFirePlume) {
		t.Fatal("fire plume should play as a brief impact")
	}
	if !replacementEffectNeedsOverscan(replacementEffectFirePlume) {
		t.Fatal("fire plume needs room for rising embers")
	}
}

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
		{id: 481, kind: replacementEffectFirePlume, ok: true},
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
	ids := [...]uint16{1, 445, 481, 1286, 1759, 1847, 2976, 2977, 2978, 3125, 5000}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		kind, _ := replacementEffectKindForPict(ids[i%len(ids)])
		benchmarkReplacementEffectKind = kind
	}
}
