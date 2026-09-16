package main

import (
	"path/filepath"
	"sort"
	"strings"

	"gothoom/eui"
)

type textCommandGroup struct {
	typeName string
	source   string
	commands []string
}

type inputBarHelpEntry struct {
	shortcut    string
	description string
}

var (
	activeCommandPicker   *commandPicker
	messageCommandButtons [2]struct{ flow, button *eui.ItemData }
)

// These commands are implemented by the client rather than sent to the
// server. Internal test and movement commands are deliberately not listed.
var goThoomTextCommands = []string{"palette", "play", "setting", "tab"}

var goThoomCommandDescriptions = map[string]string{
	"block":   "Toggle blocking a player.",
	"forget":  "Remove a player's labels.",
	"ignore":  "Toggle ignoring a player.",
	"notts":   "Manage chat text-to-speech exclusions.",
	"palette": "Open the command palette.",
	"play":    "Play a bard tune with an instrument and Clan Lord note syntax.",
	"setting": "Search, inspect, or change goThoom settings.",
	"tab":     "Switch to an open session tab by number or direction.",
}

var serverCommandDescriptions = map[string]string{
	"action":       "Show an action bubble.",
	"affiliations": "Show your or another player's group affiliations.",
	"anoncurse":    "Give anonymous bad karma.",
	"anonthank":    "Give anonymous good karma.",
	"bag":          "Ask the server about your bag.",
	"boot":         "Send an intrusive character home or out of your house.",
	"bug":          "Report a problem or suggestion.",
	"buy":          "Accept a seller's offer.",
	"curse":        "Give bad karma, optionally with a reason.",
	"depart":       "Depart for Purgatory when fallen.",
	"drop":         "Drop a selected item, named item, or coins.",
	"equip":        "Ready an item for use.",
	"examine":      "Examine the selected inventory item.",
	"give":         "Give coins to another player.",
	"help":         "Show server help, optionally for one command.",
	"info":         "Show information about yourself or a player.",
	"karma":        "Show your or another player's karma.",
	"money":        "Show the coins you are carrying.",
	"name":         "Set or clear the selected item's custom name.",
	"narrate":      "Show a narration bubble.",
	"news":         "Show the latest news.",
	"options":      "List or change server preferences.",
	"ponder":       "Share a thought with nearby players.",
	"pose":         "Strike a named pose, optionally facing a direction.",
	"pray":         "Send a message to game masters.",
	"pull":         "Exchange places with a nearby player or pull a critter.",
	"push":         "Push a nearby player ahead of you.",
	"report":       "Report offensive player behavior.",
	"sell":         "Offer the selected item for sale.",
	"share":        "Manage experience sharing with other players.",
	"show":         "Show the selected item to nearby players.",
	"sky":          "Gaze skyward.",
	"sleep":        "Indicate that you are away from the keyboard.",
	"speak":        "Set the language you speak.",
	"status":       "Publish or remove Clan Lord status information.",
	"thank":        "Give good karma, optionally with a reason.",
	"think":        "Send a sunstone thought.",
	"thinkclan":    "Send a thought to your clan.",
	"thinkgroup":   "Send a thought to your group.",
	"thinkto":      "Send a sunstone thought to one player.",
	"tip":          "Show a gameplay tip.",
	"unequip":      "Store a ready item in your backpack.",
	"unshare":      "Stop experience sharing with a player or everyone.",
	"use":          "Use your equipped item or healing ability.",
	"useitem":      "Use an equipped item with text or an item target.",
	"whisper":      "Speak so only nearby players hear you.",
	"who":          "List players currently in the game.",
	"whoclan":      "List online members of a clan.",
	"yell":         "Speak over a long distance.",
}

