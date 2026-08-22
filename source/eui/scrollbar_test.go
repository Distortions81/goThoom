package eui

import "testing"

func TestScrollbarThumbLength(t *testing.T) {
	tests := []struct {
		name           string
		track, content float32
		scale          float32
		want           float32
	}{
		{name: "proportional", track: 200, content: 400, scale: 1, want: 100},
		{name: "minimum", track: 200, content: 4000, scale: 1, want: minScrollbarThumbSize},
		{name: "scaled minimum", track: 200, content: 4000, scale: 2, want: minScrollbarThumbSize * 2},
		{name: "minimum limited by track", track: 30, content: 3000, scale: 1, want: 30},
		{name: "content fits", track: 200, content: 100, scale: 1, want: 200},
		{name: "empty track", track: 0, content: 100, scale: 1, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scrollbarThumbLength(tt.track, tt.content, tt.scale); got != tt.want {
				t.Fatalf("scrollbarThumbLength(%v, %v, %v) = %v, want %v", tt.track, tt.content, tt.scale, got, tt.want)
			}
		})
	}
}
