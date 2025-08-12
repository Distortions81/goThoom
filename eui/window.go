package eui

import (
	"log"
	"reflect"
)

// Merge one struct into another
func mergeData(original interface{}, updates interface{}) interface{} {
	// Ensure both original and updates are pointers to structs
	origVal := reflect.ValueOf(original)
	updVal := reflect.ValueOf(updates)

	// Check that both are pointers to structs
	if origVal.Kind() != reflect.Ptr || updVal.Kind() != reflect.Ptr {
		panic("Both original and updates must be pointers to structs")
	}

	// Get the elements (dereference the pointers)
	origVal = origVal.Elem()
	updVal = updVal.Elem()

	// Ensure that after dereferencing, both are structs
	if origVal.Kind() != reflect.Struct || updVal.Kind() != reflect.Struct {
		panic("Both original and updates must be structs")
	}

	// Iterate through the fields of the updates struct
	for i := 0; i < updVal.NumField(); i++ {
		origField := origVal.Field(i)
		updField := updVal.Field(i)

		if !origField.CanSet() {
			continue
		}

		// Booleans default to the theme value when false so callers
		// can omit them without overwriting defaults. Explicit true
		// values are still applied.
		if updField.Kind() == reflect.Bool {
			if updField.Bool() {
				origField.Set(updField)
			}
			continue
		}

		// Check if the update field has a non-zero value
		if !isZeroValue(updField) {
			// Set the original field to the update field's value
			origField.Set(updField)
		}
	}

	return original
}

func isZeroValue(value reflect.Value) bool {
	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}

func stripWindowColors(w *windowData) {
	w.BGColor = Color{}
	w.TitleBGColor = Color{}
	w.TitleColor = Color{}
	w.TitleTextColor = Color{}
	w.BorderColor = Color{}
	w.SizeTabColor = Color{}
	w.DragbarColor = Color{}
	w.CloseBGColor = Color{}
	w.HoverTitleColor = Color{}
	w.HoverColor = Color{}
	w.ActiveColor = Color{}
}

func stripItemColors(it *itemData) {
	it.TextColor = Color{}
	it.Color = Color{}
	it.HoverColor = Color{}
	it.ClickColor = Color{}
	it.OutlineColor = Color{}
	it.DisabledColor = Color{}
	it.SelectedColor = Color{}
}

// Add window to window list
func (target *windowData) AddWindow(toBack bool) {
	for _, win := range windows {
		if win == target {
			log.Println("Window already exists")
			return
		}
	}

	if target.PinTo != PIN_TOP_LEFT {
		target.Movable = false
	}

	if target.AutoSize {
		target.updateAutoSize()
		target.AutoSize = false
	}

	target.clampToScreen()

	if currentTheme != nil {
		applyThemeToWindow(target)
	}

	// Closed windows shouldn't steal focus, so add them to the back by
	// default and don't update the active window.
	if !target.Open {
		toBack = true
	}

	if !toBack {
		windows = append(windows, target)
		if target.PinTo == PIN_TOP_LEFT {
			activeWindow = target
		}
	} else {
		windows = append([]*windowData{target}, windows...)
	}
}

