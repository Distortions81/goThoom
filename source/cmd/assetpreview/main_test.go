package main

import (
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestMobileLikePreviewHeuristic(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		want bool
	}{
		{512, 96, true}, {512, 97, true}, {528, 100, true}, {288, 54, true},
		{200, 800, false}, {200, 200, false}, {504, 140, false}, {0, 0, false},
	} {
		if got := mobileLike(tc.w, tc.h); got != tc.want {
			t.Errorf("%dx%d = %v", tc.w, tc.h, got)
		}
	}
}

func TestThumbnailCompositesTransparencyAndPreservesAspect(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	src.SetRGBA(1, 0, color.RGBA{255, 0, 0, 255})
	dst := image.NewRGBA(image.Rect(0, 0, 20, 20))
	thumb(dst, src, src.Bounds(), dst.Bounds())
	if dst.RGBAAt(0, 0).A != 0 {
		t.Fatal("thumbnail stretched beyond centered 2:1 rectangle")
	}
	if dst.RGBAAt(0, 5).A != 255 {
		t.Fatal("transparent pixels did not reveal checkerboard")
	}
	if got := dst.RGBAAt(5, 5); got != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("nearest-neighbor pixel = %v", got)
	}
	thumb(dst, src, image.Rectangle{}, dst.Bounds()) // Empty crops must be harmless.
}

func TestWritePNGDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	if err := writePNG(path, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writePNG(path, image.NewRGBA(image.Rect(0, 0, 3, 3))); err == nil {
		t.Fatal("overwrote existing image")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("existing output changed")
	}
}

func TestLowIDCatalogIsComplete(t *testing.T) {
	data, err := os.ReadFile("../../../docs/assets/low-ids-0000-0099.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Assets []struct {
			ID         int      `json:"id"`
			Label      string   `json:"label"`
			Category   string   `json:"category"`
			Confidence string   `json:"visual_confidence"`
			Evidence   []string `json:"evidence"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Assets) != 100 {
		t.Fatalf("got %d entries", len(catalog.Assets))
	}
	for i, a := range catalog.Assets {
		if a.ID != i || a.Label == "" || a.Category == "" || len(a.Evidence) == 0 {
			t.Fatalf("incomplete or unsorted entry %d: %+v", i, a)
		}
		if a.Confidence != "high" && a.Confidence != "medium" && a.Confidence != "low" {
			t.Fatalf("invalid confidence for %d", i)
		}
	}
}
