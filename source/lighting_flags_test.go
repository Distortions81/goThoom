package main

import (
	"testing"

	"gothoom/climg"
)

func TestMobileLightEnabled(t *testing.T) {
	const attackOnly = climg.PictDefFlagOnlyAttackPosesLit
	tests := []struct {
		name  string
		flags uint32
		state uint8
		want  bool
	}{
		{name: "unflagged movement pose", state: 0, want: true},
		{name: "first attack pose", flags: attackOnly, state: 3, want: true},
		{name: "second attack pose", flags: attackOnly, state: 7, want: true},
		{name: "last attack pose", flags: attackOnly, state: 31, want: true},
		{name: "movement pose", flags: attackOnly, state: 2, want: false},
		{name: "dead pose", flags: attackOnly, state: 32, want: false},
		{name: "special pose", flags: attackOnly, state: 35, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mobileLightEnabled(tt.flags, tt.state); got != tt.want {
				t.Fatalf("mobileLightEnabled(%#x, %d) = %v, want %v", tt.flags, tt.state, got, tt.want)
			}
		})
	}
}

func TestLightFlickerIsStableAndBounded(t *testing.T) {
	const (
		pictID = uint32(1234)
		key    = uint64(5678)
	)
	wantX, wantY := lightFlickerOffset(pictID, key, 42, 0.37)
	for range 10 {
		gotX, gotY := lightFlickerOffset(pictID, key, 42, 0.37)
		if gotX != wantX || gotY != wantY {
			t.Fatalf("same logical frame changed from (%v,%v) to (%v,%v)", wantX, wantY, gotX, gotY)
		}
	}

	continuous := false
	for frame := 0; frame < 256; frame++ {
		dx, dy := lightFlickerTarget(pictID, key, frame)
		if dx < -1 || dx > 1 || dy < -1 || dy > 1 {
			t.Fatalf("frame %d produced out-of-range target (%v,%v)", frame, dx, dy)
		}
		if dx != -1 && dx != 0 && dx != 1 && dy != -1 && dy != 0 && dy != 1 {
			continuous = true
		}
	}
	if !continuous {
		t.Fatal("flicker targets were limited to the original three integer positions")
	}
}

func TestLightFlickerInterpolatesLogicalFrames(t *testing.T) {
	const (
		pictID = uint32(1234)
		key    = uint64(5678)
		frame  = 42
	)
	prevX, prevY := lightFlickerTarget(pictID, key, frame-1)
	curX, curY := lightFlickerTarget(pictID, key, frame)
	startX, startY := lightFlickerOffset(pictID, key, frame, 0)
	endX, endY := lightFlickerOffset(pictID, key, frame, 1)
	if startX != prevX || startY != prevY {
		t.Fatalf("start offset = (%v,%v), want previous target (%v,%v)", startX, startY, prevX, prevY)
	}
	if endX != curX || endY != curY {
		t.Fatalf("end offset = (%v,%v), want current target (%v,%v)", endX, endY, curX, curY)
	}
}

func TestFlameLightFlicker(t *testing.T) {
	plain := flameLightFlicker(0, 1, 2, 3, 0.5, 1)
	if plain.offsetX != 0 || plain.offsetY != 0 || plain.radius != 1 || plain.brightness != 1 {
		t.Fatalf("unflagged modulation = %+v", plain)
	}

	flame := flameLightFlicker(climg.PictDefFlagLightFlicker, 1, 2, 3, 0.5, 1)
	dx, dy := lightFlickerOffset(1, 2, 3, 0.5)
	if flame.offsetX != dx || flame.offsetY != dy {
		t.Fatalf("flame offsets = (%v,%v), want (%v,%v)", flame.offsetX, flame.offsetY, dx, dy)
	}
	if flame.radius < 0.92 || flame.radius > 1.02 {
		t.Fatalf("flame radius multiplier %v is out of range", flame.radius)
	}
	if flame.brightness < 0.84 || flame.brightness > 1.04 {
		t.Fatalf("flame brightness multiplier %v is out of range", flame.brightness)
	}
	steady := flameLightFlicker(climg.PictDefFlagLightFlicker, 1, 2, 3, 0.5, 0)
	if steady.offsetX != 0 || steady.offsetY != 0 || steady.radius != 1 || steady.brightness != 1 {
		t.Fatalf("zero-strength modulation = %+v", steady)
	}
}
