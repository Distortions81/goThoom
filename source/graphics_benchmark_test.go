package main

import "testing"

func TestGraphicsBenchmarkRecommendation(t *testing.T) {
	if graphicsBenchmarkFPS != 80 {
		t.Fatalf("FPS limit = %v, want 80", graphicsBenchmarkFPS)
	}
	if recommendIGPUGraphics(80) {
		t.Fatal("exactly 80 FPS recommended iGPU graphics")
	}
	if recommendIGPUGraphics(120) {
		t.Fatal("120 FPS recommended iGPU graphics")
	}
	if !recommendIGPUGraphics(79.9) {
		t.Fatal("frame rate below 80 FPS did not recommend iGPU graphics")
	}
	if recommendIGPUGraphics(0) {
		t.Fatal("unavailable frame rate recommended iGPU graphics")
	}
	if got := graphicsBenchmarkRecommendedPreset(graphicsBenchmarkResult{}); got != "Full Graphics" {
		t.Fatalf("fast result preset = %q, want Full Graphics", got)
	}
	if got := graphicsBenchmarkRecommendedPreset(graphicsBenchmarkResult{RecommendIGPU: true}); got != "iGPU Graphics" {
		t.Fatalf("slow result preset = %q, want iGPU Graphics", got)
	}
	if got := graphicsBenchmarkRecommendedLabel(graphicsBenchmarkResult{}); got != "Full Quality (Recommended)" {
		t.Fatalf("fast result label = %q", got)
	}
	if got := graphicsBenchmarkRecommendedLabel(graphicsBenchmarkResult{RecommendIGPU: true}); got != "iGPU Graphics (Recommended)" {
		t.Fatalf("slow result label = %q", got)
	}
}
