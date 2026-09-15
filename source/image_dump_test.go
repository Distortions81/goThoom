package main

import (
	"image"
	"testing"
)

func TestImageDumpFrameCount(t *testing.T) {
	tests := []struct {
		name        string
		frames      int
		singleFrame bool
		want        int
	}{
		{name: "all animation frames", frames: 7, want: 7},
		{name: "single animation frame", frames: 7, singleFrame: true, want: 1},
		{name: "static image", frames: 1, want: 1},
		{name: "invalid frame count", frames: 0, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := imageDumpFrameCount(test.frames, test.singleFrame); got != test.want {
				t.Fatalf("imageDumpFrameCount(%d, %v) = %d, want %d", test.frames, test.singleFrame, got, test.want)
			}
		})
	}
}

func TestImageDumpMobilePoseRect(t *testing.T) {
	tests := []struct {
		name                string
		width, height       int
		frames              int
		wantRect            image.Rectangle
		wantMobilePoseSheet bool
	}{
		{name: "three-row mobile", width: 512, height: 96, frames: 1, wantRect: image.Rect(128, 0, 160, 32), wantMobilePoseSheet: true},
		{name: "four-row mobile", width: 608, height: 152, frames: 1, wantRect: image.Rect(152, 0, 190, 38), wantMobilePoseSheet: true},
		{name: "square tiled floor", width: 400, height: 400, frames: 1},
		{name: "ordinary animated image", width: 512, height: 96, frames: 4},
		{name: "non-pose strip", width: 512, height: 1280, frames: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rect, ok := imageDumpMobilePoseRect(test.width, test.height, test.frames)
			if rect != test.wantRect || ok != test.wantMobilePoseSheet {
				t.Fatalf("imageDumpMobilePoseRect(%d, %d, %d) = (%v, %v), want (%v, %v)",
					test.width, test.height, test.frames, rect, ok, test.wantRect, test.wantMobilePoseSheet)
			}
		})
	}
}

func TestImageDumpFrameFilename(t *testing.T) {
	if got := imageDumpFrameFilename(123, 0, 1); got != "123.png" {
		t.Fatalf("static image filename = %q, want %q", got, "123.png")
	}
	if got := imageDumpFrameFilename(123, 0, 7); got != "123_0.png" {
		t.Fatalf("first animation frame filename = %q, want %q", got, "123_0.png")
	}
	if got := imageDumpFrameFilename(123, 6, 7); got != "123_6.png" {
		t.Fatalf("animation frame filename = %q, want %q", got, "123_6.png")
	}
}
