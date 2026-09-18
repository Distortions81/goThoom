package eui

type buttonColorOverride struct {
	normal, hover Color
}

// SetButtonColors sets action-specific colors while retaining the current
// theme's text contrast, geometry, and disabled appearance.
func (item *itemData) SetButtonColors(normal, hover Color) {
	item.buttonColors = &buttonColorOverride{normal: normal, hover: hover}
	item.markDirty()
}
