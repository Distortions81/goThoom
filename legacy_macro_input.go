package main

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// legacyMacroClickEvent contains the information available at a click trigger.
// Player-list clicks deliberately leave button and chord unset, like the
// reference client's player-window path.
type legacyMacroClickEvent struct {
	Name      string
	HasName   bool
	OnPlayer  bool
	Button    int
	Chord     int
	HasButton bool
	HasChord  bool
	Modifiers legacyMacroModifiers
}

var legacyMacroInputState struct {
	sync.Mutex
	consumedKeys  map[ebiten.Key]bool
	consumedMouse map[ebiten.MouseButton]bool
	consumed      map[string]bool
}

func legacyMacroBeginInputFrame() {
	legacyMacroInputState.Lock()
	if legacyMacroInputState.consumed == nil {
		legacyMacroInputState.consumed = make(map[string]bool)
	} else {
		clear(legacyMacroInputState.consumed)
	}
	for key := range legacyMacroInputState.consumedKeys {
		if !ebiten.IsKeyPressed(key) {
			delete(legacyMacroInputState.consumedKeys, key)
		}
	}
	for button := range legacyMacroInputState.consumedMouse {
		if !ebiten.IsMouseButtonPressed(button) {
			delete(legacyMacroInputState.consumedMouse, button)
		}
	}
	legacyMacroInputState.Unlock()
}

func legacyMacroMarkInputConsumed(name string) {
	legacyMacroInputState.Lock()
	if legacyMacroInputState.consumed == nil {
		legacyMacroInputState.consumed = make(map[string]bool)
	}
	legacyMacroInputState.consumed[legacyMacroInputName(name)] = true
	legacyMacroInputState.Unlock()
}

func legacyMacroMarkKeyConsumed(key ebiten.Key, name string) {
	legacyMacroInputState.Lock()
	if legacyMacroInputState.consumedKeys == nil {
		legacyMacroInputState.consumedKeys = make(map[ebiten.Key]bool)
	}
	if legacyMacroInputState.consumed == nil {
		legacyMacroInputState.consumed = make(map[string]bool)
	}
	legacyMacroInputState.consumedKeys[key] = true
	legacyMacroInputState.consumed[legacyMacroInputName(name)] = true
	legacyMacroInputState.Unlock()
}

func legacyMacroMarkMouseConsumed(button ebiten.MouseButton, name string) {
	legacyMacroInputState.Lock()
	if legacyMacroInputState.consumedMouse == nil {
		legacyMacroInputState.consumedMouse = make(map[ebiten.MouseButton]bool)
	}
	if legacyMacroInputState.consumed == nil {
		legacyMacroInputState.consumed = make(map[string]bool)
	}
	legacyMacroInputState.consumedMouse[button] = true
	legacyMacroInputState.consumed[legacyMacroInputName(name)] = true
	legacyMacroInputState.Unlock()
}

func legacyMacroKeyConsumed(key ebiten.Key) bool {
	legacyMacroInputState.Lock()
	defer legacyMacroInputState.Unlock()
	return legacyMacroInputState.consumedKeys[key]
}

func legacyMacroMouseConsumed(button ebiten.MouseButton) bool {
	legacyMacroInputState.Lock()
	defer legacyMacroInputState.Unlock()
	return legacyMacroInputState.consumedMouse[button]
}

func legacyMacroSuppressesTypedInput() bool {
	legacyMacroInputState.Lock()
	defer legacyMacroInputState.Unlock()
	for key := range legacyMacroInputState.consumedKeys {
		if ebiten.IsKeyPressed(key) && legacyMacroKeyProducesText(key) {
			return true
		}
	}
	return false
}

func legacyMacroHotkeySuppressed(combo string) bool {
	parts := strings.Split(strings.TrimSpace(combo), "-")
	if len(parts) == 0 {
		return false
	}
	legacyMacroInputState.Lock()
	defer legacyMacroInputState.Unlock()
	name := legacyMacroInputName(parts[len(parts)-1])
	if legacyMacroInputState.consumed[name] {
		return true
	}
	return (name == "delete" && legacyMacroInputState.consumed["del"]) ||
		(name == "del" && legacyMacroInputState.consumed["delete"])
}

