package climg

import "testing"

func TestFrameIndexUsesAnimationTable(t *testing.T) {
	images := &CLImages{idrefs: map[uint32]*dataLocation{
		7: {
			numFrames:      4,
			numAnims:       3,
			animFrameTable: [16]int16{2, 0, 3},
		},
	}}

	want := []int{2, 0, 3, 2, 0, 3}
	for counter, expected := range want {
		if got := images.FrameIndex(7, counter); got != expected {
			t.Fatalf("FrameIndex(7, %d) = %d, want %d", counter, got, expected)
		}
	}
}

func TestRandomAnimationIsStablePerFrameAndInstance(t *testing.T) {
	images := &CLImages{idrefs: map[uint32]*dataLocation{
		7: {
			flags:          PictDefFlagRandomAnimation,
			numFrames:      4,
			numAnims:       4,
			animFrameTable: [16]int16{0, 1, 2, 3},
		},
	}}

	const firstInstance = 0x00100020
	for counter := 0; counter < 32; counter++ {
		first := images.FrameIndexForInstance(7, counter, firstInstance)
		if again := images.FrameIndexForInstance(7, counter, firstInstance); again != first {
			t.Fatalf("random frame changed during logical frame %d: %d then %d", counter, first, again)
		}
		if first < 0 || first >= 4 {
			t.Fatalf("random frame %d is outside [0, 4)", first)
		}
	}

	different := false
	for counter := 0; counter < 32; counter++ {
		first := images.FrameIndexForInstance(7, counter, firstInstance)
		second := images.FrameIndexForInstance(7, counter, 0x00300040)
		if first != second {
			different = true
			break
		}
	}
	if !different {
		t.Fatal("separate picture instances remained in lockstep")
	}
}

func TestRandomAnimationUsesAnimationTable(t *testing.T) {
	images := &CLImages{idrefs: map[uint32]*dataLocation{
		7: {
			flags:          PictDefFlagRandomAnimation,
			numFrames:      6,
			numAnims:       2,
			animFrameTable: [16]int16{1, 5},
		},
	}}

	seen := map[int]bool{}
	for counter := 0; counter < 32; counter++ {
		frame := images.FrameIndexForInstance(7, counter, 123)
		if frame != 1 && frame != 5 {
			t.Fatalf("random frame %d is not an animation-table entry", frame)
		}
		seen[frame] = true
	}
	if !seen[1] || !seen[5] {
		t.Fatalf("random animation did not select both table entries: %v", seen)
	}
}
