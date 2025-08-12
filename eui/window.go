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

	// Iterate through the fields of the updates struct by name so that
	// structs with mismatched layouts can be merged safely.
	for i := 0; i < updVal.NumField(); i++ {
		field := updVal.Type().Field(i)
		origField := origVal.FieldByName(field.Name)
		if !origField.IsValid() || !origField.CanSet() {
			// Skip fields that don't exist in the original struct
			// or can't be set instead of panicking.
			continue
		}
		updField := updVal.Field(i)

		// Boolean handling: detect whether the field was explicitly
		// provided so callers can set values to false. Two mechanisms
		// are supported:
		//   1. Pointer-to-bool fields. A nil pointer means the field
		//      was omitted. A non-nil pointer is dereferenced and the
		//      value applied regardless of true/false.
		//   2. A companion "FieldNameSet" bool field. When present and
		//      true, the associated bool field is applied even if the
		//      value is false.
		if updField.Kind() == reflect.Ptr && updField.Type().Elem().Kind() == reflect.Bool {
			if !updField.IsNil() && origField.Kind() == reflect.Bool {
				origField.SetBool(updField.Elem().Bool())
			}
			continue
		}
		if updField.Kind() == reflect.Bool {
			setField := updVal.FieldByName(field.Name + "Set")
			set := setField.IsValid() && setField.Kind() == reflect.Bool && setField.Bool()
			if set || updField.Bool() {
				origField.SetBool(updField.Bool())
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
	return value.IsZero()
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
	if target == nil {
		log.Println("AddWindow: target is nil")
		return
	}

	for _, win := range windows {
		if win == target {
			if toBack {
				target.ToBack()
			} else {
				target.BringForward()
			}
			return
		}
	}

	if target.PinTo != PIN_TOP_LEFT {
		target.Movable = false
	}

	if target.NoTitle {
		target.NoTitleSet = true
		target.TitleHeightSet = true
		target.TitleHeight = 0
	} else if target.TitleHeight > 0 {
		target.TitleHeightSet = true
	}
	if currentTheme != nil {
		applyThemeToWindow(target)
	}
	if target.NoTitle {
		target.TitleHeight = 0
	}

	if target.AutoSize {
		target.updateAutoSize()
		target.AutoSizeOnScale = true
		target.AutoSize = false
	}

	if target.Size.X <= 0 || target.Size.Y <= 0 {
		log.Printf("AddWindow: rejecting window with non-positive size: %+v", target.Size)
		return
	}

	target.clampToScreen()

	// Closed windows shouldn't steal focus, so add them to the back by
	// default and don't update the active window.
	if !target.open {
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
	if target == nil {
		log.Println("RemoveWindow: target is nil")
		return
	}

	for i, win := range windows {
		if win == target { // Compare pointers
			win.deallocImages()
			windows = append(windows[:i], windows[i+1:]...)
			win.open = false
			if activeWindow == target {
				activeWindow = nil
				for j := len(windows) - 1; j >= 0; j-- {
					if windows[j].open {
						activeWindow = windows[j]
						break
					}
				}
			}
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
	stripWindowColors(&newWindow)
	if newWindow.Theme == nil {
		newWindow.Theme = currentTheme
	}
	return &newWindow
}

// Create a new button from the default theme
func NewButton() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Button
	stripItemColors(&newItem)
	if newItem.Theme == nil {
		newItem.Theme = currentTheme
	}
	newItem.ItemType = ITEM_BUTTON
	h := newHandler()
	newItem.Handler = h
	return &newItem, h
}

// Create a new checkbox from the default theme
func NewCheckbox() (*itemData, *EventHandler) {
	if currentTheme == nil {
		currentTheme = baseTheme
	}
	newItem := currentTheme.Checkbox
	stripItemColors(&newItem)
	if newItem.Theme == nil {
		newItem.Theme = currentTheme
	}
	newItem.ItemType = ITEM_CHECKBOX
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
	stripItemColors(&newItem)
	if newItem.Theme == nil {
		newItem.Theme = currentTheme
	}
	newItem.ItemType = ITEM_RADIO
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
	stripItemColors(&newItem)
	if newItem.Theme == nil {
		newItem.Theme = currentTheme
	}
	newItem.ItemType = ITEM_INPUT
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
	stripItemColors(&newItem)
	if newItem.Theme == nil {
		newItem.Theme = currentTheme
	}
	newItem.ItemType = ITEM_SLIDER
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
	stripItemColors(&newItem)
	if newItem.Theme == nil {
		newItem.Theme = currentTheme
	}
	newItem.ItemType = ITEM_DROPDOWN
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
	stripItemColors(&newItem)
	if ac, ok := namedColors["accent"]; ok && newItem.WheelColor == (Color{}) {
		newItem.WheelColor = ac
	}
	if newItem.Theme == nil {
		newItem.Theme = currentTheme
	}
	newItem.ItemType = ITEM_COLORWHEEL
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
	stripItemColors(&newItem)
	if newItem.Theme == nil {
		newItem.Theme = currentTheme
	}
	newItem.ItemType = ITEM_TEXT
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
			return
		}
	}
}

// MarkOpen sets the window to open and brings it forward if necessary.
func (target *windowData) MarkOpen() {
	target.open = true
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
	if target.Dirty {
		target.Refresh()
	}
	target.clampToScreen()
}

// MarkClosed marks the window as closed and updates the active window.
func (target *windowData) MarkClosed() {
	target.open = false
	if activeWindow == target {
		activeWindow = nil
		for j := len(windows) - 1; j >= 0; j-- {
			if windows[j].open {
				activeWindow = windows[j]
				break
			}
		}
	}
}

// Send a window to the back
func (target *windowData) ToBack() {
	for w, win := range windows {
		if win == target {
			windows = append(windows[:w], windows[w+1:]...)
			windows = append([]*windowData{target}, windows...)
			if activeWindow == target {
				numWindows := len(windows)
				if numWindows > 0 {
					activeWindow = windows[numWindows-1]
				}
			}
			return
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
		return point{X: float32(screenWidth)/2 - win.GetSize().X/2 + win.GetPos().X, Y: win.GetPos().Y}
	case PIN_MID_LEFT:
		return point{X: win.GetPos().X, Y: float32(screenHeight)/2 - win.GetSize().Y/2 + win.GetPos().Y}
	case PIN_MID_CENTER:
		return point{X: float32(screenWidth)/2 - win.GetSize().X/2 + win.GetPos().X, Y: float32(screenHeight)/2 - win.GetSize().Y/2 + win.GetPos().Y}
	case PIN_MID_RIGHT:
		return point{X: float32(screenWidth) - win.GetSize().X - win.GetPos().X, Y: float32(screenHeight)/2 - win.GetSize().Y/2 + win.GetPos().Y}
	case PIN_BOTTOM_LEFT:
		return point{X: win.GetPos().X, Y: float32(screenHeight) - win.GetSize().Y - win.GetPos().Y}
	case PIN_BOTTOM_CENTER:
		return point{X: float32(screenWidth)/2 - (win.GetSize().X / 2) + win.GetPos().X, Y: float32(screenHeight) - win.GetSize().Y - win.GetPos().Y}
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

// Refresh forces the window to recalculate layout, resize to its contents,
// and adjust scrolling after modifying contents.
func (win *windowData) Refresh() {
	if !win.open {
		for _, it := range win.Contents {
			markItemTreeDirty(it)
		}
		win.Dirty = true
		return
	}
	win.resizeFlows()
	if win.AutoSize {
		win.updateAutoSize()
	} else {
		win.clampToScreen()
	}
	win.adjustScrollForResize()
	for _, it := range win.Contents {
		markItemTreeDirty(it)
	}
	win.Dirty = true
}
