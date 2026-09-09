package main

import (
	"fmt"
	"strings"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"gothoom/eui"
)

type emojiPicker struct {
	win                            *eui.WindowData
	search, sidebar, panel, status *eui.ItemData
	group                          string
	groupButtons                   map[string]*eui.ItemData
	choose                         func(emojiEntry)
	columns                        int
}

var activeEmojiPicker *emojiPicker
var messageEmojiButtons [2]struct{ flow, button *eui.ItemData }

func closeEmojiPicker() {
	if activeEmojiPicker != nil {
		activeEmojiPicker.win.Close()
	}
}

func messageEmojiButton(flow *eui.ItemData) *eui.ItemData {
	if !gs.ExpandEmojiNames || flow == nil {
		return nil
	}
	index := 0
	if flow == chatInputFlow {
		index = 1
	}
	cache := &messageEmojiButtons[index]
	if cache.flow == flow {
		styleMessageEmojiButton(cache.button)
		return cache.button
	}
	button, events := eui.NewButton()
	button.Size = eui.Point{X: 28, Y: 28}
	button.Position = eui.Point{X: 2}
	styleMessageEmojiButton(button)
	button.SetTooltip("Choose an emoji")
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick && gs.ExpandEmojiNames {
			openMessageEmojiPicker(flow, button)
		}
	}
	cache.flow, cache.button = flow, button
	return button
}

func styleMessageEmojiButton(button *eui.ItemData) {
	button.FontSize = 18
	if emojiFaceSource == nil {
		button.Text = ":)"
		return
	}
	button.Text = "😄"
	button.Face = withEmojiFace(&text.GoTextFace{Source: eui.FontSource(), Size: float64(button.FontSize*eui.UIScale()) + 2})
}

func openMessageEmojiPicker(flow, anchor *eui.ItemData) {
	if activeEmojiPicker != nil {
		closeEmojiPicker()
		return
	}
	selectedMessageInput = flow
	before := string(inputText)
	start, end := inputPos, inputPos
	if item := messageInputItem(flow); item != nil && item.SelectStart != item.SelectEnd {
		start, end = plainCursorPos(item.Text, item.SelectStart), plainCursorPos(item.Text, item.SelectEnd)
		if start > end {
			start, end = end, start
		}
	}
	picker := newEmojiPicker(func(entry emojiEntry) {
		if string(inputText) != before {
			start, end = inputPos, inputPos
		}
		insertMessageEmoji(flow, encodeEmojiShortcodes(entry.Emoji), start, end)
	})
	activeEmojiPicker = picker
	picker.win.OnClose = func() {
		eui.ClearFocus(picker.search)
		picker.win.RemoveWindow()
		if activeEmojiPicker == picker {
			activeEmojiPicker = nil
		}
	}
	picker.win.MarkOpenNear(anchor)
	eui.Focus(picker.search)
}

func insertMessageEmoji(flow *eui.ItemData, shortcode string, start, end int) {
	inputMu.Lock()
	start = max(0, min(start, len(inputText)))
	end = max(start, min(end, len(inputText)))
	insert := []rune(shortcode)
	updated := append([]rune(nil), inputText[:start]...)
	updated = append(updated, insert...)
	inputText = append(updated, inputText[end:]...)
	inputPos = start + len(insert)
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
	syncNativeChatInput()
}

func emojiGroupNames() []string {
	var groups []string
	for _, entry := range emojiCatalog {
		if len(groups) == 0 || groups[len(groups)-1] != entry.Group {
			groups = append(groups, entry.Group)
		}
	}
	return groups
}

func filterEmojiCatalog(group, query string) []emojiEntry {
	query = strings.TrimSpace(strings.ReplaceAll(strings.ToLower(query), "_", " "))
	query = strings.Trim(query, ":")
	var result []emojiEntry
	for _, entry := range emojiCatalog {
		if query == "" {
			if entry.Group == group {
				result = append(result, entry)
			}
			continue
		}
		searchable := strings.ToLower(entry.Name + " " + entry.Subgroup + " " + entry.Group + " " + strings.ReplaceAll(encodeEmojiShortcodes(entry.Emoji), "_", " "))
		matches := entry.Emoji == query
		if !matches {
			matches = true
			for _, word := range strings.Fields(query) {
				if !strings.Contains(searchable, word) {
					matches = false
					break
				}
			}
		}
		if matches {
			result = append(result, entry)
		}
	}
	return result
}

