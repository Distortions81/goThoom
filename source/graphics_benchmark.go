package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

const graphicsBenchmarkFPS = 80.0

type graphicsBenchmarkResult struct {
	ActualFPS     float64
	RecommendIGPU bool
}

func recommendIGPUGraphics(actualFPS float64) bool {
	return actualFPS > 0 && actualFPS < graphicsBenchmarkFPS
}

func graphicsBenchmarkRecommendedPreset(result graphicsBenchmarkResult) string {
	if result.RecommendIGPU {
		return "iGPU Graphics"
	}
	return "Full Graphics"
}

func graphicsBenchmarkRecommendedLabel(result graphicsBenchmarkResult) string {
	if result.RecommendIGPU {
		return "iGPU Graphics (Recommended)"
	}
	return "Full Quality (Recommended)"
}

// runGraphicsBenchmark reads the complete renderer's existing FPS measurement.
// The setup preview is already exercising the real game and UI rendering path.
func runGraphicsBenchmark() (graphicsBenchmarkResult, error) {
	if isWASM {
		return graphicsBenchmarkResult{}, fmt.Errorf("graphics detection is unavailable in browser builds")
	}
	fps := ebiten.ActualFPS()
	if fps <= 0 {
		return graphicsBenchmarkResult{}, fmt.Errorf("frame-rate measurement is not ready; wait a moment and rerun detection")
	}
	return graphicsBenchmarkResult{
		ActualFPS:     fps,
		RecommendIGPU: recommendIGPUGraphics(fps),
	}, nil
}
