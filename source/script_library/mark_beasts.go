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

type markColor struct {
	R, G, B, A uint8
}

type markStyle struct {
	Tint, Outline               markColor
	TintEnabled, OutlineEnabled bool
}

type hitFlashSettings struct {
	Enabled    bool
	AllMobiles bool
	Color      markColor
}

type spriteEntry struct {
	Key  int
	ID   string
	Name string
	Note string
	Mark markStyle
}

// Read the original flat color fields only when migrating saved entries.
type legacyMark struct {
	R, G, B, A                             uint8
	OutlineR, OutlineG, OutlineB, OutlineA uint8
}

type legacyEntry struct {
	ID, Name, Note string
	Mark           legacyMark
}

var marks = map[uint16]markStyle{}
var namedMarks = map[string]markStyle{}
var entries []spriteEntry
var nextEntryKey = 1
var lastiesWindow gt2.Window
var markDefaults = markStyle{
	Tint:           markColor{R: 255, G: 255, B: 128, A: 255},
	Outline:        markColor{R: 255, G: 255, B: 0, A: 255},
	OutlineEnabled: true,
}
var hitFlash = hitFlashSettings{Color: markColor{R: 255, G: 48, B: 48, A: 255}}

func defaultMark() markStyle {
	return markDefaults
}

func loadEntries() {
	if gt2.LoadInteger("entries-version", 0) >= 2 {
		gt2.LoadJSON("entries", &entries)
		return
	}
	var oldEntries []legacyEntry
	if !gt2.LoadJSON("entries", &oldEntries) {
		oldMarks := map[uint16]legacyMark{}
		gt2.LoadJSON("marks", &oldMarks)
		ids := make([]int, 0, len(oldMarks))
		for id := range oldMarks {
			ids = append(ids, int(id))
		}
		sort.Ints(ids)
		for _, id := range ids {
			mark := oldMarks[uint16(id)]
			if mark.OutlineA == 0 {
				mark.OutlineR, mark.OutlineG, mark.OutlineB, mark.OutlineA = mark.R, mark.G, mark.B, 255
			}
			oldEntries = append(oldEntries, legacyEntry{ID: strconv.Itoa(id), Mark: mark})
		}
	}
	outlineEnabled := gt2.LoadBool("show-outline", true)
	for _, entry := range oldEntries {
		mark := entry.Mark
		entries = append(entries, spriteEntry{ID: entry.ID, Name: entry.Name, Note: entry.Note, Mark: markStyle{
			Tint:        markColor{R: mark.R, G: mark.G, B: mark.B, A: mark.A},
			Outline:     markColor{R: mark.OutlineR, G: mark.OutlineG, B: mark.OutlineB, A: mark.OutlineA},
			TintEnabled: true, OutlineEnabled: outlineEnabled,
		}})
	}
}

