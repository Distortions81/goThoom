package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestKeyboardTestLayoutIncludesFunctionKeys(t *testing.T) {
	found := make(map[ebiten.Key]bool)
	for _, row := range keyboardTestLayout() {
		for _, spec := range row {
			found[spec.Key] = true
		}
	}
	for key := ebiten.KeyF1; key <= ebiten.KeyF24; key++ {
		if !found[key] {
			t.Errorf("keyboard test layout does not include %s", key)
		}
	}
}

func TestKeyboardTestLabelsUseBundledFontGlyphs(t *testing.T) {
	for _, row := range keyboardTestLayout() {
		for _, spec := range row {
			for _, r := range spec.Label {
				if r >= utf8.RuneSelf {
					t.Errorf("keyboard label %q contains non-ASCII glyph %q", spec.Label, r)
				}
			}
		}
	}
}

func TestKeyboardTestNavigationUsesStandardCluster(t *testing.T) {
	rows := keyboardTestNavigationLayout()
	want := [][]ebiten.Key{
		{ebiten.KeyInsert, ebiten.KeyHome, ebiten.KeyPageUp},
		{ebiten.KeyDelete, ebiten.KeyEnd, ebiten.KeyPageDown},
	}
	for rowIndex, wantRow := range want {
		if len(rows[rowIndex]) != len(wantRow) {
			t.Fatalf("navigation row %d has %d keys, want %d", rowIndex, len(rows[rowIndex]), len(wantRow))
		}
		for keyIndex, wantKey := range wantRow {
			if got := rows[rowIndex][keyIndex].Key; got != wantKey {
				t.Errorf("navigation row %d key %d = %s, want %s", rowIndex, keyIndex, got, wantKey)
			}
		}
	}
}

func TestKeyboardTestPressedNamesAreSorted(t *testing.T) {
	names := keyboardTestPressedNames([]ebiten.Key{ebiten.KeyF12, ebiten.KeyA})
	if len(names) != 2 || !strings.HasPrefix(names[0], "A") || !strings.HasPrefix(names[1], "F12") {
		t.Fatalf("names = %#v", names)
	}
}

func TestKeyboardTestMouseButtonNames(t *testing.T) {
	want := map[ebiten.MouseButton]string{
		ebiten.MouseButtonLeft:   "Left Mouse",
		ebiten.MouseButtonMiddle: "Middle Mouse",
		ebiten.MouseButtonRight:  "Right Mouse",
		ebiten.MouseButton3:      "Mouse 4",
		ebiten.MouseButton4:      "Mouse 5",
	}
	for button, wantName := range want {
		if got := keyboardTestMouseButtonName(button); got != wantName {
			t.Errorf("mouse button %d = %q, want %q", button, got, wantName)
		}
	}
}

func TestKeyboardTestWheelDirections(t *testing.T) {
	tests := []struct {
		x, y float64
		want []string
	}{
		{y: 1, want: []string{"Wheel Up"}},
		{y: -1, want: []string{"Wheel Down"}},
		{x: -1, want: []string{"Wheel Left"}},
		{x: 1, want: []string{"Wheel Right"}},
		{x: -1, y: 1, want: []string{"Wheel Up", "Wheel Left"}},
	}
	for _, test := range tests {
		got := keyboardTestWheelDirections(test.x, test.y)
		if strings.Join(got, ",") != strings.Join(test.want, ",") {
			t.Errorf("wheel (%v, %v) = %v, want %v", test.x, test.y, got, test.want)
		}
	}
}