// Argument signatures come from the client's captured /help responses. Angle
// brackets are required values; square brackets are optional forms.
var textCommandArgumentSyntax = map[string]string{
	"action":       "<text>",
	"affiliations": "[name | /quit <organization>]",
	"anoncurse":    "<person>",
	"anonthank":    "<person>",
	"bag":          "[options]",
	"block":        "<player>",
	"boot":         "<player>",
	"bug":          "<message>",
	"buy":          "<price> <seller>",
	"curse":        "<person> [reason]",
	"drop":         "[item | amount | /mine]",
	"equip":        "<item> [number]",
	"forget":       "<player>",
	"give":         "<person> <amount>",
	"help":         "[command]",
	"ignore":       "<player>",
	"info":         "[player]",
	"karma":        "[player]",
	"name":         "[text]",
	"narrate":      "<text>",
	"notts":        "<add|remove> <name> | list",
	"options":      "[list | ? <option> | <option> <value>]",
	"play":         "<instrument> <notes>",
	"ponder":       "<message>",
	"pose":         "<position> [direction]",
	"pray":         "<message>",
	"pull":         "<person | /critter>",
	"push":         "<person>",
	"report":       "<person> <reason>",
	"sell":         "<price> <buyer>",
	"setting":      "<search|get|set|reset> ...",
	"share":        "[player | /lock <player> | /unlock [/all]]",
	"show":         "[player]",
	"speak":        "<language>",
	"status":       "<text | /remove>",
	"tab":          "<number | next | previous>",
	"thank":        "<person> [reason]",
	"think":        "<message>",
	"thinkclan":    "<message>",
	"thinkgroup":   "<message>",
	"thinkto":      "<player> <message>",
	"unequip":      "<item | left | right>",
	"unshare":      "[player]",
	"use":          "[player | /lock ... | /unlock | /off]",
	"useitem":      "<item> [text]",
	"whisper":      "<message>",
	"who":          "[player]",
	"whoclan":      "[clan]",
	"yell":         "<message>",
}

var inputBarHelpEntries = []inputBarHelpEntry{
	{"Tab", "Accept a gray word completion when Autocomplete is enabled. Gray argument syntax is a guide and is not inserted."},
	{"Up / Down", "Browse sent-message history; Down after the newest entry clears the draft."},
	{"Enter", "Send the draft. If the input bar is closed, Enter opens it instead."},
	{"Esc", "Clear the draft and close the input bar when always-open is disabled."},
	{"Ctrl/Cmd+A", "Select the whole draft."},
	{"Ctrl/Cmd+C, X, V", "Copy, cut, or paste. Copy uses the whole draft when nothing is selected."},
	{"Ctrl+Left / Right", "Move by word. On Mac, use Option+Left / Right."},
	{"Ctrl+Backspace", "Delete the preceding word. On Mac, use Option+Backspace."},
	{"Cmd+Left / Right", "On Mac, move to the beginning or end of the draft."},
	{"Cmd+Backspace", "On Mac, delete back to the beginning of the draft."},
	{"Right-click", "Input: paste, copy, or clear. Chat and Console lines: copy the line."},
}

type commandPicker struct {
	win  *eui.WindowData
	list *eui.ItemData
	flow *eui.ItemData
}

func messageCommandButton(flow *eui.ItemData) *eui.ItemData {
	if flow == nil {
		return nil
	}
	index := 0
	if flow == chatInputFlow {
		index = 1
	}
	cache := &messageCommandButtons[index]
	if cache.flow == flow && cache.button != nil {
		return cache.button
	}
	button, events := eui.NewButton()
	button.Size = eui.Point{X: 28, Y: 28}
	setMaterialIconOnly(button, "terminal", "/")
	button.SetTooltip("Browse text commands and input-bar help")
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openMessageCommandPicker(flow, button)
		}
	}
	cache.flow, cache.button = flow, button
	return button
}

func openMessageCommandPicker(flow, anchor *eui.ItemData) {
	if activeCommandPicker != nil {
		activeCommandPicker.win.Close()
		return
	}
	picker := newCommandPicker(flow, selectedAppSession())
	activeCommandPicker = picker
	picker.win.OnClose = func() {
		picker.win.RemoveWindow()
		if activeCommandPicker == picker {
			activeCommandPicker = nil
		}
	}
	picker.win.MarkOpenNear(anchor)
}

func newCommandPicker(flow *eui.ItemData, session *Session) *commandPicker {
	width, height := commandPickerSize()
	picker := &commandPicker{flow: flow}
	picker.win = eui.NewWindow()
	picker.win.Title = "Text Commands"
	picker.win.Closable, picker.win.Movable, picker.win.AutoSize, picker.win.NoScroll = true, true, true, true
	picker.win.Resizable = false
	picker.win.Padding = 8

	root := eui.NewColumn()
	root.Size = eui.Point{X: width}
	hint := eui.NewWrappedLabel("Choose a command to start it in the input bar.", width)
	root.AddItem(hint)
	picker.list = eui.NewColumn()
	picker.list.Fixed, picker.list.Scrollable = true, true
	picker.list.Size = eui.Point{X: width, Y: height}
	root.AddItem(picker.list)
	picker.win.AddItem(root)
	picker.win.AddWindow(false)
	picker.rebuild(session)
	return picker
}

