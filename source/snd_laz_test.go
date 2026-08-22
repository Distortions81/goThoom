package main

import (
	"math"
	"testing"
)

func TestPadDBAmplificationAndAttenuation(t *testing.T) {
	input := []int16{1000, -1000}

	neg := PadDB(input, -6)
	pos := PadDB(input, 6)

	if absInt(neg[0]) >= absInt(input[0]) || absInt(neg[1]) >= absInt(input[1]) {
		t.Fatalf("negative pad did not reduce amplitude: %v", neg)
	}

	if absInt(pos[0]) <= absInt(input[0]) || absInt(pos[1]) <= absInt(input[1]) {
		t.Fatalf("positive pad did not increase amplitude: %v", pos)
	}

	// Also verify scaling amounts roughly match expected values
	expNeg := int16(float64(input[0]) * math.Pow(10, -6/20.0))
	expPos := int16(float64(input[0]) * math.Pow(10, 6/20.0))
	if neg[0] != expNeg {
		t.Errorf("expected %d for -6dB, got %d", expNeg, neg[0])
	}
	if pos[0] != expPos {
		t.Errorf("expected %d for +6dB, got %d", expPos, pos[0])
	}
}

func absInt(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}
