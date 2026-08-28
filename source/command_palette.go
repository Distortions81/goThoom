package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type commandPaletteAction struct {
	label  string
	detail string
	search string
	run    func()
}

var (
	commandPaletteWin      *eui.WindowData
	commandPaletteInput    *eui.ItemData
	commandPaletteList     *eui.ItemData
	commandPaletteRunBtn   *eui.ItemData
	commandPaletteActions  []commandPaletteAction
	commandPaletteMatches  []commandPaletteAction
	commandPaletteButtons  []*eui.ItemData
	commandPaletteSelected int
)

const commandPaletteMaxResults = 30

var commandSettingSpecials = []settingsSchemaEntry{
	{field: "SpriteUpscaleMode", category: settingsRendering, name: "artwork_upscale_style"},
	{field: "BarPlacement", category: settingsInterface, name: "status_bar_placement"},
}

func commandSettingEntries() []settingsSchemaEntry {
	entries := make([]settingsSchemaEntry, 0, len(settingsSchema)+len(commandSettingSpecials))
	entries = append(entries, settingsSchema...)
	entries = append(entries, commandSettingSpecials...)
	return entries
}

func commandPaletteShortcutPressed() bool {
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	return ctrl && shift && inpututil.IsKeyJustPressed(ebiten.KeyP)
}

func toggleCommandPalette() {
	if commandPaletteWin != nil && commandPaletteWin.IsOpen() {
		closeCommandPalette()
		return
	}
	openCommandPalette()
}

func openCommandPalette() {
	makeCommandPaletteWindow()
	commandPaletteActions = buildCommandPaletteActions()
	commandPaletteInput.Text = ""
	if commandPaletteInput.TextPtr != nil {
		*commandPaletteInput.TextPtr = ""
	}
	commandPaletteInput.CursorPos = 0
	refreshCommandPalette("")
	commandPaletteWin.MarkOpen()
	eui.Focus(commandPaletteInput)
}

func closeCommandPalette() {
	if commandPaletteInput != nil {
		eui.ClearFocus(commandPaletteInput)
	}
	if commandPaletteWin != nil && commandPaletteWin.IsOpen() {
		commandPaletteWin.Close()
	}
}

func makeCommandPaletteWindow() {
	if commandPaletteWin != nil {
		return
	}
	commandPaletteWin = eui.NewWindow()
	commandPaletteWin.Title = "Command Palette"
	commandPaletteWin.Closable = true
	commandPaletteWin.Resizable = true
	commandPaletteWin.Movable = true
	commandPaletteWin.NoScroll = true
	commandPaletteWin.Size = eui.Point{X: 680, Y: 470}
	commandPaletteWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	commandPaletteInput, _ = eui.NewInput()
	commandPaletteInput.Label = "Search settings, windows, scripts, actions, and commands"
	commandPaletteInput.Size = eui.Point{X: 650, Y: 30}
	commandPaletteInput.Handler.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			refreshCommandPalette(ev.Text)
		}
	}
	root.AddItem(commandPaletteInput)

	commandPaletteList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	commandPaletteList.Size = eui.Point{X: 650, Y: 350}
	root.AddItem(commandPaletteList)

	buttons := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	commandPaletteRunBtn, _ = eui.NewButton()
	commandPaletteRunBtn.Text = "Run"
	commandPaletteRunBtn.Size = eui.Point{X: 90, Y: 28}
	commandPaletteRunBtn.Handler.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			runFirstCommandPaletteMatch()
		}
	}
	buttons.AddItem(commandPaletteRunBtn)

	closeBtn, closeEvents := eui.NewButton()
	closeBtn.Text = "Close"
	closeBtn.Size = eui.Point{X: 90, Y: 28}
	closeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			closeCommandPalette()
		}
	}
	buttons.AddItem(closeBtn)
	root.AddItem(buttons)

	commandPaletteWin.DefaultButton = commandPaletteRunBtn
	commandPaletteWin.OnResize = layoutCommandPalette
	commandPaletteWin.OnClose = func() { eui.ClearFocus(commandPaletteInput) }
	commandPaletteWin.AddItem(root)
	commandPaletteWin.AddWindow(false)
	layoutCommandPalette()
}

