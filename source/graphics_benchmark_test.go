package main

import (
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestGraphicsBenchmarkRecommendation(t *testing.T) {
	if graphicsBenchmarkLimit != 12500*time.Microsecond {
		t.Fatalf("benchmark limit = %v, want the 80 FPS frame budget", graphicsBenchmarkLimit)
	}
	if recommendIGPUGraphics(graphicsBenchmarkLimit - time.Nanosecond) {
		t.Fatal("benchmark below the limit recommended iGPU graphics")
	}
	if recommendIGPUGraphics(graphicsBenchmarkLimit) {
		t.Fatal("benchmark at exactly 80 FPS recommended iGPU graphics")
	}
	if !recommendIGPUGraphics(graphicsBenchmarkLimit + time.Nanosecond) {
		t.Fatal("benchmark below 80 FPS did not recommend iGPU graphics")
	}
	if got := graphicsBenchmarkRecommendedPreset(graphicsBenchmarkResult{}); got != "Full Graphics" {
		t.Fatalf("fast benchmark preset = %q, want Full Graphics", got)
	}
	if got := graphicsBenchmarkRecommendedPreset(graphicsBenchmarkResult{RecommendIGPU: true}); got != "iGPU Graphics" {
		t.Fatalf("slow benchmark preset = %q, want iGPU Graphics", got)
	}
	if got := graphicsBenchmarkRecommendedLabel(graphicsBenchmarkResult{}); got != "Full Quality (Recommended)" {
		t.Fatalf("fast benchmark label = %q", got)
	}
	if got := graphicsBenchmarkRecommendedLabel(graphicsBenchmarkResult{RecommendIGPU: true}); got != "iGPU Graphics (Recommended)" {
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
	t.Logf("graphics benchmark median=%v slowest=%v igpu=%v", game.result.Median, game.result.Slowest, game.result.RecommendIGPU)
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
