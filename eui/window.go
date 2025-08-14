package eui

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

// Add window to window list
func (target *windowData) AddWindow(toBack bool) {
	for _, win := range windows {
		if win == target {
			log.Println("Window already exists")
			return
		}
	}

	if target.AutoSize {
		target.updateAutoSize()
		if target.zone != nil {
			// Re-center to the chosen zone after size changes
			target.updateZonePosition()
		}
	}

	// Closed windows shouldn't steal focus, so add them to the back by
	// default and don't update the active window.
	if !target.Open {
		toBack = true
	}

	if target.AlwaysDrawFirst {
		windows = append([]*windowData{target}, windows...)
		return
	}

	if !toBack {
		windows = append(windows, target)
	} else {
		idx := 0
		for idx < len(windows) && windows[idx].AlwaysDrawFirst {
			idx++
		}
		windows = append(windows[:idx], append([]*windowData{target}, windows[idx:]...)...)
	}
	if windowTiling && target.Open {
		target.clampToScreen()
		preventOverlap(target)
	}
}

// deallocate releases cached render images for the window and its items.
func (target *windowData) deallocate() {
	if target.Render != nil {
		target.Render.Deallocate()
		target.Render = nil
	}
	for _, item := range target.Contents {
		item.deallocate()
	}
}

// deallocate releases cached render images for the item and its children.
func (item *itemData) deallocate() {
	if item.Render != nil {
		item.Render.Deallocate()
		item.Render = nil
	}
	for _, child := range item.Contents {
		child.deallocate()
	}
}

// RemoveWindow removes a window from the active list. Any cached images
// belonging to the window are disposed and pointers cleared.
func (target *windowData) RemoveWindow() {
	for i, win := range windows {
		if win == target { // Compare pointers
			windows = append(windows[:i], windows[i+1:]...)
			win.deallocate()
			win.Open = false
			return
		}
	}

	log.Println("Window not found")
}

// Create a new window from the default theme
func NewWindow() *windowData {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newWindow := currentTheme.Window
	newWindow.Theme = currentTheme
	return &newWindow
}

// Create a new button from the default theme
func NewButton() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Button
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = currentTheme
	return &newItem, h
}

// Create a new button from the default theme
func NewCheckbox() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Checkbox
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = currentTheme
	return &newItem, h
}

// Create a new radio button from the default theme
func NewRadio() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Radio
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = currentTheme
	return &newItem, h
}

// Create a new input box from the default theme
func NewInput() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Input
	if newItem.TextPtr == nil {
		newItem.TextPtr = &newItem.Text
	} else {
		*newItem.TextPtr = newItem.Text
	}
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = currentTheme
	return &newItem, h
}

// Create a new slider from the default theme
func NewSlider() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Slider
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = currentTheme
	return &newItem, h
}

// Create a new dropdown from the default theme
func NewDropdown() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Dropdown
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = currentTheme
	return &newItem, h
}

// Create a new color wheel from the default theme
func NewColorWheel() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := baseColorWheel
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = currentTheme
	return &newItem, h
}

// Create a new image item with a new image buffer
func NewImageItem(w, h int) (*itemData, *ebiten.Image) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := itemData{
		ItemType: ITEM_IMAGE,
		Size:     point{X: float32(w), Y: float32(h)},
		Theme:    currentTheme,
	}
	newItem.Image = newImage(w, h)
	return &newItem, newItem.Image
}

// Create a new textbox from the default theme
func NewText() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Text
	h := newHandler()
	newItem.Handler = h
	newItem.Theme = currentTheme
	return &newItem, h
}

// Bring a window to the front
func (target *windowData) BringForward() {
	if target.AlwaysDrawFirst {
		return
	}
	for w, win := range windows {
		if win == target {
			windows = append(windows[:w], windows[w+1:]...)
			windows = append(windows, target)
			activeWindow = target
		}
	}
}

// MarkOpen sets the window to open and brings it forward if necessary.
func (target *windowData) MarkOpen() {
	target.Open = true
	found := false
	for _, win := range windows {
		if win == target {
			found = true
			break
		}
	}
	if !found {
		target.AddWindow(false)
	} else {
		target.BringForward()
	}
	target.Refresh()
	if WindowStateChanged != nil {
		WindowStateChanged()
	}
}

