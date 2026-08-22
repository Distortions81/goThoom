package climg

import "testing"

func TestIsSemiTransparent(t *testing.T) {
	c := &CLImages{idrefs: map[uint32]*dataLocation{
		1: {flags: pictDefFlagTransparent},
		2: {flags: 1},
		3: {flags: pictDefFlagTransparent | 2},
		4: {flags: 0},
	}}

	if c.IsSemiTransparent(1) {
		t.Fatalf("id 1: expected not semi-transparent")
	}
	if !c.IsSemiTransparent(2) {
		t.Fatalf("id 2: expected semi-transparent")
	}
	if !c.IsSemiTransparent(3) {
		t.Fatalf("id 3: expected semi-transparent")
	}
	if c.IsSemiTransparent(4) {
		t.Fatalf("id 4: expected not semi-transparent")
	}
	if c.IsSemiTransparent(5) {
		t.Fatalf("missing id: expected not semi-transparent")
	}
}