// commandPickerSize uses as much of the display as is useful without forcing
// the command list off-screen. The list remains scrollable after it reaches
// that responsive height.
func commandPickerSize() (width, height float32) {
	const (
		preferredWidth  float32 = 560
		minimumWidth    float32 = 360
		preferredHeight float32 = 660
		minimumHeight   float32 = 240
		windowAllowance float32 = 120
	)
	screenWidth, screenHeight := eui.ScreenSize()
	scale := eui.UIScale()
	if scale <= 0 {
		scale = 1
	}
	if screenWidth <= 0 || screenHeight <= 0 {
		return preferredWidth, preferredHeight
	}
	availableWidth := float32(screenWidth)/scale - 40
	availableHeight := float32(screenHeight)/scale - windowAllowance
	width = preferredWidth
	if availableWidth < width {
		width = availableWidth
	}
	if width < minimumWidth {
		width = max(1, availableWidth)
	}
	height = preferredHeight
	if availableHeight < height {
		height = availableHeight
	}
	if height < minimumHeight {
		height = max(1, availableHeight)
	}
	return width, height
}

func (picker *commandPicker) rebuild(session *Session) {
	if picker == nil || picker.list == nil {
		return
	}
	picker.list.Contents = picker.list.Contents[:0]
	addInputBarHelp(picker.list)
	for _, group := range availableTextCommandGroups(session) {
		heading := eui.NewLabel(group.typeName + " — " + group.source)
		heading.FontSize = 12
		heading.Size = eui.Point{X: picker.list.Size.X, Y: 24}
		picker.list.AddItem(heading)
		for _, command := range group.commands {
			command := command
			button, events := eui.NewButton()
			description := textCommandDescription(group, command)
			syntax := textCommandSyntax(group, command)
			button.Text = command + "\n" + description
			button.Prediction = syntax
			button.FontSize = 11
			button.Size = eui.Point{X: picker.list.Size.X - eui.ScrollbarWidth()/eui.UIScale(), Y: 42}
			button.ConstrainToSize = true
			button.SetTooltip(description + " Syntax: " + command + syntax + ". Select to start it in the input bar.")
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					startMessageCommand(picker.flow, command)
					picker.win.Close()
				}
			}
			picker.list.AddItem(button)
		}
	}
	if len(picker.list.Contents) == 0 {
		picker.list.AddItem(eui.NewLabel("No text commands are available."))
	}
}

func textCommandSyntax(group textCommandGroup, command string) string {
	if group.typeName == "Script" || group.typeName == "Macro" {
		return " [arguments]"
	}
	if syntax := textCommandArgumentSyntax[normalizeScriptCommand(command)]; syntax != "" {
		return " " + syntax
	}
	return ""
}

func addInputBarHelp(list *eui.ItemData) {
	if list == nil {
		return
	}
	heading := eui.NewLabel("Input bar help")
	heading.FontSize = 12
	heading.Size = eui.Point{X: list.Size.X, Y: 24}
	list.AddItem(heading)
	for _, entry := range inputBarHelpEntries {
		line := eui.NewWrappedLabel(entry.shortcut+" — "+entry.description, list.Size.X-eui.ScrollbarWidth()/eui.UIScale())
		line.FontSize = 10
		line.SelectableText = true
		list.AddItem(line)
	}
}

func textCommandDescription(group textCommandGroup, command string) string {
	name := normalizeScriptCommand(command)
	switch group.typeName {
	case "Client":
		if description := goThoomCommandDescriptions[name]; description != "" {
			return description
		}
		return "Run this goThoom command."
	case "Server":
		if description := serverCommandDescriptions[name]; description != "" {
			return description
		}
		return "Send this command to the Clan Lord server."
	case "Script":
		return "Run the command registered by " + group.source + "."
	case "Macro":
		return "Run this text macro from " + group.source + "."
	default:
		return "Start this command in the input bar."
	}
}

