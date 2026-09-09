//go:build script

package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gt2"
)

const scriptID = "mark-beasts"
const scriptName = "Mark Beasts"
const scriptAuthor = "goThoom"
const scriptCategory = "utility"
const scriptDescription = "Keep notes and colored highlights for creatures and characters."
const scriptAPIVersion = 2

type lastyMark struct {
	R        uint8
	G        uint8
	B        uint8
	A        uint8
	OutlineR uint8
	OutlineG uint8
	OutlineB uint8
	OutlineA uint8
}

type hitFlashSettings struct {
	Enabled    bool
	AllMobiles bool
	Color      lastyMark
}

type spriteEntry struct {
	Key  int
	ID   string
	Name string
	Note string
	Mark lastyMark
}

var marks = map[uint16]lastyMark{}
var namedMarks = map[string]lastyMark{}
var entries []spriteEntry
var nextEntryKey = 1
var lastiesWindow gt2.Window
var effectsWindow gt2.Window
var showOutline = true
var hitFlash = hitFlashSettings{Enabled: true, Color: lastyMark{R: 255, G: 48, B: 48, A: 255}}

func defaultMark() lastyMark {
	return withOutline(lastyMark{R: 255, G: 255, B: 128, A: 255}, lastyMark{R: 255, G: 255, B: 0, A: 255})
}

func Init() {
	if !gt2.LoadJSON("entries", &entries) {
		gt2.LoadJSON("marks", &marks)
		ids := make([]int, 0, len(marks))
		for id := range marks {
			ids = append(ids, int(id))
		}
		sort.Ints(ids)
		for _, id := range ids {
			mark := marks[uint16(id)]
			if mark.OutlineA == 0 {
				mark.OutlineR, mark.OutlineG, mark.OutlineB, mark.OutlineA = mark.R, mark.G, mark.B, 255
			}
			entries = append(entries, spriteEntry{ID: strconv.Itoa(id), Mark: mark})
		}
	}
	for i := range entries {
		entries[i].Key = nextEntryKey
		nextEntryKey++
	}
	showOutline = gt2.LoadBool("show-outline", true)
	gt2.LoadJSON("hit-flash", &hitFlash)
	lastiesWindow = gt2.CreateWindow(gt2.WindowOptions{
		Title: "Mark Beasts",
		Width: 680,
		Text:  "Changes save automatically. Alt-click a creature or player to add or remove it.",
		Rows:  spriteRows(),
		Buttons: []gt2.WindowButton{
			{ID: "add", Label: "Add", OnClick: addMark},
			{ID: "effects", Label: "Effects", OnClick: showEffects},
		},
	})
	gt2.Command("lasties", lastiesCommand)
	gt2.Command("marks", lastiesCommand)
	gt2.AddToolbar(gt2.ToolbarOptions{Label: "Mark Beasts", Buttons: []gt2.ToolbarButton{{
		Label: "Mark Beasts", Tooltip: "Open Mark Beasts", OnClick: showWindow,
	}}})
	gt2.Bind("Alt-LeftClick", toggleClickedMobile)
	gt2.OnWorld(updateWorld)
	persistEntries()
}

func lastiesCommand(args string) {
	if strings.EqualFold(strings.TrimSpace(args), "clear") {
		entries = nil
		persistEntries()
		refreshWindow()
		gt2.Print("Cleared sprite marks and notes.")
		return
	}
	showWindow()
}

func showWindow() {
	lastiesWindow.Show()
}

func rowControlID(key int, field string) string {
	return fmt.Sprintf("entry-%d-%s", key, field)
}

func spriteRows() []gt2.WindowRow {
	rows := make([]gt2.WindowRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, spriteRow(entry))
	}
	return rows
}