func layoutCommandPalette() {
	if commandPaletteWin == nil || commandPaletteInput == nil || commandPaletteList == nil {
		return
	}
	size := commandPaletteWin.GetRawSize()
	width := size.X - 30
	if width < 280 {
		width = 280
	}
	height := size.Y - 120
	if height < 120 {
		height = 120
	}
	commandPaletteInput.Size.X = width
	commandPaletteList.Size = eui.Point{X: width, Y: height}
	for _, item := range commandPaletteList.Contents {
		item.Size.X = width
	}
	commandPaletteWin.Refresh()
}

func refreshCommandPalette(query string) {
	if commandPaletteList == nil {
		return
	}
	commandPaletteMatches = filterCommandPaletteActions(commandPaletteActions, query)
	commandPaletteSelected = 0
	commandPaletteButtons = commandPaletteButtons[:0]
	commandPaletteList.Contents = commandPaletteList.Contents[:0]
	width := commandPaletteList.Size.X
	for i, action := range commandPaletteMatches {
		if i >= commandPaletteMaxResults {
			break
		}
		action := action
		button, events := eui.NewButton()
		button.Text = commandPaletteResultText(action, i == commandPaletteSelected)
		button.Size = eui.Point{X: width, Y: 30}
		button.SetTooltip(action.detail)
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				runCommandPaletteAction(action)
			}
		}
		commandPaletteList.AddItem(button)
		commandPaletteButtons = append(commandPaletteButtons, button)
	}
	commandPaletteList.Scroll = eui.Point{}
	commandPaletteRunBtn.Disabled = len(commandPaletteMatches) == 0
	commandPaletteWin.Refresh()
}

func commandPaletteResultText(action commandPaletteAction, selected bool) string {
	prefix := "   "
	if selected {
		prefix = ">  "
	}
	text := prefix + action.label
	if action.detail != "" {
		text += "  —  " + action.detail
	}
	return text
}

func moveCommandPaletteSelection(delta int) {
	count := len(commandPaletteButtons)
	if count == 0 {
		return
	}
	commandPaletteSelected += delta
	if commandPaletteSelected < 0 {
		commandPaletteSelected = count - 1
	} else if commandPaletteSelected >= count {
		commandPaletteSelected = 0
	}
	for i, button := range commandPaletteButtons {
		button.UpdateText(commandPaletteResultText(commandPaletteMatches[i], i == commandPaletteSelected))
	}
	const rowHeight float32 = 30
	top := float32(commandPaletteSelected) * rowHeight
	bottom := top + rowHeight
	if top < commandPaletteList.Scroll.Y {
		commandPaletteList.Scroll.Y = top
	} else if bottom > commandPaletteList.Scroll.Y+commandPaletteList.Size.Y {
		commandPaletteList.Scroll.Y = bottom - commandPaletteList.Size.Y
	}
	commandPaletteWin.Refresh()
}

