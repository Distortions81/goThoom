package main

import "testing"

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