// Give each row's interpreted callbacks their own entry binding.
func spriteRow(entry spriteEntry) gt2.WindowRow {
	key := entry.Key
	id, _ := parseSpriteID(entry.ID)
	outline := lastyMark{R: entry.Mark.OutlineR, G: entry.Mark.OutlineG, B: entry.Mark.OutlineB, A: entry.Mark.OutlineA}
	return gt2.WindowRow{
		Controls: []gt2.WindowControl{
			{ID: rowControlID(key, "image"), Label: "Sprite", Kind: gt2.ControlImage, Image: id, Width: 48},
			{ID: rowControlID(key, "id"), Label: "ID", Kind: gt2.ControlText, Text: entry.ID, Width: 64, OnChange: func(e gt2.WindowControlEvent) { changeID(key, e.Text) }},
			{ID: rowControlID(key, "name"), Label: "Name", Kind: gt2.ControlText, Text: entry.Name, Width: 128, Tooltip: "Match this name instead of the sprite ID.", OnChange: func(e gt2.WindowControlEvent) { changeName(key, e.Text) }},
			{ID: rowControlID(key, "tint"), Label: "Tint", Kind: gt2.ControlColor, Color: packColor(entry.Mark), Width: 64, OnChange: func(e gt2.WindowControlEvent) { changeColor(key, e.Color, false) }},
			{ID: rowControlID(key, "outline"), Label: "Outline", Kind: gt2.ControlColor, Color: packColor(outline), Width: 64, OnChange: func(e gt2.WindowControlEvent) { changeColor(key, e.Color, true) }},
			{ID: rowControlID(key, "note"), Label: "Notes", Kind: gt2.ControlText, Text: entry.Note, Width: 220, OnChange: func(e gt2.WindowControlEvent) { changeNote(key, e.Text) }},
		},
		Buttons: []gt2.WindowButton{{ID: rowControlID(key, "delete"), Label: "×", Tooltip: "Delete this entry", Width: 32, OnClick: func() { removeMark(key) }}},
	}
}

func addMark() {
	if len(entries) >= 256 {
		return
	}
	entries = append(entries, spriteEntry{Key: nextEntryKey, Mark: defaultMark()})
	nextEntryKey++
	persistEntries()
	refreshWindow()
}

func changeID(key int, value string) {
	for i := range entries {
		if entries[i].Key == key {
			entries[i].ID = value
			id, _ := parseSpriteID(value)
			lastiesWindow.SetControlImage(rowControlID(key, "image"), id)
			persistEntries()
			return
		}
	}
}

func changeName(key int, value string) {
	for i := range entries {
		if entries[i].Key == key {
			entries[i].Name = value
			persistEntries()
			return
		}
	}
}

func changeNote(key int, value string) {
	for i := range entries {
		if entries[i].Key == key {
			entries[i].Note = value
			gt2.Store("entries", entries)
			return
		}
	}
}

func changeColor(key int, value uint32, outline bool) {
	for i := range entries {
		if entries[i].Key != key {
			continue
		}
		color := unpackColor(value)
		if outline {
			entries[i].Mark.OutlineR, entries[i].Mark.OutlineG, entries[i].Mark.OutlineB, entries[i].Mark.OutlineA = color.R, color.G, color.B, color.A
		} else {
			entries[i].Mark.R, entries[i].Mark.G, entries[i].Mark.B, entries[i].Mark.A = color.R, color.G, color.B, color.A
		}
		persistEntries()
		return
	}
}

func removeMark(key int) {
	for i, entry := range entries {
		if entry.Key == key {
			entries = append(entries[:i], entries[i+1:]...)
			persistEntries()
			refreshWindow()
			return
		}
	}
}

func toggleClickedMobile(event gt2.InputEvent) {
	if !event.OnMobile || event.Mobile.PictID == 0 || event.Mobile.PictID == 0xffff {
		return
	}
	event.Consume()
	name := strings.TrimSpace(event.Mobile.Name)
	for _, entry := range entries {
		if name != "" {
			if normalizeName(entry.Name) == normalizeName(name) {
				removeMark(entry.Key)
				return
			}
			continue
		}
		if normalizeName(entry.Name) != "" {
			continue
		}
		id, ok := parseSpriteID(entry.ID)
		if ok && id == event.Mobile.PictID {
			removeMark(entry.Key)
			return
		}
	}
	if len(entries) < 256 {
		entries = append(entries, spriteEntry{Key: nextEntryKey, ID: strconv.Itoa(int(event.Mobile.PictID)), Name: name, Mark: defaultMark()})
		nextEntryKey++
		persistEntries()
		refreshWindow()
	}
}

func persistEntries() {
	marks = map[uint16]lastyMark{}
	namedMarks = map[string]lastyMark{}
	message := "Changes save automatically. Alt-click a creature or player to add or remove it."
	for _, entry := range entries {
		if name := normalizeName(entry.Name); name != "" {
			if _, duplicate := namedMarks[name]; duplicate {
				message = fmt.Sprintf("%s is listed twice. Only the first row applies.", strings.TrimSpace(entry.Name))
				continue
			}
			namedMarks[name] = entry.Mark
			continue
		}
		id, ok := parseSpriteID(entry.ID)
		if !ok {
			if strings.TrimSpace(entry.ID) != "" {
				message = "Use sprite IDs from 1 to 65534. Invalid IDs have no effect."
			}
			continue
		}
		if _, duplicate := marks[id]; duplicate {
			message = fmt.Sprintf("Sprite %d is listed twice. Only its first row applies.", id)
			continue
		}
		marks[id] = entry.Mark
	}
	gt2.Store("entries", entries)
	gt2.Store("marks", marks)
	lastiesWindow.SetText(message)
	applyTints()
}

