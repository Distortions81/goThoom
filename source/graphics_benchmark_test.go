package main

import (
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestGraphicsBenchmarkRecommendation(t *testing.T) {
	if recommendLowVRAMMode(graphicsBenchmarkLimit - time.Nanosecond) {
		t.Fatal("benchmark below the limit recommended Low-VRAM mode")
	}
	if !recommendLowVRAMMode(graphicsBenchmarkLimit) {
		t.Fatal("benchmark at the limit did not recommend Low-VRAM mode")
	}
	if got := graphicsBenchmarkRecommendedPreset(graphicsBenchmarkResult{}); got != "High" {
		t.Fatalf("fast benchmark preset = %q, want High", got)
	}
	if got := graphicsBenchmarkRecommendedPreset(graphicsBenchmarkResult{RecommendLowVRAM: true}); got != "iGPU / Low-VRAM (Potato GPU)" {
		t.Fatalf("slow benchmark preset = %q, want Low-VRAM", got)
	}
	if got := graphicsBenchmarkRecommendedLabel(graphicsBenchmarkResult{}); got != "Full Quality (Recommended)" {
		t.Fatalf("fast benchmark label = %q", got)
	}
	if got := graphicsBenchmarkRecommendedLabel(graphicsBenchmarkResult{RecommendLowVRAM: true}); got != "Low-VRAM (Recommended)" {
		t.Fatalf("slow benchmark label = %q", got)
	}
}

func TestRunGraphicsBenchmark(t *testing.T) {
	if os.Getenv("GOTHOOM_RUN_GRAPHICS_BENCHMARK") == "" {
		t.Skip("set GOTHOOM_RUN_GRAPHICS_BENCHMARK=1 to run the synchronized GPU test")
	}
	game := &graphicsBenchmarkGame{}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
	if game.result.Median <= 0 || game.result.Slowest < game.result.Median {
		t.Fatalf("invalid graphics benchmark result: %+v", game.result)
	}
	t.Logf("graphics benchmark median=%v slowest=%v low-vram=%v", game.result.Median, game.result.Slowest, game.result.RecommendLowVRAM)
}

type graphicsBenchmarkGame struct {
	run    bool
	result graphicsBenchmarkResult
	err    error
}

func (g *graphicsBenchmarkGame) Update() error {
	if g.run {
		return ebiten.Termination
	}
	return nil
}

func (g *graphicsBenchmarkGame) Draw(_ *ebiten.Image) {
	if g.run {
		return
	}
	g.result, g.err = runGraphicsBenchmark()
	g.run = true
}

func (g *graphicsBenchmarkGame) Layout(_, _ int) (int, int) {
	return 16, 16
}
