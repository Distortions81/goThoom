package main

import "testing"

func TestGraphicsBenchmarkRecommendation(t *testing.T) {
	if graphicsBenchmarkFPS != 80 {
		t.Fatalf("FPS limit = %v, want 80", graphicsBenchmarkFPS)
	}
	if recommendLowQuality(80) {
		t.Fatal("exactly 80 FPS recommended Low quality")
	}
	if recommendLowQuality(120) {
		t.Fatal("120 FPS recommended Low quality")
	}
	if !recommendLowQuality(79.9) {
		t.Fatal("frame rate below 80 FPS did not recommend Low quality")
	}
	if recommendLowQuality(0) {
		t.Fatal("unavailable frame rate recommended Low quality")
	}
	if got := graphicsBenchmarkRecommendedPreset(graphicsBenchmarkResult{}); got != "High" {
		t.Fatalf("fast result preset = %q, want High", got)
	}
	if got := graphicsBenchmarkRecommendedPreset(graphicsBenchmarkResult{RecommendLow: true}); got != "Low" {
		t.Fatalf("slow result preset = %q, want Low", got)
	}
	if got := graphicsBenchmarkRecommendedLabel(graphicsBenchmarkResult{}); got != "High (Recommended)" {
		t.Fatalf("fast result label = %q", got)
	}
	if got := graphicsBenchmarkRecommendedLabel(graphicsBenchmarkResult{RecommendLow: true}); got != "Low (Recommended)" {
		t.Fatalf("slow result label = %q", got)
	}
}
