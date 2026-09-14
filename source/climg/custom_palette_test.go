package climg

import (
	"reflect"
	"testing"
)

func TestRemapCustomPaletteInfluenceDeltasUsesLastSuppliedDuplicate(t *testing.T) {
	direct := []float32{
		1, 2, 3, 4,
		5, 6, 7, 8,
	}
	got := remapCustomPaletteInfluenceDeltas(
		[]byte{12, 12, 12},
		direct,
		2,
		3,
	)
	want := []float32{
		0, 0, 0, 0,
		0, 0, 0, 0,
		5, 6, 7, 8,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped deltas = %v, want %v", got, want)
	}
}

func TestRemapCustomPaletteInfluenceDeltasPreservesOtherColors(t *testing.T) {
	direct := []float32{
		1, 2, 3, 4,
		5, 6, 7, 8,
	}
	got := remapCustomPaletteInfluenceDeltas(
		[]byte{12, 13, 12},
		direct,
		2,
		3,
	)
	want := []float32{
		0, 0, 0, 0,
		5, 6, 7, 8,
		1, 2, 3, 4,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped deltas = %v, want %v", got, want)
	}
}
