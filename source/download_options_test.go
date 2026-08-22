package main

import (
	"testing"

	"gothoom/eui"
)

func TestOptionalDownloadSelections(t *testing.T) {
	soundfont, tts := optionalDownloadSelections(nil, nil)
	if soundfont || tts {
		t.Fatalf("nil optional checkboxes should not download anything, got soundfont=%v tts=%v", soundfont, tts)
	}

	soundfont, tts = optionalDownloadSelections(&eui.ItemData{Checked: false}, &eui.ItemData{Checked: false})
	if soundfont || tts {
		t.Fatalf("unchecked optional checkboxes should not download anything, got soundfont=%v tts=%v", soundfont, tts)
	}

	soundfont, tts = optionalDownloadSelections(&eui.ItemData{Checked: true}, &eui.ItemData{Checked: true})
	if !soundfont || !tts {
		t.Fatalf("checked optional checkboxes should download both, got soundfont=%v tts=%v", soundfont, tts)
	}
}
