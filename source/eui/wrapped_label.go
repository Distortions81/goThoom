package eui

import "strings"

type wrappedLabelLayout struct {
	raw    string
	config textWindowWrapConfig
}

// NewWrappedLabel creates a label that wraps to width (in logical pixels) and
// grows vertically. It reflows when the width, font, or UI scale changes.
// Use SetWrappedText to change its content; Text contains the rendered lines.
func NewWrappedLabel(value string, width float32) *ItemData {
	item := NewLabel("")
	item.Size.X = width
	item.SetWrappedText(value)
	return item
}

// SetWrappedText updates a wrapped label, preserving the original paragraphs
// so widening the label can join previously wrapped lines again.
func (item *itemData) SetWrappedText(value string) {
	item.wrappedLabel = &wrappedLabelLayout{raw: value}
	item.layoutWrappedLabel()
}

func (item *itemData) layoutWrappedLabel() {
	if item.wrappedLabel == nil || item.ItemType != ITEM_TEXT || item.Size.X <= 0 {
		return
	}
	fontSize := item.FontSize
	if fontSize <= 0 {
		fontSize = 12
	}
	config := textWindowWrapConfig{
		width: float64(item.Size.X * UIScale()), faceSize: float64(fontSize*UIScale() + 2), source: FontSource(),
	}
	if item.wrappedLabel.config == config {
		return
	}
	_, lines := WrapText(item.wrappedLabel.raw, textFace(float32(config.faceSize)), config.width)
	item.Text = strings.Join(lines, "\n")
	item.wrappedLabel.config = config
	item.Dirty = true
}
