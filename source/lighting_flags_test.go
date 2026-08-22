package main

import (
	"image"
	"testing"

	"gothoom/climg"
)

func TestPictureLightGeometry(t *testing.T) {
	const dark = climg.PictDefFlagLightDarkcaster
	tests := []struct {
		name           string
		metadataRadius uint16
		flags          uint32
		width, height  int
		wantRadius     float32
	}{
		{name: "default radius", width: 40, height: 20, wantRadius: 60},
		{name: "explicit radius", metadataRadius: 50, width: 40, height: 20, wantRadius: 50},
		{name: "minimum radius", metadataRadius: 5, width: 40, height: 20, wantRadius: 30},
		{name: "darkcaster radius", metadataRadius: 50, flags: dark, width: 40, height: 20, wantRadius: 200},
		{name: "default darkcaster radius", flags: dark, width: 40, height: 20, wantRadius: 240},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pictureLightGeometry(tt.metadataRadius, tt.flags, tt.width, tt.height)
			if got.radius != tt.wantRadius || got.intensity != 1 {
				t.Fatalf("pictureLightGeometry() = %+v, want radius %v intensity 1", got, tt.wantRadius)
			}
		})
	}
}

func TestMobileLightGeometry(t *testing.T) {
	const dark = climg.PictDefFlagLightDarkcaster
	tests := []struct {
		name           string
		metadataRadius uint16
		flags          uint32
		size           int
		state          uint8
		wantRadius     float32
		wantIntensity  float32
	}{
		{name: "default radius", size: 20, wantRadius: 40, wantIntensity: 1},
		{name: "explicit radius", metadataRadius: 30, size: 20, wantRadius: 30, wantIntensity: 1},
		{name: "minimum radius", metadataRadius: 5, size: 20, wantRadius: 20, wantIntensity: 1},
		{name: "darkcaster radius", metadataRadius: 30, flags: dark, size: 20, wantRadius: 120, wantIntensity: 1},
		{name: "dead emitter", metadataRadius: 60, size: 20, state: poseDead, wantRadius: 30, wantIntensity: 0.5},
		{name: "dead minimum", metadataRadius: 10, size: 20, state: poseDead, wantRadius: 20, wantIntensity: 0.5},
		{name: "dead darkcaster", metadataRadius: 30, flags: dark, size: 20, state: poseDead, wantRadius: 60, wantIntensity: 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mobileLightGeometry(tt.metadataRadius, tt.flags, tt.size, tt.state)
			if got.radius != tt.wantRadius || got.intensity != tt.wantIntensity {
				t.Fatalf("mobileLightGeometry() = %+v, want radius %v intensity %v", got, tt.wantRadius, tt.wantIntensity)
			}
		})
	}
}

func TestLightIntersectsViewport(t *testing.T) {
	bounds := image.Rect(10, 20, 110, 120)
	tests := []struct {
		name    string
		x, y, r float32
		want    bool
	}{
		{name: "inside", x: 50, y: 60, r: 10, want: true},
		{name: "touching left", x: 0, y: 60, r: 10, want: true},
		{name: "left", x: -1, y: 60, r: 10, want: false},
		{name: "touching bottom", x: 50, y: 130, r: 10, want: true},
		{name: "bottom", x: 50, y: 131, r: 10, want: false},
		{name: "zero radius", x: 50, y: 60, r: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lightIntersectsViewport(tt.x, tt.y, tt.r, bounds); got != tt.want {
				t.Fatalf("lightIntersectsViewport(%v, %v, %v) = %v, want %v", tt.x, tt.y, tt.r, got, tt.want)
			}
		})
	}
}

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