// MarkOpen sets the window to open and brings it forward if necessary.
func (target *windowData) Toggle() {
	if target.Open {
		target.Close()
	} else {
		target.MarkOpen()
	}
}

func (target *windowData) Close() {
	target.Open = false
	if target.OnClose != nil {
		target.OnClose()
	}
	target.deallocate()
	//target.RemoveWindow()
	if WindowStateChanged != nil {
		WindowStateChanged()
	}
}

// MarkOpenNear opens the window and attempts to place it near the given
// anchor item (typically the button that triggered the open). The window is
// positioned adjacent to the anchor while trying to avoid overlapping the
// anchor's parent window when possible and clamping to screen bounds.
func (target *windowData) MarkOpenNear(anchor *itemData) {
	// Respect explicit zone pinning: if a window has a zone, open it at
	// the zone rather than near the anchor.
	if target.zone != nil {
		target.MarkOpen()
		return
	}
	if anchor != nil {
		placeWindowNear(target, anchor)
	}
	target.MarkOpen()
}

// ToggleNear toggles the window's open state. When opening, it places the
// window near the given anchor item as in MarkOpenNear.
func (target *windowData) ToggleNear(anchor *itemData) {
	if target.Open {
		target.Close()
		return
	}
	// Respect zone pinning if set.
	if target.zone != nil {
		target.MarkOpen()
	} else {
		target.MarkOpenNear(anchor)
	}
}

// placeWindowNear computes a good position for win next to anchor and moves it
// there, preferring not to overlap the anchor's parent window if possible.
func placeWindowNear(win *windowData, anchor *itemData) {
	if win == nil || anchor == nil {
		return
	}
	// Anchor screen rect
	ar := anchor.DrawRect
	// Candidate positions: right, below, left, above
	gap := float32(8) * UIScale()
	size := win.GetSize()
	candidates := []point{
		{X: ar.X1 + gap, Y: ar.Y0},          // right
		{X: ar.X0, Y: ar.Y1 + gap},          // below
		{X: ar.X0 - size.X - gap, Y: ar.Y0}, // left
		{X: ar.X0, Y: ar.Y0 - size.Y - gap}, // above
	}

	// Parent window rect to avoid overlapping.
	var parentRect rect
	hasParent := false
	if anchor.ParentWindow != nil {
		parentRect = anchor.ParentWindow.getWinRect()
		hasParent = true
	}

	// Helper to test overlap area after clamping.
	bestIdx := -1
	var bestOverlap float32 = -1
	for i, c := range candidates {
		// Attempt to set position (this clamps to screen)
		win.SetPos(Point{X: c.X, Y: c.Y})
		// Compute resulting rect
		wp := win.getPosition()
		wr := rect{X0: wp.X, Y0: wp.Y, X1: wp.X + size.X, Y1: wp.Y + size.Y}
		var overlap float32
		if hasParent {
			inter := intersectRect(wr, parentRect)
			if inter.X1 > inter.X0 && inter.Y1 > inter.Y0 {
				overlap = (inter.X1 - inter.X0) * (inter.Y1 - inter.Y0)
			} else {
				overlap = 0
			}
		} else {
			overlap = 0
		}
		if overlap == 0 {
			// Perfect candidate: no overlap with source window
			bestIdx = i
			bestOverlap = 0
			break
		}
		if bestIdx == -1 || overlap < bestOverlap {
			bestIdx = i
			bestOverlap = overlap
		}
	}

	// Apply best candidate if we didn't already via SetPos loop
	if bestIdx >= 0 {
		c := candidates[bestIdx]
		win.SetPos(Point{X: c.X, Y: c.Y})
	}
}

// Send a window to the back
func (target *windowData) ToBack() {
	for w, win := range windows {
		if win == target {
			windows = append(windows[:w], windows[w+1:]...)
			idx := 0
			for idx < len(windows) && windows[idx].AlwaysDrawFirst {
				idx++
			}
			if target.AlwaysDrawFirst {
				idx = 0
			}
			windows = append(windows[:idx], append([]*windowData{target}, windows[idx:]...)...)
		}
	}
	if activeWindow == target {
		numWindows := len(windows)
		if numWindows > 0 {
			activeWindow = windows[numWindows-1]
		}
	}
}

func (win *windowData) getPosition() point {
	return win.Position
}

func (item *itemData) getPosition(win *windowData) point {

	return item.Position
}