func legacyMacroInputName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "leftclick":
		return "click"
	case "rightclick":
		return "click2"
	case "middleclick":
		return "click3"
	case "arrowup":
		return "up"
	case "arrowdown":
		return "down"
	case "arrowleft":
		return "left"
	case "arrowright":
		return "right"
	case "enter":
		return "return"
	case "numpadenter":
		return "enter"
	case "backspace":
		return "delete"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func legacyMacroRuntimeSnapshot() *legacyMacroRuntime {
	legacyMacrosMu.RLock()
	runtime := legacyMacrosRuntime
	legacyMacrosMu.RUnlock()
	return runtime
}

func legacyMacroFirstInputWord(text string) string {
	word, _, _ := legacyMacroFirstInputWordRange(text)
	return word
}

func legacyMacroFirstInputWordRange(text string) (word string, start, end int) {
	start = 0
	for start < len(text) && legacyMacroInputBreak(text[start]) {
		start++
	}
	end = start
	for end < len(text) && !legacyMacroInputBreak(text[end]) {
		end++
	}
	return text[start:end], start, end
}

func legacyMacroInputBreak(char byte) bool {
	switch char {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func legacyMacroTriggerMatches(declaration legacyMacroDeclaration, trigger string) bool {
	if declaration.Attributes&legacyMacroIgnoreCase != 0 {
		return strings.EqualFold(declaration.Trigger, trigger)
	}
	return declaration.Trigger == trigger
}

func legacyMacroKeyMatches(declaration legacyMacroDeclaration, name string, modifiers legacyMacroModifiers) bool {
	if declaration.Key.Modifiers != modifiers {
		return false
	}
	if strings.EqualFold(declaration.Key.Name, name) {
		return true
	}
	return (declaration.Key.Name == "delete" && name == "del") ||
		(declaration.Key.Name == "del" && name == "delete") ||
		(declaration.Key.Name == "clear" && name == "escape") ||
		(declaration.Key.Name == "escape" && name == "clear")
}

func (runtime *legacyMacroRuntime) startTriggeredLocked(index int, context legacyMacroExecutionContext, frame int64) (*legacyMacroExecution, error) {
	execution, err := runtime.startDeclarationWithContextLocked(index, context)
	if err != nil {
		return nil, err
	}
	runtime.advanceExecutionLocked(execution, frame)
	if execution.complete {
		runtime.removeExecutionLocked(execution)
	}
	return execution, nil
}

func (runtime *legacyMacroRuntime) removeExecutionLocked(execution *legacyMacroExecution) {
	for index, active := range runtime.active {
		if active != execution {
			continue
		}
		runtime.active = append(runtime.active[:index], runtime.active[index+1:]...)
		return
	}
}

func (runtime *legacyMacroRuntime) triggerExpression(text string, frame int64) bool {
	trigger := legacyMacroFirstInputWord(text)
	if trigger == "" {
		return false
	}

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for index, declaration := range runtime.program.Macros {
		if declaration.Kind != legacyMacroExpression || !legacyMacroTriggerMatches(declaration, trigger) {
			continue
		}
		macroText := ""
		if len(text) > len(declaration.Trigger) {
			macroText = text[len(declaration.Trigger):]
		}
		_, _ = runtime.startTriggeredLocked(index, legacyMacroExecutionContext{
			Text:          macroText,
			TextSelection: text,
		}, frame)
		return true
	}
	return false
}

func (runtime *legacyMacroRuntime) hasExpression(text string) bool {
	trigger := legacyMacroFirstInputWord(text)
	if trigger == "" {
		return false
	}

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for _, declaration := range runtime.program.Macros {
		if declaration.Kind == legacyMacroExpression && legacyMacroTriggerMatches(declaration, trigger) {
			return true
		}
	}
	return false
}

func (runtime *legacyMacroRuntime) triggerKey(name string, modifiers legacyMacroModifiers, frame int64) (started, allowDefault bool) {
	context := legacyMacroDefaultExecutionContext()
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for index, declaration := range runtime.program.Macros {
		if declaration.Kind != legacyMacroKey || !legacyMacroKeyMatches(declaration, name, modifiers) {
			continue
		}
		_, _ = runtime.startTriggeredLocked(index, context, frame)
		return true, declaration.Attributes&legacyMacroNoOverride != 0
	}
	return false, true
}

func (runtime *legacyMacroRuntime) triggerClick(event legacyMacroClickEvent, frame int64) (started, allowDefault bool) {
	context := legacyMacroDefaultExecutionContext()
	context.ClickName = event.Name
	context.HasClickName = event.HasName
	context.ClickButton = event.Button
	context.HasClickButton = event.HasButton
	context.ClickChord = event.Chord
	context.HasClickChord = event.HasChord

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for index, declaration := range runtime.program.Macros {
		if declaration.Kind != legacyMacroClick || declaration.Key.Modifiers != event.Modifiers || declaration.Key.Button != event.Button {
			continue
		}
		if !event.OnPlayer && declaration.Attributes&legacyMacroAnyClick == 0 {
			continue
		}
		_, _ = runtime.startTriggeredLocked(index, context, frame)
		return true, declaration.Attributes&legacyMacroNoOverride != 0
	}
	return false, true
}

func (runtime *legacyMacroRuntime) triggerWheel(name string, modifiers legacyMacroModifiers, frame int64) (started, allowDefault bool) {
	context := legacyMacroDefaultExecutionContext()
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for index, declaration := range runtime.program.Macros {
		if declaration.Kind != legacyMacroWheel || !legacyMacroKeyMatches(declaration, name, modifiers) {
			continue
		}
		_, _ = runtime.startTriggeredLocked(index, context, frame)
		return true, declaration.Attributes&legacyMacroNoOverride != 0
	}
	return false, true
}

func (runtime *legacyMacroRuntime) triggerReplacement(text string, cursor int) (string, int, bool) {
	runes := []rune(text)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	start := cursor
	for start > 0 && !legacyMacroReplacementBreak(runes[start-1]) {
		start--
	}
	if start == cursor {
		return text, cursor, false
	}
	word := string(runes[start:cursor])

	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for index, declaration := range runtime.program.Macros {
		if declaration.Kind != legacyMacroReplacement || !legacyMacroTriggerMatches(declaration, word) {
			continue
		}
		execution, err := runtime.startDeclarationWithOptionsLocked(index, legacyMacroExecutionContext{
			Text:          text,
			TextSelection: word,
		}, true)
		if err != nil {
			return text, cursor, false
		}
		for !execution.complete {
			runtime.advanceExecutionLocked(execution, 0)
			if execution.waitUntil > 0 && !execution.complete {
				runtime.failExecutionLocked(execution, execution.lastLine, "replacement macros may not pause")
			}
		}
		runtime.removeExecutionLocked(execution)
		replacement := []rune(execution.result)
		if len(replacement) == 0 && start > 0 {
			// The classic client deletes the separator before an empty
			// replacement so typing the triggering separator does not leave two.
			start--
		}
		updated := make([]rune, 0, len(runes)-cursor+start+len(replacement))
		updated = append(updated, runes[:start]...)
		updated = append(updated, replacement...)
		updated = append(updated, runes[cursor:]...)
		return string(updated), start + len(replacement), true
	}
	return text, cursor, false
}

func legacyMacroReplacementBreak(char rune) bool {
	return char == ' ' || char == '\t' || char == '\r' || char == '\n'
}

func legacyMacroTriggerExpression(text string, frame int64) bool {
	runtime := legacyMacroRuntimeSnapshot()
	return runtime != nil && runtime.triggerExpression(text, frame)
}

func legacyMacroHasExpression(text string) bool {
	runtime := legacyMacroRuntimeSnapshot()
	return runtime != nil && runtime.hasExpression(text)
}

func legacyMacroTriggerReplacement(text string, cursor int) (string, int, bool) {
	runtime := legacyMacroRuntimeSnapshot()
	if runtime == nil {
		return text, cursor, false
	}
	return runtime.triggerReplacement(text, cursor)
}

func legacyMacroTriggerClick(event legacyMacroClickEvent, frame int64) (started, allowDefault bool) {
	runtime := legacyMacroRuntimeSnapshot()
	if runtime == nil {
		return false, true
	}
	return runtime.triggerClick(event, frame)
}

func legacyMacroTriggerWheel(name string, modifiers legacyMacroModifiers, frame int64) (started, allowDefault bool) {
	runtime := legacyMacroRuntimeSnapshot()
	if runtime == nil {
		return false, true
	}
	return runtime.triggerWheel(name, modifiers, frame)
}

func legacyMacroWorldClickEvent(info ClickInfo, button, chord int) legacyMacroClickEvent {
	event := legacyMacroClickEvent{
		Button:    button,
		Chord:     chord,
		HasName:   true,
		HasButton: true,
		HasChord:  true,
		Modifiers: legacyMacroCurrentModifiers(false),
	}
	if info.OnPlayer {
		event.Name = info.Mobile.Name
		event.OnPlayer = true
	}
	return event
}

func legacyMacroPlayerClickEvent(name string) legacyMacroClickEvent {
	return legacyMacroClickEvent{
		Name:      name,
		HasName:   true,
		OnPlayer:  true,
		Button:    1,
		Modifiers: legacyMacroCurrentModifiers(false),
	}
}

func legacyMacroPollKeyboard(frame int64, typingElsewhere bool) {
	if typingElsewhere || !ebiten.IsFocused() {
		return
	}
	runtime := legacyMacroRuntimeSnapshot()
	if runtime == nil {
		return
	}
	for _, key := range inpututil.AppendJustPressedKeys(nil) {
		name, numpad, ok := legacyMacroKeyName(key)
		if !ok {
			continue
		}
		started, allowDefault := runtime.triggerKey(name, legacyMacroCurrentModifiers(numpad), frame)
		if started && !allowDefault {
			legacyMacroMarkKeyConsumed(key, name)
		}
	}
}

func legacyMacroCurrentModifiers(numpad bool) legacyMacroModifiers {
	var modifiers legacyMacroModifiers
	if ebiten.IsKeyPressed(ebiten.KeyMeta) || ebiten.IsKeyPressed(ebiten.KeyMetaLeft) || ebiten.IsKeyPressed(ebiten.KeyMetaRight) {
		modifiers |= legacyMacroModCommand
	}
	if ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight) {
		modifiers |= legacyMacroModControl
	}
	if numpad {
		modifiers |= legacyMacroModNumpad
	}
	if ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight) {
		modifiers |= legacyMacroModOption
	}
	if ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
		modifiers |= legacyMacroModShift
	}
	return modifiers
}

