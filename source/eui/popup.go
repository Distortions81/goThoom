package eui

import (
	"strings"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// PopupButton defines a button in a popup dialog.
type PopupButton struct {
	Text       string
	Width      float32
	Color      *Color
	HoverColor *Color
	Action     func()
}

// ShowPopup opens a floating dialog with extra items, a message, and buttons.
// It does not block input to other windows. Button actions run before close;
// closing removes the temporary window. Call Init or supply a font first.
func ShowPopup(title, message string, buttons []PopupButton, extras ...*ItemData) *WindowData {
	win := NewWindow()
	win.Title = title
	win.OnClose = win.RemoveWindow
	win.Closable = false
	win.Resizable = false
	win.AutoSize = true
	win.Movable = true
	win.NoScroll = true
	win.SetZone(HZoneCenter, VZoneMiddleTop)
	// Add some breathing room so text doesn't hug the border
	win.Padding = 8
	win.BorderPad = 4

	flow := NewColumn()
	// Optional extra items (e.g., images) shown above the message
	for _, ex := range extras {
		if ex != nil {
			flow.AddItem(ex)
		}
	}
	if message != "" {
		// Message (wrapped to a reasonable width)
		uiScale := UIScale()
		targetWidthPx := float64(520)
		// Add horizontal padding on both sides to avoid right-edge clipping.
		hpadPx := float64(24)
		padUnits := float32(hpadPx / float64(uiScale))
		// targetWidthUnits not used directly; inner width sets actual text width
		// Match renderer size: (FontSize*uiScale)+2
		facePx := float64(12*uiScale + 2)
		var face text.Face
		if src := FontSource(); src != nil {
			face = &text.GoTextFace{Source: src, Size: facePx}
		} else {
			face = &text.GoTextFace{Size: facePx}
		}
		// Wrap to inner width (minus horizontal padding)
		innerPx := targetWidthPx - 2*hpadPx
		if innerPx < 50 {
			innerPx = 50
		}
		_, lines := WrapText(message, face, innerPx)
		wrapped := strings.Join(lines, "\n")
		gm := face.Metrics()
		lineHpx := float64(gm.HAscent + gm.HDescent)
		if lineHpx < 14 {
			lineHpx = 14
		}
		heightUnits := float32((lineHpx*float64(len(lines)) + 8) / float64(uiScale))
		if heightUnits < 24 {
			heightUnits = 24
		}
		txt, _ := NewText()
		txt.Text = wrapped
		txt.FontSize = 12
		txt.SelectableText = true
		// Slight width fudge to avoid right-edge clipping from rounding
		fudgeUnits := float32(2.0 / float64(uiScale))
		txt.Size = Point{X: float32(innerPx/float64(uiScale)) + fudgeUnits, Y: heightUnits}
		txt.Position = Point{X: padUnits, Y: 0}
		flow.AddItem(txt)
	}

	// Buttons row
	btnRow := NewRow()
	for _, b := range buttons {
		btn, ev := NewButton()
		btn.Text = b.Text
		btn.Size = Point{X: 120, Y: 24}
		if b.Width > 0 {
			btn.Size.X = b.Width
		}
		if b.Color != nil {
			btn.Color = *b.Color
		}
		if b.HoverColor != nil {
			btn.HoverColor = *b.HoverColor
		}
		action := b.Action
		ev.Handle = func(ev UIEvent) {
			if ev.Type == EventClick {
				if action != nil {
					action()
				}
				win.Close()
			}
		}
		btnRow.AddItem(btn)
	}
	flow.AddItem(btnRow)

	win.AddItem(flow)
	win.AddWindow(false)
	win.MarkOpen()
	return win
}