func newEmojiPicker(choose func(emojiEntry)) *emojiPicker {
	const cell float32 = 40
	scale := eui.UIScale()
	if scale <= 0 {
		scale = 1
	}
	w, h := eui.ScreenSize()
	width, height := float32(520), float32(330)
	if w > 0 {
		width = float32(max(360, min(int(width), int(float32(w)/scale)-48)))
	}
	if h > 0 {
		height = float32(max(240, min(int(height), int(float32(h)/scale)-130)))
	}
	sideWidth := float32(152)
	panelWidth := width - sideWidth - 8
	win := eui.NewWindow()
	win.Title = "Emoji"
	win.Closable, win.Movable, win.AutoSize, win.NoScroll = true, true, true, true
	win.Resizable = false
	win.Padding = 8
	p := &emojiPicker{win: win, choose: choose, groupButtons: make(map[string]*eui.ItemData)}
	p.columns = max(1, int((panelWidth-eui.ScrollbarWidth()/scale)/cell))
	root := eui.NewColumn()
	root.Size.X = width
	search, events := eui.NewInput()
	p.search = search
	search.Label = "Search emoji"
	search.Size = eui.Point{X: width, Y: 28}
	search.SetTooltip("Search names such as smile, heart, or thumbs_up.")
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			search.Text = ev.Text
			p.rebuild()
		}
	}
	root.AddItem(search)
	root.AddItem(&eui.ItemData{ItemType: eui.ITEM_FLOW, Fixed: true, Size: eui.Point{X: width, Y: 8}})
	row := eui.NewRow()
	p.sidebar = eui.NewColumn()
	p.sidebar.Fixed = true
	p.sidebar.Size = eui.Point{X: sideWidth, Y: height}
	p.panel = eui.NewColumn()
	p.panel.Fixed = true
	p.panel.Scrollable = true
	p.panel.Size = eui.Point{X: panelWidth, Y: height}
	p.panel.Position.X = 8
	row.AddItem(p.sidebar)
	row.AddItem(p.panel)
	root.AddItem(row)
	groups := emojiGroupNames()
	if len(groups) > 0 {
		p.group = groups[0]
	}
	for _, group := range groups {
		button, events := eui.NewButton()
		button.Text = group
		button.FontSize = 11
		button.Position = eui.Point{Y: 2}
		button.Size = eui.Point{X: sideWidth, Y: float32(min(30, int(height)/len(groups))) - 2}
		p.groupButtons[group] = button
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				p.group = group
				p.search.Text = ""
				p.search.CursorPos = 0
				p.rebuild()
			}
		}
		p.sidebar.AddItem(button)
	}
	p.status = eui.NewLabel("")
	p.status.Size = eui.Point{X: width, Y: 24}
	root.AddItem(p.status)
	win.AddItem(root)
	win.OnClose = win.RemoveWindow
	win.AddWindow(false)
	p.rebuild()
	return p
}

func (p *emojiPicker) rebuild() {
	entries := filterEmojiCatalog(p.group, p.search.Text)
	items := make([]*eui.ItemData, 0, len(entries)/p.columns+32)
	var row *eui.ItemData
	lastSubgroup := ""
	for _, entry := range entries {
		if entry.Subgroup != lastSubgroup {
			caption := strings.ReplaceAll(entry.Subgroup, "-", " ")
			if caption != "" {
				caption = strings.ToUpper(caption[:1]) + caption[1:]
			}
			label := eui.NewLabel(caption)
			label.Size = eui.Point{X: p.panel.Size.X - eui.ScrollbarWidth()/eui.UIScale(), Y: 26}
			items = append(items, label)
			row = nil
			lastSubgroup = entry.Subgroup
		}
		if row == nil || len(row.Contents) == p.columns {
			row = eui.NewRow()
			items = append(items, row)
		}
		button, events := eui.NewButton()
		button.Text = entry.Emoji
		button.FontSize = 22
		button.Size = eui.Point{X: 38, Y: 38}
		button.Position = eui.Point{X: 2, Y: 2}
		button.Face = &text.GoTextFace{Source: emojiFaceSource, Size: float64(button.FontSize*eui.UIScale()) + 2}
		button.SetTooltip(entry.Name + " (" + encodeEmojiShortcodes(entry.Emoji) + ")")
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick && gs.ExpandEmojiNames {
				p.win.Close()
				if p.choose != nil {
					p.choose(entry)
				}
			}
		}
		row.AddItem(button)
	}
	if len(entries) == 0 {
		items = append(items, eui.NewLabel("No matching emoji"))
	}
	p.panel.SetItems(items)
	p.panel.Scroll.Y = 0
	for group, button := range p.groupButtons {
		button.Filled = p.search.Text == "" && group == p.group
	}
	p.status.Text = fmt.Sprintf("%d emoji · Choose one to insert its name", len(entries))
	p.win.Refresh()
}