func legacyMacroKeyName(key ebiten.Key) (name string, numpad, ok bool) {
	if key >= ebiten.KeyA && key <= ebiten.KeyZ {
		return strings.ToLower(key.String()), false, true
	}
	if key >= ebiten.KeyDigit0 && key <= ebiten.KeyDigit9 {
		return string(rune(int(key-ebiten.KeyDigit0) + '0')), false, true
	}
	if key >= ebiten.KeyNumpad0 && key <= ebiten.KeyNumpad9 {
		return string(rune(int(key-ebiten.KeyNumpad0) + '0')), true, true
	}
	if key >= ebiten.KeyF1 && key <= ebiten.KeyF16 {
		return strings.ToLower(key.String()), false, true
	}

	switch key {
	case ebiten.KeyEscape:
		return "escape", false, true
	case ebiten.KeyMinus, ebiten.KeyNumpadSubtract:
		return "minus", key == ebiten.KeyNumpadSubtract, true
	case ebiten.KeyBackspace:
		return "delete", false, true
	case ebiten.KeyDelete:
		return "del", false, true
	case ebiten.KeyTab:
		return "tab", false, true
	case ebiten.KeyEnter:
		return "return", false, true
	case ebiten.KeyNumpadEnter:
		return "enter", true, true
	case ebiten.KeySpace:
		return "space", false, true
	case ebiten.KeyHome:
		return "home", false, true
	case ebiten.KeyPageUp:
		return "pageup", false, true
	case ebiten.KeyEnd:
		return "end", false, true
	case ebiten.KeyPageDown:
		return "pagedown", false, true
	case ebiten.KeyArrowUp:
		return "up", false, true
	case ebiten.KeyArrowDown:
		return "down", false, true
	case ebiten.KeyArrowLeft:
		return "left", false, true
	case ebiten.KeyArrowRight:
		return "right", false, true
	case ebiten.KeyComma:
		return ",", false, true
	case ebiten.KeyPeriod, ebiten.KeyNumpadDecimal:
		return ".", key == ebiten.KeyNumpadDecimal, true
	case ebiten.KeySlash, ebiten.KeyNumpadDivide:
		return "/", key == ebiten.KeyNumpadDivide, true
	case ebiten.KeySemicolon:
		return ";", false, true
	case ebiten.KeyQuote:
		return "'", false, true
	case ebiten.KeyBackquote:
		return "`", false, true
	case ebiten.KeyBackslash, ebiten.KeyIntlBackslash:
		return "\\", false, true
	case ebiten.KeyBracketLeft:
		return "[", false, true
	case ebiten.KeyBracketRight:
		return "]", false, true
	case ebiten.KeyEqual, ebiten.KeyNumpadEqual:
		return "=", key == ebiten.KeyNumpadEqual, true
	case ebiten.KeyNumpadAdd:
		return "+", true, true
	case ebiten.KeyNumpadMultiply:
		return "*", true, true
	}
	return "", false, false
}

