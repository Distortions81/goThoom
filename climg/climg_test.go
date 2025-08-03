package climg

import "testing"

func applyAlpha(idx uint16, alpha uint8, transparent bool) uint8 {
	if idx == 0 && transparent {
		return 0
	}
	return alpha
}

func TestAlphaTransparentForFlags(t *testing.T) {
	cases := []struct {
		name        string
		flags       uint32
		alpha       uint8
		transparent bool
	}{
		{"NoBlend", 0, 0xFF, false},
		{"Transparent", pictDefFlagTransparent, 0xFF, true},
		{"Blend25", 1, 0xBF, false},
		{"Blend50", 2, 0x7F, false},
		{"Blend75", 3, 0x3F, false},
		{"Transparent25", pictDefFlagTransparent | 1, 0xBF, true},
		{"Transparent50", pictDefFlagTransparent | 2, 0x7F, true},
		{"Transparent75", pictDefFlagTransparent | 3, 0x3F, true},
	}

	for _, tt := range cases {
		alpha, transparent := alphaTransparentForFlags(tt.flags)
		if alpha != tt.alpha || transparent != tt.transparent {
			t.Errorf("%v: got alpha=%#x transparent=%v", tt.name, alpha, transparent)
		}

		a0 := applyAlpha(0, alpha, transparent)
		want0 := tt.alpha
		if tt.transparent {
			want0 = 0
		}
		if a0 != want0 {
			t.Errorf("%v: index0 alpha=%#x want %#x", tt.name, a0, want0)
		}

		a1 := applyAlpha(1, alpha, transparent)
		if a1 != tt.alpha {
			t.Errorf("%v: index1 alpha=%#x want %#x", tt.name, a1, tt.alpha)
		}
	}
}