func filterCommandPaletteActions(actions []commandPaletteAction, query string) []commandPaletteAction {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return append([]commandPaletteAction(nil), actions...)
	}
	type scoredAction struct {
		action commandPaletteAction
		score  int
	}
	scored := make([]scoredAction, 0, len(actions))
	for _, action := range actions {
		haystack := strings.ToLower(action.label + " " + action.detail + " " + action.search)
		matched := true
		for _, term := range terms {
			if !strings.Contains(haystack, term) {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		label := strings.ToLower(action.label)
		score := 2
		if label == strings.Join(terms, " ") {
			score = 0
		} else if strings.HasPrefix(label, strings.Join(terms, " ")) {
			score = 1
		}
		scored = append(scored, scoredAction{action: action, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score < scored[j].score
		}
		return strings.ToLower(scored[i].action.label) < strings.ToLower(scored[j].action.label)
	})
	result := make([]commandPaletteAction, len(scored))
	for i := range scored {
		result[i] = scored[i].action
	}
	return result
}

func runFirstCommandPaletteMatch() {
	if commandPaletteSelected >= 0 && commandPaletteSelected < len(commandPaletteMatches) {
		runCommandPaletteAction(commandPaletteMatches[commandPaletteSelected])
	}
}

func runCommandPaletteAction(action commandPaletteAction) {
	closeCommandPalette()
	if action.run != nil {
		action.run()
	}
}

func openPaletteWindow(win *eui.WindowData) {
	if win != nil {
		win.MarkOpen()
	}
}

func prefillPaletteCommand(text string) {
	scriptSetInputText(text)
	updateConsoleWindow()
}

func buildCommandPaletteActions() []commandPaletteAction {
	actions := []commandPaletteAction{
		{label: "Command: /setting", detail: "Prefill settings command", search: "search get set reset", run: func() { prefillPaletteCommand("/setting ") }},
		{label: "Window: Settings", detail: "Open settings", run: func() { openPaletteWindow(settingsWin) }},
		{label: "Window: Quality Options", detail: "Open graphics quality settings", run: func() { openPaletteWindow(qualityWin) }},
		{label: "Window: Audio Mixer", detail: "Open audio controls", run: func() { openPaletteWindow(mixerWin) }},
		{label: "Window: Notifications", detail: "Open notification settings", run: func() { openPaletteWindow(notificationsWin) }},
		{label: "Window: Advanced Settings", detail: "Open advanced settings", run: func() { openPaletteWindow(advancedWin) }},
		{label: "Window: Speech Bubbles", detail: "Open speech bubble settings", run: func() { openPaletteWindow(bubbleWin) }},
		{label: "Window: Scripts", detail: "Open script manager", run: func() { refreshscriptsWindow(); openPaletteWindow(scriptsWin) }},
		{label: "Window: Legacy Macros", detail: "Open macro manager", run: func() {
			makeLegacyMacroLibraryWindow()
			refreshLegacyMacroLibraryWindow()
			openPaletteWindow(legacyMacroLibraryWin)
		}},
		{label: "Window: Hotkeys", detail: "Open hotkey editor", run: func() { openPaletteWindow(hotkeysWin) }},
		{label: "Window: Shortcuts", detail: "Open shortcut editor", run: func() { refreshShortcutsList(); openPaletteWindow(shortcutsWin) }},
		{label: "Window: Keybindings", detail: "Open movement keybindings", run: func() { refreshKeybindingsList(); openPaletteWindow(keybindingsWin) }},
		{label: "Window: Saved Data", detail: "Open script saved data", run: func() { makeSavedDataWindow(); openPaletteWindow(savedDataWin) }},
		{label: "Window: Windows", detail: "Manage window visibility", run: func() { openPaletteWindow(windowsWin) }},
		{label: "Window: Inventory", detail: "Open inventory", run: func() { openPaletteWindow(inventoryWin) }},
		{label: "Window: Players", detail: "Open player list", run: func() { openPaletteWindow(playersWin) }},
		{label: "Window: Chat", detail: "Open chat", run: func() { openPaletteWindow(chatWin) }},
		{label: "Window: Console", detail: "Open console", run: func() { openPaletteWindow(consoleWin) }},
		{label: "Window: Help", detail: "Open help", run: func() { openHelpWindow(nil) }},
		{label: "Window: Debug", detail: "Open debugging information", run: func() { openPaletteWindow(debugWin) }},
		{label: "Action: Exit Session", detail: "Disconnect or return to login", search: "exit disconnect movie", run: confirmExitSession},
		{label: "Action: Quit goThoom", detail: "Close the application", search: "exit application", run: confirmQuit},
		{label: "Scripts: Rescan and Reload", detail: "Rescan script folders and reload enabled scripts", run: rescanscripts},
	}

	if selectedPlayerName != "" {
		name := selectedPlayerName
		for _, item := range []struct {
			label   string
			command string
		}{
			{label: "Thank", command: "/thank "},
			{label: "Curse", command: "/curse "},
			{label: "Share", command: "/share "},
			{label: "Unshare", command: "/unshare "},
			{label: "Info", command: "/info "},
			{label: "Pull", command: "/pull "},
			{label: "Push", command: "/push "},
		} {
			item := item
			actions = append(actions, commandPaletteAction{
				label:  "Player: " + item.label + " " + name,
				detail: "Run for selected player",
				search: "selected player action",
				run: func() {
					enqueueCommand(item.command + maybeQuoteName(name))
					nextCommand()
				},
			})
		}
	}

	SettingsLock.Lock()
	settingsValue := reflect.ValueOf(gs)
	for _, entry := range commandSettingEntries() {
		entry := entry
		field := settingsValue.FieldByName(entry.field)
		if !field.IsValid() {
			continue
		}
		fullName := settingFullName(entry)
		value := formatCommandSettingValue(entry, field)
		label := "Setting: " + humanizeSettingName(entry.name)
		detail := fullName + " = " + value
		action := commandPaletteAction{label: label, detail: detail, search: entry.category + " " + entry.field}
		if field.Kind() == reflect.Bool {
			action.search += " toggle enable disable on off"
			action.run = func() { executeSettingCommand("set " + fullName + " toggle") }
		} else {
			action.run = func() { prefillPaletteCommand("/setting set " + fullName + " ") }
		}
		actions = append(actions, action)
	}
	SettingsLock.Unlock()

	scriptMu.RLock()
	commands := make([]string, 0, len(scriptCommands))
	for name := range scriptCommands {
		commands = append(commands, name)
	}
	scripts := make([]string, 0, len(scriptDisplayNames))
	for _, name := range scriptDisplayNames {
		scripts = append(scripts, name)
	}
	scriptMu.RUnlock()
	sort.Strings(commands)
	sort.Strings(scripts)
	for _, name := range commands {
		name := name
		actions = append(actions, commandPaletteAction{
			label: "Command: /" + name, detail: "Prefill command input", search: "script slash command",
			run: func() { prefillPaletteCommand("/" + name + " ") },
		})
	}
	for _, name := range scripts {
		actions = append(actions, commandPaletteAction{
			label: "Script: " + name, detail: "Show in the Scripts window", search: "go script enabled disabled",
			run: func() { refreshscriptsWindow(); openPaletteWindow(scriptsWin) },
		})
	}
	return actions
}

func humanizeSettingName(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' })
	for i, word := range words {
		runes := []rune(word)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func settingFullName(entry settingsSchemaEntry) string {
	return entry.category + "." + entry.name
}

func normalizeSettingLookup(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, "/", ".")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func findSettingEntry(name string) (settingsSchemaEntry, error) {
	want := normalizeSettingLookup(name)
	var matches []settingsSchemaEntry
	for _, entry := range commandSettingEntries() {
		full := normalizeSettingLookup(settingFullName(entry))
		field := normalizeSettingLookup(entry.field)
		short := normalizeSettingLookup(entry.name)
		if want == full || want == field || want == short {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return settingsSchemaEntry{}, fmt.Errorf("setting %q is ambiguous; use category.name", name)
	}
	return settingsSchemaEntry{}, fmt.Errorf("unknown setting %q", name)
}

func formatSettingValue(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(raw)
}

func formatCommandSettingValue(entry settingsSchemaEntry, field reflect.Value) string {
	switch entry.field {
	case "SpriteUpscaleMode":
		return strconv.Quote(artworkUpscaleModeName(int(field.Int())))
	case "BarPlacement":
		return strconv.Quote(barPlacementName(BarPlacement(field.Int())))
	default:
		return formatSettingValue(field.Interface())
	}
}

func trimSettingString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `"`) {
		var value string
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return "", err
		}
		return strings.ToLower(value), nil
	}
	return strings.ToLower(raw), nil
}

func parseCommandSettingValue(entry settingsSchemaEntry, field reflect.Value, raw string) (reflect.Value, error) {
	value := reflect.New(field.Type()).Elem()
	switch entry.field {
	case "SpriteUpscaleMode":
		name, err := trimSettingString(raw)
		if err != nil {
			return reflect.Value{}, err
		}
		valid := map[string]int{
			"off": artworkUpscaleOff, "crisp": artworkUpscaleCrisp, "balanced": artworkUpscaleBalanced,
			"smooth": artworkUpscaleSmooth, "ultra_smooth": artworkUpscaleUltraSmooth,
		}
		mode, ok := valid[name]
		if !ok {
			return reflect.Value{}, fmt.Errorf("use off, crisp, balanced, smooth, or ultra_smooth")
		}
		value.SetInt(int64(mode))
		return value, nil
	case "BarPlacement":
		name, err := trimSettingString(raw)
		if err != nil {
			return reflect.Value{}, err
		}
		valid := map[string]BarPlacement{
			"bottom": BarPlacementBottom, "lower_left": BarPlacementLowerLeft,
			"lower_right": BarPlacementLowerRight, "upper_right": BarPlacementUpperRight,
		}
		placement, ok := valid[name]
		if !ok {
			return reflect.Value{}, fmt.Errorf("use bottom, lower_left, lower_right, or upper_right")
		}
		value.SetInt(int64(placement))
		return value, nil
	default:
		return parseSettingValue(field, raw)
	}
}

func parseSettingValue(field reflect.Value, raw string) (reflect.Value, error) {
	value := reflect.New(field.Type()).Elem()
	raw = strings.TrimSpace(raw)
	if field.Kind() == reflect.Bool {
		switch strings.ToLower(raw) {
		case "toggle":
			value.SetBool(!field.Bool())
			return value, nil
		case "on", "yes", "enabled", "enable":
			raw = "true"
		case "off", "no", "disabled", "disable":
			raw = "false"
		}
	}
	if field.Kind() == reflect.String && !strings.HasPrefix(raw, `"`) {
		value.SetString(raw)
		return value, nil
	}
	if raw == "" {
		return reflect.Value{}, fmt.Errorf("missing value")
	}
	if err := json.Unmarshal([]byte(raw), value.Addr().Interface()); err != nil {
		return reflect.Value{}, err
	}
	return value, nil
}

func applySettingCommandSideEffects(entry settingsSchemaEntry, value reflect.Value) error {
	if entry.field == "Theme" {
		if err := eui.LoadTheme(value.String()); err != nil {
			return err
		}
	}
	if entry.field == "Style" {
		if err := eui.LoadStyle(value.String()); err != nil {
			return err
		}
	}
	return nil
}

func applySettingRuntimeEffects(entry settingsSchemaEntry) {
	switch entry.field {
	case "ServerAddress":
		applyServerAddressSetting()
	case "HighQualityResampling":
		setHighQualityResamplingEnabled(gs.HighQualityResampling)
	case "ClickToToggle":
		if !gs.ClickToToggle {
			walkToggled = false
		}
	case "Theme":
		updateInventoryWindow()
		updatePlayersWindow()
		updateDimmedScreenBG()
	case "ToolbarPlacement":
		placeToolbar(gs.ToolbarPlacement, true)
	case "SpriteUpscaleMode":
		setArtworkUpscaleMode(gs.SpriteUpscaleMode)
	}
	if strings.HasPrefix(entry.field, "Tiled") || entry.field == "MessagesToConsole" {
		applyTiledWorkspaceLayout()
	}
}

func setSettingFromText(entry settingsSchemaEntry, raw string) (string, error) {
	SettingsLock.Lock()
	settingsValue := reflect.ValueOf(&gs).Elem()
	field := settingsValue.FieldByName(entry.field)
	if !field.IsValid() || !field.CanSet() {
		SettingsLock.Unlock()
		return "", fmt.Errorf("setting %s is unavailable", settingFullName(entry))
	}
	value, err := parseCommandSettingValue(entry, field, raw)
	if err != nil {
		SettingsLock.Unlock()
		return "", fmt.Errorf("invalid value for %s: %w", settingFullName(entry), err)
	}
	if err := applySettingCommandSideEffects(entry, value); err != nil {
		SettingsLock.Unlock()
		return "", fmt.Errorf("could not apply %s: %w", settingFullName(entry), err)
	}
	field.Set(value)
	if entry.field == "Theme" {
		gs.Style = eui.CurrentStyleName()
	}
	if entry.field == "UIScale" {
		gs.UIScale = clampUIScalePreference(gs.UIScale)
		eui.SetUserUIScale(float32(gs.UIScale))
		updateGameWindowSize()
	}
	applySettings()
	applySettingRuntimeEffects(entry)
	settingsDirty = true
	formatted := formatCommandSettingValue(entry, field)
	SettingsLock.Unlock()
	if uiReady {
		rebuildConfigurationWindows()
	}
	return formatted, nil
}

// Settings controls store their displayed values in widgets, so refresh alone
// cannot reflect a change made through a slash command. Rebuild the small
// configuration windows and preserve which ones were open.
func rebuildConfigurationWindows() {
	rebuild := func(win **eui.WindowData, makeWindow func()) {
		if *win == nil {
			return
		}
		wasOpen := (*win).IsOpen()
		(*win).RemoveWindow()
		*win = nil
		makeWindow()
		if wasOpen && *win != nil {
			(*win).MarkOpen()
		}
	}
	rebuild(&settingsWin, makeSettingsWindow)
	rebuild(&qualityWin, makeQualityWindow)
	rebuild(&advancedWin, makeAdvancedSettingsWindow)
	rebuild(&notificationsWin, makeNotificationsWindow)
	rebuild(&bubbleWin, makeBubbleWindow)
	rebuild(&mixerWin, makeMixerWindow)
	rebuild(&joystickWin, makeJoystickWindow)
	rebuild(&tileLayoutWin, makeTileLayoutWindow)
	if legacyMacroLibraryWin != nil {
		wasOpen := legacyMacroLibraryWin.IsOpen()
		legacyMacroLibraryWin.RemoveWindow()
		legacyMacroLibraryWin = nil
		makeLegacyMacroLibraryWindow()
		if wasOpen {
			legacyMacroLibraryWin.MarkOpen()
		}
	}
}

func resetSetting(entry settingsSchemaEntry) (string, error) {
	defaultField := reflect.ValueOf(gsdef).FieldByName(entry.field)
	if !defaultField.IsValid() {
		return "", fmt.Errorf("setting %s has no default", settingFullName(entry))
	}
	return setSettingFromText(entry, formatCommandSettingValue(entry, defaultField))
}

func settingSearchResults(query string) []settingsSchemaEntry {
	terms := strings.Fields(strings.ToLower(query))
	results := make([]settingsSchemaEntry, 0)
	for _, entry := range commandSettingEntries() {
		haystack := strings.ToLower(settingFullName(entry) + " " + entry.field + " " + humanizeSettingName(entry.name))
		matches := true
		for _, term := range terms {
			if !strings.Contains(haystack, term) {
				matches = false
				break
			}
		}
		if matches {
			results = append(results, entry)
		}
	}
	return results
}

func executeSettingCommand(args string) {
	verb, rest, _ := strings.Cut(strings.TrimSpace(args), " ")
	verb = strings.ToLower(verb)
	rest = strings.TrimSpace(rest)
	if verb == "" || verb == "help" {
		consoleMessage("Usage: /setting search <text> | get <name> | set <name> <value> | reset <name>")
		return
	}
	switch verb {
	case "search":
		results := settingSearchResults(rest)
		if len(results) == 0 {
			consoleMessage("No settings match " + strconv.Quote(rest))
			return
		}
		limit := len(results)
		if limit > commandPaletteMaxResults {
			limit = commandPaletteMaxResults
		}
		SettingsLock.Lock()
		settingsValue := reflect.ValueOf(gs)
		for _, entry := range results[:limit] {
			field := settingsValue.FieldByName(entry.field)
			consoleMessage(settingFullName(entry) + " = " + formatCommandSettingValue(entry, field))
		}
		SettingsLock.Unlock()
		if len(results) > limit {
			consoleMessage(fmt.Sprintf("%d more settings match; narrow the search", len(results)-limit))
		}
	case "get":
		entry, err := findSettingEntry(rest)
		if err != nil {
			consoleMessage("Setting error: " + err.Error())
			return
		}
		SettingsLock.Lock()
		field := reflect.ValueOf(gs).FieldByName(entry.field)
		value := formatCommandSettingValue(entry, field)
		SettingsLock.Unlock()
		consoleMessage(settingFullName(entry) + " = " + value)
	case "set":
		name, raw, found := strings.Cut(rest, " ")
		if !found || strings.TrimSpace(raw) == "" {
			consoleMessage("Usage: /setting set <name> <value>")
			return
		}
		entry, err := findSettingEntry(name)
		if err != nil {
			consoleMessage("Setting error: " + err.Error())
			return
		}
		value, err := setSettingFromText(entry, raw)
		if err != nil {
			consoleMessage("Setting error: " + err.Error())
			return
		}
		consoleMessage(settingFullName(entry) + " = " + value)
	case "reset":
		entry, err := findSettingEntry(rest)
		if err != nil {
			consoleMessage("Setting error: " + err.Error())
			return
		}
		value, err := resetSetting(entry)
		if err != nil {
			consoleMessage("Setting error: " + err.Error())
			return
		}
		consoleMessage(settingFullName(entry) + " reset to " + value)
	default:
		consoleMessage("Unknown /setting action. Use search, get, set, or reset.")
	}
}
