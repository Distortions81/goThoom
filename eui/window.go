package eui

import (
	"log"
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
		target.AutoSize = false
	}

	// Closed windows shouldn't steal focus, so add them to the back by
	// default and don't update the active window.
	if !target.Open {
		toBack = true
	}

	if !toBack {
		windows = append(windows, target)
	} else {
		windows = append([]*windowData{target}, windows...)
	}
}

// RemoveWindow removes a window from the active list. Any cached images
// belonging to the window are disposed and pointers cleared.
func (target *windowData) RemoveWindow() {
	for i, win := range windows {
		if win == target { // Compare pointers
			windows = append(windows[:i], windows[i+1:]...)
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
	return &newItem, h
}

// Create a new textbox from the default theme
func NewText() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Text
	h := newHandler()
	newItem.Handler = h
	return &newItem, h
}

// Bring a window to the front
func (target *windowData) BringForward() {
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
}

// Send a window to the back
func (target *windowData) ToBack() {
	for w, win := range windows {
		if win == target {
			windows = append(windows[:w], windows[w+1:]...)
			windows = append([]*windowData{target}, windows...)
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