func refreshWindow() {
	lastiesWindow.SetRows(spriteRows())
	lastiesWindow.SetButtonEnabled("add", len(entries) < 256)
}

func showEffects() {
	if effectsWindow.Active() {
		effectsWindow.Show()
		return
	}
	effectsWindow = gt2.CreateWindow(gt2.WindowOptions{
		Title: "Beast Effects", Width: 320,
		Text: "Applies to every marked creature or player. Changes save automatically.",
		Controls: []gt2.WindowControl{
			{ID: "outline", Label: "Show outlines", Kind: gt2.ControlCheckbox, Checked: showOutline, OnChange: func(e gt2.WindowControlEvent) {
				showOutline = e.Checked
				gt2.Store("show-outline", showOutline)
				applyTints()
			}},
			{ID: "flash-hits", Label: "Flash when blood appears", Kind: gt2.ControlCheckbox, Checked: hitFlash.Enabled, OnChange: func(e gt2.WindowControlEvent) {
				hitFlash.Enabled = e.Checked
				persistHitFlash()
			}},
			{ID: "flash-color", Label: "Flash color", Kind: gt2.ControlColor, Color: packColor(hitFlash.Color), OnChange: func(e gt2.WindowControlEvent) {
				hitFlash.Color = unpackColor(e.Color)
				persistHitFlash()
			}},
			{ID: "flash-all", Label: "Flash all mobiles", Kind: gt2.ControlCheckbox, Checked: hitFlash.AllMobiles, Tooltip: "Include mobiles that aren't in your list.", OnChange: func(e gt2.WindowControlEvent) {
				hitFlash.AllMobiles = e.Checked
				persistHitFlash()
			}},
		},
	})
}

func persistHitFlash() {
	gt2.Store("hit-flash", hitFlash)
}

func applyTints() {
	gt2.ClearMobileTints()
	gt2.ClearMobileOutlines()
	for id, tint := range marks {
		gt2.SetMobileTint(id, tint.R, tint.G, tint.B, tint.A)
		if showOutline {
			gt2.SetMobileOutline(id, tint.OutlineR, tint.OutlineG, tint.OutlineB, tint.OutlineA)
		}
	}
	for name, tint := range namedMarks {
		gt2.SetNamedMobileTint(name, tint.R, tint.G, tint.B, tint.A)
		if showOutline {
			gt2.SetNamedMobileOutline(name, tint.OutlineR, tint.OutlineG, tint.OutlineB, tint.OutlineA)
		}
	}
}

func updateWorld(world gt2.World) {
	flashHitMobiles(world)
}

func flashHitMobiles(world gt2.World) {
	if !hitFlash.Enabled {
		return
	}
	for _, picture := range world.Pictures {
		if picture.PictID != 33 {
			continue
		}
		centerX := int(picture.H) + picture.Width/2
		centerY := int(picture.V) + picture.Height/2
		for _, mobile := range world.Mobiles {
			if mobile.Stale || (!hitFlash.AllMobiles && !isMarked(mobile)) {
				continue
			}
			radius := mobile.Size / 4
			if radius < 8 {
				radius = 8
			}
			if abs(centerX-int(mobile.H)) <= radius && abs(centerY-int(mobile.V)) <= radius {
				color := hitFlash.Color
				gt2.FlashMobile(mobile.Index, color.R, color.G, color.B, color.A, 180*time.Millisecond)
			}
		}
	}
}

func isMarked(mobile gt2.Mobile) bool {
	if _, marked := namedMarks[normalizeName(mobile.Name)]; marked {
		return true
	}
	_, marked := marks[mobile.PictID]
	return marked
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func parseSpriteID(value string) (uint16, bool) {
	number, err := strconv.ParseUint(strings.TrimSpace(value), 10, 16)
	if err != nil || number == 0 || number == 0xffff {
		return 0, false
	}
	return uint16(number), true
}

func packColor(mark lastyMark) uint32 {
	return uint32(mark.R)<<24 | uint32(mark.G)<<16 | uint32(mark.B)<<8 | uint32(mark.A)
}

func unpackColor(value uint32) lastyMark {
	return lastyMark{R: uint8(value >> 24), G: uint8(value >> 16), B: uint8(value >> 8), A: uint8(value)}
}

func withOutline(tint, outline lastyMark) lastyMark {
	tint.OutlineR, tint.OutlineG, tint.OutlineB, tint.OutlineA = outline.R, outline.G, outline.B, outline.A
	return tint
}
