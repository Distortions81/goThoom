package main

import (
	"testing"

	"gothoom/eui"

	"golang.org/x/image/font/gofont/goregular"
)

func TestPopupMessageTextIsSelectable(t *testing.T) {
	if err := eui.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatal(err)
	}
	win := showPopup("Error", "copy this diagnostic", nil)
	t.Cleanup(func() { win.Close() })

	if len(win.Contents) != 1 || len(win.Contents[0].Contents) == 0 {
		t.Fatalf("popup contents = %#v", win.Contents)
	}
	message := win.Contents[0].Contents[0]
	if !message.SelectableText {
		t.Fatal("popup message text is not selectable")
	}
}