func Init() {
	loadEntries()
	// These legacy values become defaults only for installations migrating from
	// the former custom settings window. Native preferences own future changes.
	gt2.LoadJSON("mark-defaults", &markDefaults)
	gt2.LoadJSON("hit-flash", &hitFlash)
	loadPreferences()
	for i := range entries {
		entries[i].Key = nextEntryKey
		nextEntryKey++
	}
	lastiesWindow = gt2.CreateWindow(gt2.WindowOptions{
		Title: "Mark Beasts",
		Width: 680,
		Text:  "Changes save automatically. Alt-click a creature or player to add or remove it.",
		Rows:  spriteRows(),
		Buttons: []gt2.WindowButton{
			{ID: "add", Label: "Add", OnClick: addMark},
			{ID: "settings", Label: "Settings", Icon: "settings", Tooltip: "Open Mark Beasts preferences", OnClick: gt2.OpenSettings},
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

func loadPreferences() {
	markDefaults.TintEnabled = gt2.Bool(gt2.BoolOption{
		Key: "default-tint", Label: "Tint new entries", Default: markDefaults.TintEnabled,
		OnChange: func(value bool) { markDefaults.TintEnabled = value },
	})
	markDefaults.Tint = unpackColor(gt2.Color(gt2.ColorOption{
		Key: "default-tint-color", Label: "Default tint color", Default: packColor(markDefaults.Tint),
		OnChange: func(value uint32) { markDefaults.Tint = unpackColor(value) },
	}))
	markDefaults.OutlineEnabled = gt2.Bool(gt2.BoolOption{
		Key: "default-outline", Label: "Outline new entries", Default: markDefaults.OutlineEnabled,
		OnChange: func(value bool) { markDefaults.OutlineEnabled = value },
	})
	markDefaults.Outline = unpackColor(gt2.Color(gt2.ColorOption{
		Key: "default-outline-color", Label: "Default outline color", Default: packColor(markDefaults.Outline),
		OnChange: func(value uint32) { markDefaults.Outline = unpackColor(value) },
	}))
	hitFlash.Enabled = gt2.Bool(gt2.BoolOption{
		Key: "flash-hits", Label: "Flash when blood appears", Default: hitFlash.Enabled,
		OnChange: func(value bool) { hitFlash.Enabled = value },
	})
	hitFlash.Color = unpackColor(gt2.Color(gt2.ColorOption{
		Key: "flash-color", Label: "Flash color", Default: packColor(hitFlash.Color),
		OnChange: func(value uint32) { hitFlash.Color = unpackColor(value) },
	}))
	hitFlash.AllMobiles = gt2.Bool(gt2.BoolOption{
		Key: "flash-all", Label: "Flash all mobiles", Help: "Include mobiles that aren't in your list.", Default: hitFlash.AllMobiles,
		OnChange: func(value bool) { hitFlash.AllMobiles = value },
	})
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
	return gt2.WindowRow{
		Controls: []gt2.WindowControl{
			{ID: rowControlID(key, "image"), Label: "Sprite", Kind: gt2.ControlImage, Image: id, Width: 40},
			{ID: rowControlID(key, "id"), Label: "ID", Kind: gt2.ControlText, Text: entry.ID, Width: 48, OnChange: func(e gt2.WindowControlEvent) { changeID(key, e.Text) }},
			{ID: rowControlID(key, "name"), Label: "Name", Kind: gt2.ControlText, Text: entry.Name, Width: 96, Tooltip: "Match this name instead of the sprite ID.", OnChange: func(e gt2.WindowControlEvent) { changeName(key, e.Text) }},
			{ID: rowControlID(key, "tint-enabled"), Label: "Tint", Kind: gt2.ControlCheckbox, Checked: entry.Mark.TintEnabled, Width: 56, Tooltip: "Enable tint for this entry.", OnChange: func(e gt2.WindowControlEvent) { changeEffect(key, e.Checked, false) }},
			{ID: rowControlID(key, "tint"), Label: "Color", Kind: gt2.ControlColor, Color: packColor(entry.Mark.Tint), Width: 40, OnChange: func(e gt2.WindowControlEvent) { changeColor(key, e.Color, false) }},
			{ID: rowControlID(key, "outline-enabled"), Label: "Outline", Kind: gt2.ControlCheckbox, Checked: entry.Mark.OutlineEnabled, Width: 76, Tooltip: "Enable outline for this entry.", OnChange: func(e gt2.WindowControlEvent) { changeEffect(key, e.Checked, true) }},
			{ID: rowControlID(key, "outline"), Label: "Color", Kind: gt2.ControlColor, Color: packColor(entry.Mark.Outline), Width: 40, OnChange: func(e gt2.WindowControlEvent) { changeColor(key, e.Color, true) }},
			{ID: rowControlID(key, "note"), Label: "Notes", Kind: gt2.ControlText, Text: entry.Note, Width: 140, OnChange: func(e gt2.WindowControlEvent) { changeNote(key, e.Text) }},
		},
		Buttons: []gt2.WindowButton{{ID: rowControlID(key, "delete"), Label: "×", Tooltip: "Delete this entry", Width: 28, OnClick: func() { removeMark(key) }}},
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
			entries[i].Mark.Outline = color
		} else {
			entries[i].Mark.Tint = color
		}
		persistEntries()
		return
	}
}

func changeEffect(key int, enabled, outline bool) {
	for i := range entries {
		if entries[i].Key != key {
			continue
		}
		if outline {
			entries[i].Mark.OutlineEnabled = enabled
		} else {
			entries[i].Mark.TintEnabled = enabled
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
	marks = map[uint16]markStyle{}
	namedMarks = map[string]markStyle{}
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
	gt2.Store("entries-version", 2)
	lastiesWindow.SetText(message)
	applyMarks()
}

func refreshWindow() {
	lastiesWindow.SetRows(spriteRows())
	lastiesWindow.SetButtonEnabled("add", len(entries) < 256)
}

func applyMarks() {
	gt2.ClearMobileTints()
	gt2.ClearMobileOutlines()
	for id, mark := range marks {
		if mark.TintEnabled {
			c := mark.Tint
			gt2.SetMobileTint(id, c.R, c.G, c.B, c.A)
		}
		if mark.OutlineEnabled {
			c := mark.Outline
			gt2.SetMobileOutline(id, c.R, c.G, c.B, c.A)
		}
	}
	for name, mark := range namedMarks {
		if mark.TintEnabled {
			c := mark.Tint
			gt2.SetNamedMobileTint(name, c.R, c.G, c.B, c.A)
		}
		if mark.OutlineEnabled {
			c := mark.Outline
			gt2.SetNamedMobileOutline(name, c.R, c.G, c.B, c.A)
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

func packColor(mark markColor) uint32 {
	return uint32(mark.R)<<24 | uint32(mark.G)<<16 | uint32(mark.B)<<8 | uint32(mark.A)
}

func unpackColor(value uint32) markColor {
	return markColor{R: uint8(value >> 24), G: uint8(value >> 16), B: uint8(value >> 8), A: uint8(value)}
}