func startMessageCommand(flow *eui.ItemData, command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	inputMu.Lock()
	inputText = []rune(command + " ")
	inputPos = len(inputText)
	inputActive = true
	inputMu.Unlock()
	selectedMessageInput = flow
	spellDirty = true
	updateMessageInputWindows()
	if item := messageInputItem(flow); item != nil {
		eui.Focus(item)
		item.CursorPos = wrappedCursorPos(item.Text, inputPos)
		item.SelectStart, item.SelectEnd = item.CursorPos, item.CursorPos
	}
}

// availableTextCommandGroups returns every command the selected session can
// currently accept, grouped by command type and the feature that supplied it.
func availableTextCommandGroups(session *Session) []textCommandGroup {
	groups := []textCommandGroup{{typeName: "Client", source: "goThoom", commands: slashCommands(goThoomTextCommands)}}
	groups = append(groups, scriptTextCommandGroups(session)...)
	groups = append(groups, legacyMacroTextCommandGroups(session)...)
	groups = append(groups, textCommandGroup{typeName: "Server", source: "Clan Lord", commands: slashCommands(serverCommandNames)})
	groups = mergeTextCommandGroups(groups)
	for index := range groups {
		groups[index].commands = sortedUniqueCandidates(groups[index].commands)
	}
	return groups
}

func mergeTextCommandGroups(groups []textCommandGroup) []textCommandGroup {
	merged := make([]textCommandGroup, 0, len(groups))
	indexes := make(map[string]int, len(groups))
	for _, group := range groups {
		key := group.typeName + "\x00" + group.source
		if index, ok := indexes[key]; ok {
			merged[index].commands = append(merged[index].commands, group.commands...)
			continue
		}
		indexes[key] = len(merged)
		merged = append(merged, group)
	}
	return merged
}

func slashCommands(names []string) []string {
	commands := make([]string, 0, len(names))
	for _, name := range names {
		name = normalizeScriptCommand(name)
		if name != "" {
			commands = append(commands, "/"+name)
		}
	}
	return commands
}

func scriptTextCommandGroups(session *Session) []textCommandGroup {
	byOwner := make(map[string][]string)
	if session != nil && session.automation != nil {
		session.automation.scriptMu.RLock()
		for name, entry := range session.automation.localCommands {
			byOwner[entry.owner] = append(byOwner[entry.owner], "/"+name)
		}
		session.automation.scriptMu.RUnlock()
	} else {
		scriptMu.RLock()
		for name, owner := range scriptCommandOwners {
			if scriptCommands[name] != nil && !scriptDisabled[owner] {
				byOwner[owner] = append(byOwner[owner], "/"+name)
			}
		}
		scriptMu.RUnlock()
	}
	owners := make([]string, 0, len(byOwner))
	for owner := range byOwner {
		owners = append(owners, owner)
	}
	sort.Slice(owners, func(i, j int) bool {
		return strings.ToLower(scriptDisplayName(owners[i])) < strings.ToLower(scriptDisplayName(owners[j]))
	})
	groups := make([]textCommandGroup, 0, len(owners))
	for _, owner := range owners {
		if owner == "client" {
			groups = append(groups, textCommandGroup{typeName: "Client", source: "goThoom", commands: byOwner[owner]})
			continue
		}
		groups = append(groups, textCommandGroup{typeName: "Script", source: scriptDisplayName(owner), commands: byOwner[owner]})
	}
	return groups
}

func legacyMacroTextCommandGroups(session *Session) []textCommandGroup {
	if session == nil {
		return nil
	}
	bySource := make(map[string][]string)
	for _, declaration := range session.legacyMacroProgramSnapshot().Macros {
		if declaration.Kind != legacyMacroExpression || strings.TrimSpace(declaration.Trigger) == "" {
			continue
		}
		source := filepath.Base(declaration.Header.Path)
		if source == "." || source == "" {
			source = "Legacy macros"
		}
		bySource[source] = append(bySource[source], declaration.Trigger)
	}
	sources := make([]string, 0, len(bySource))
	for source := range bySource {
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return strings.ToLower(sources[i]) < strings.ToLower(sources[j]) })
	groups := make([]textCommandGroup, 0, len(sources))
	for _, source := range sources {
		groups = append(groups, textCommandGroup{typeName: "Macro", source: source, commands: bySource[source]})
	}
	return groups
}
