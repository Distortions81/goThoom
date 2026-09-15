package main

import (
	"math"
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

func TestWavingFlagShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(wavingFlagShaderSource)
	if err != nil {
		t.Fatalf("compile waving flag shader: %v", err)
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

func TestReplacementEffectsPreviewGrassPatternIsStable(t *testing.T) {
	first := replacementEffectsPreviewGrassPattern(4, 7)
	if got := replacementEffectsPreviewGrassPattern(4, 7); got != first {
		t.Fatalf("grass pattern changed between calls: %08x then %08x", first, got)
	}
	if other := replacementEffectsPreviewGrassPattern(5, 7); other == first {
		t.Fatalf("adjacent grass tiles share pattern %08x", first)
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

func TestWavingFlagIsPersistent(t *testing.T) {
	if replacementEffectIsPersistent(replacementEffectWavingFlag) == false {
		t.Fatal("waving flags must remain visible with their source sprite")
	}
	if replacementEffectIsPersistent(replacementEffectFirePlume) {
		t.Fatal("fire plume must remain a transient effect")
	}
}

func TestReplacementEffectFramePhaseUsesSourceAnimationTimeline(t *testing.T) {
	if got := replacementEffectFramePhase(0, 4, 1.35); got != 0 {
		t.Fatalf("first frame phase = %v, want 0", got)
	}
	if got, want := replacementEffectFramePhase(3, 4, 1.35), float32(1.35); math.Abs(float64(got-want)) > 0.0001 {
		t.Fatalf("last frame phase = %v, want %v", got, want)
	}
	if got, want := replacementEffectFramePhase(7, 4, 1), float32(1); got != want {
		t.Fatalf("wrapped source frame phase = %v, want %v", got, want)
	}
}

func TestWavingFlagThemes(t *testing.T) {
	for _, test := range []struct {
		id   uint16
		want float32
	}{
		{885, 0}, {886, 1}, {887, 2}, {5645, 3}, {5646, 4}, {5647, 5},
	} {
		if got := replacementEffectFlagTheme(test.id); got != test.want {
			t.Errorf("flag theme for %d = %v, want %v", test.id, got, test.want)
		}
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
		{id: 482, kind: replacementEffectFirePlume, ok: true},
		{id: 572, kind: replacementEffectFirePlume, ok: true},
		{id: 885, kind: replacementEffectWavingFlag, ok: true},
		{id: 5647, kind: replacementEffectWavingFlag, ok: true},
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
	ids := [...]uint16{1, 445, 481, 482, 572, 885, 5647, 1286, 1759, 1847, 2976, 2977, 2978, 3125, 5000}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		kind, _ := replacementEffectKindForPict(ids[i%len(ids)])
		benchmarkReplacementEffectKind = kind
	}
}