func legacyMacroKeyProducesText(key ebiten.Key) bool {
	name, _, ok := legacyMacroKeyName(key)
	return ok && (utf8.RuneCountInString(name) == 1 || name == "space" || name == "minus")
}

// legacyMacroMovePlayer translates the reference client's directional move
// command into one queued game-input sample. Repeating movement macros issue
// another sample after each pause, just as the original client does.
func legacyMacroMovePlayer(move legacyMacroMove) {
	if move.Direction == legacyMacroMoveStop {
		inputMu.Lock()
		previous := latestInput
		inputMu.Unlock()
		queueInput(inputState{mouseX: previous.mouseX, mouseY: previous.mouseY})
		return
	}

	dx, dy := 0, 0
	switch move.Direction {
	case legacyMacroMoveEast:
		dx = 1
	case legacyMacroMoveNorthEast:
		dx, dy = 1, -1
	case legacyMacroMoveNorth:
		dy = -1
	case legacyMacroMoveNorthWest:
		dx, dy = -1, -1
	case legacyMacroMoveWest:
		dx = -1
	case legacyMacroMoveSouthWest:
		dx, dy = -1, 1
	case legacyMacroMoveSouth:
		dy = 1
	case legacyMacroMoveSouthEast:
		dx, dy = 1, 1
	default:
		return
	}

	speed := gs.KBWalkSpeed
	if move.Run {
		speed = 1
	}
	queueInput(inputState{
		mouseX:    int16(float64(dx) * float64(fieldCenterX) * speed),
		mouseY:    int16(float64(dy) * float64(fieldCenterY) * speed),
		mouseDown: true,
	})
}

func legacyMacroReplacementBoundary(char rune) bool {
	return unicode.IsPrint(char) && !unicode.IsLetter(char) && !unicode.IsDigit(char)
}

func legacyMacroInsertInputText(text string) {
	if text == "" {
		return
	}
	inputMu.Lock()
	if inputPos < 0 {
		inputPos = 0
	}
	if inputPos > len(inputText) {
		inputPos = len(inputText)
	}
	updated := make([]rune, 0, len(inputText)+utf8.RuneCountInString(text))
	updated = append(updated, inputText[:inputPos]...)
	updated = append(updated, []rune(text)...)
	updated = append(updated, inputText[inputPos:]...)
	inputText = updated
	inputPos += utf8.RuneCountInString(text)
	inputActive = true
	inputMu.Unlock()
	legacyMacroRefreshInput()
}

func legacyMacroSetInputText(text string) {
	inputMu.Lock()
	inputText = []rune(text)
	inputPos = len(inputText)
	inputActive = true
	inputMu.Unlock()
	legacyMacroRefreshInput()
}

func legacyMacroRefreshInput() {
	spellDirty = true
	updateConsoleWindow()
	if consoleWin != nil {
		consoleWin.Refresh()
	}
}
