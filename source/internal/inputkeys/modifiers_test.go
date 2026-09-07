package inputkeys

import "testing"

func TestEditingModifiers(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		mods                           Modifiers
		shortcut, word, line, suppress bool
	}{
		{"Mac Command", Modifiers{Mac: true, Meta: true}, true, false, true, true},
		{"Mac Option", Modifiers{Mac: true, Alt: true}, false, true, false, false},
		{"Mac Control compatibility", Modifiers{Mac: true, Control: true}, true, true, false, true},
		{"Control", Modifiers{Control: true}, true, true, false, true},
		{"Windows or Linux Meta", Modifiers{Meta: true}, false, false, false, true},
		{"AltGr text", Modifiers{Alt: true}, false, false, false, false},
		{"Windows AltGr Control+Alt", Modifiers{Control: true, Alt: true}, false, true, false, false},
		{"No modifiers", Modifiers{}, false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.mods
			if m.Shortcut() != tc.shortcut || m.Word() != tc.word || m.Line() != tc.line || m.SuppressText() != tc.suppress {
				t.Fatalf("unexpected editing modifiers: %+v", m)
			}
		})
	}
}