// RemoveWindow removes a window from the active list. Any cached images
// belonging to the window are disposed and pointers cleared.
func (target *windowData) RemoveWindow() {
	for i, win := range windows {
		if win == target { // Compare pointers
			win.disposeImages()
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

// Get window position, considering pinned position
func (pin pinType) getWinPosition(win *windowData) point {
	switch pin {
	case PIN_TOP_LEFT:
		return win.GetPos()
	case PIN_TOP_RIGHT:
		return point{X: float32(screenWidth) - win.GetSize().X - win.GetPos().X, Y: win.GetPos().Y}
	case PIN_TOP_CENTER:
		return point{X: float32(screenWidth/2) - win.GetSize().X/2 + win.GetPos().X, Y: win.GetPos().Y}
	case PIN_MID_LEFT:
		return point{X: win.GetPos().X, Y: float32(screenHeight/2) - win.GetSize().Y/2 + win.GetPos().Y}
	case PIN_MID_CENTER:
		return point{X: float32(screenWidth/2) - win.GetSize().X/2 + win.GetPos().X, Y: float32(screenHeight/2) - win.GetSize().Y/2 + win.GetPos().Y}
	case PIN_MID_RIGHT:
		return point{X: float32(screenWidth) - win.GetSize().X - win.GetPos().X, Y: float32(screenHeight/2) - win.GetSize().Y/2 + win.GetPos().Y}
	case PIN_BOTTOM_LEFT:
		return point{X: win.GetPos().X, Y: float32(screenHeight) - win.GetSize().Y - win.GetPos().Y}
	case PIN_BOTTOM_CENTER:
		return point{X: float32(screenWidth/2) - (win.GetSize().X / 2) + win.GetPos().X, Y: float32(screenHeight) - win.GetSize().Y - win.GetPos().Y}
	case PIN_BOTTOM_RIGHT:
		return point{X: float32(screenWidth) - win.GetSize().X - win.GetPos().X, Y: float32(screenHeight) - win.GetSize().Y - win.GetPos().Y}
	default:
		return win.GetPos()
	}
}

// Get item position, considering its pinned position
func (pin pinType) getItemPosition(win *windowData, item *itemData) point {
	switch pin {
	case PIN_TOP_LEFT:
		return item.Position
	case PIN_TOP_RIGHT:
		return point{
			X: float32(win.GetSize().X) - item.GetSize().X - item.GetPos().X,
			Y: item.GetPos().Y,
		}
	case PIN_TOP_CENTER:
		return point{
			X: float32(win.GetSize().X)/2 - item.GetSize().X/2 + item.GetPos().X,
			Y: item.GetPos().Y,
		}
	case PIN_MID_LEFT:
		return point{
			X: item.GetPos().X,
			Y: float32(win.GetSize().Y)/2 - item.GetSize().Y/2 + item.GetPos().Y,
		}
	case PIN_MID_CENTER:
		return point{
			X: float32(win.GetSize().X)/2 - item.GetSize().X/2 + item.GetPos().X,
			Y: float32(win.GetSize().Y)/2 - item.GetSize().Y/2 + item.GetPos().Y,
		}
	case PIN_MID_RIGHT:
		return point{
			X: float32(win.GetSize().X) - item.GetSize().X - item.GetPos().X,
			Y: float32(win.GetSize().Y)/2 - item.GetSize().Y/2 + item.GetPos().Y,
		}
	case PIN_BOTTOM_LEFT:
		return point{
			X: item.GetPos().X,
			Y: float32(win.GetSize().Y) - win.GetTitleSize() - item.GetSize().Y - item.GetPos().Y,
		}
	case PIN_BOTTOM_CENTER:
		return point{
			X: float32(win.GetSize().X)/2 - item.GetSize().X/2 + item.GetPos().X,
			Y: float32(win.GetSize().Y) - win.GetTitleSize() - item.GetSize().Y - item.GetPos().Y,
		}
	case PIN_BOTTOM_RIGHT:
		return point{
			X: float32(win.GetSize().X) - item.GetSize().X - item.GetPos().X,
			Y: float32(win.GetSize().Y) - win.GetTitleSize() - item.GetSize().Y - item.GetPos().Y,
		}
	default:
		return item.GetPos()
	}
}

// getOverlayItemPosition returns the screen position for an item pinned without a window
func (pin pinType) getOverlayItemPosition(item *itemData) point {
	switch pin {
	case PIN_TOP_LEFT:
		return item.GetPos()
	case PIN_TOP_RIGHT:
		return point{X: float32(screenWidth) - item.GetSize().X - item.GetPos().X, Y: item.GetPos().Y}
	case PIN_TOP_CENTER:
		return point{X: float32(screenWidth)/2 - item.GetSize().X/2 + item.GetPos().X, Y: item.GetPos().Y}
	case PIN_MID_LEFT:
		return point{X: item.GetPos().X, Y: float32(screenHeight)/2 - item.GetSize().Y/2 + item.GetPos().Y}
	case PIN_MID_CENTER:
		return point{X: float32(screenWidth)/2 - item.GetSize().X/2 + item.GetPos().X, Y: float32(screenHeight)/2 - item.GetSize().Y/2 + item.GetPos().Y}
	case PIN_MID_RIGHT:
		return point{X: float32(screenWidth) - item.GetSize().X - item.GetPos().X, Y: float32(screenHeight)/2 - item.GetSize().Y/2 + item.GetPos().Y}
	case PIN_BOTTOM_LEFT:
		return point{X: item.GetPos().X, Y: float32(screenHeight) - item.GetSize().Y - item.GetPos().Y}
	case PIN_BOTTOM_CENTER:
		return point{X: float32(screenWidth)/2 - item.GetSize().X/2 + item.GetPos().X, Y: float32(screenHeight) - item.GetSize().Y - item.GetPos().Y}
	case PIN_BOTTOM_RIGHT:
		return point{X: float32(screenWidth) - item.GetSize().X - item.GetPos().X, Y: float32(screenHeight) - item.GetSize().Y - item.GetPos().Y}
	default:
		return item.GetPos()
	}
}

func (win *windowData) getPosition() point {
	pos := win.PinTo.getWinPosition(win)
	m := win.Margin * uiScale

	switch win.PinTo {
	case PIN_TOP_RIGHT, PIN_MID_RIGHT, PIN_BOTTOM_RIGHT:
		pos.X -= m
	case PIN_TOP_LEFT, PIN_MID_LEFT, PIN_BOTTOM_LEFT:
		pos.X += m
	}

	switch win.PinTo {
	case PIN_BOTTOM_LEFT, PIN_BOTTOM_CENTER, PIN_BOTTOM_RIGHT:
		pos.Y -= m
	case PIN_TOP_LEFT, PIN_TOP_CENTER, PIN_TOP_RIGHT:
		pos.Y += m
	}

	return pos
}

func (item *itemData) getPosition(win *windowData) point {
	pos := item.PinTo.getItemPosition(win, item)
	m := item.Margin * uiScale

	switch item.PinTo {
	case PIN_TOP_RIGHT, PIN_MID_RIGHT, PIN_BOTTOM_RIGHT:
		pos.X -= m
	case PIN_TOP_LEFT, PIN_MID_LEFT, PIN_BOTTOM_LEFT:
		pos.X += m
	}

	switch item.PinTo {
	case PIN_BOTTOM_LEFT, PIN_BOTTOM_CENTER, PIN_BOTTOM_RIGHT:
		pos.Y -= m
	case PIN_TOP_LEFT, PIN_TOP_CENTER, PIN_TOP_RIGHT:
		pos.Y += m
	}

	return pos
}

func (item *itemData) getOverlayPosition() point {
	pos := item.PinTo.getOverlayItemPosition(item)
	m := item.Margin * uiScale

	switch item.PinTo {
	case PIN_TOP_RIGHT, PIN_MID_RIGHT, PIN_BOTTOM_RIGHT:
		pos.X -= m
	case PIN_TOP_LEFT, PIN_MID_LEFT, PIN_BOTTOM_LEFT:
		pos.X += m
	}

	switch item.PinTo {
	case PIN_BOTTOM_LEFT, PIN_BOTTOM_CENTER, PIN_BOTTOM_RIGHT:
		pos.Y -= m
	case PIN_TOP_LEFT, PIN_TOP_CENTER, PIN_TOP_RIGHT:
		pos.Y += m
	}

	return pos
}

// Do the window items overlap?
func (win windowData) itemOverlap(size point) (bool, bool) {

	rectList := []rect{}

	win.Size = size

	for _, item := range win.Contents {
		rectList = append(rectList, item.getItemRect(&win))
	}

	xc, yc := false, false
	for _, ra := range rectList {
		for _, rb := range rectList {
			if ra == rb {
				continue
			}

			if ra.containsPoint(point{X: rb.X0, Y: rb.Y0}) {
				xc = true
			}
			if ra.containsPoint(point{X: rb.X1, Y: rb.Y1}) {
				yc = true
			}
		}
	}

	return xc, yc
}
