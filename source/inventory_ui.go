//go:build !test

package main

import (
	"bytes"
	"fmt"
	"gothoom/eui"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var inventoryWin *eui.WindowData
var inventoryList *eui.ItemData
var inventoryDirty bool

const inventoryMaxSlots = 32

func inventoryWindowTitle(used int) string {
	free := inventoryMaxSlots - used
	if free <= 0 {
		return fmt.Sprintf("Inventory   Slots: %d/%d (pack full)", used, inventoryMaxSlots)
	}
	if free <= 5 {
		return fmt.Sprintf("Inventory   Slots: %d/%d (%d free)", used, inventoryMaxSlots, free)
	}
	return fmt.Sprintf("Inventory   Slots: %d/%d", used, inventoryMaxSlots)
}

type invRef struct {
	id     uint16
	idx    int
	global int
}

var inventoryRowRefs = map[*eui.ItemData]invRef{}

type inventoryRow struct {
	key    invGroupKey
	row    *eui.ItemData
	icon   *eui.ItemData
	label  *eui.ItemData
	slot   *eui.ItemData
	id     uint16
	idx    int
	global int
	action func()
}

type inventoryRenderState struct {
	rows         map[invGroupKey]*inventoryRow
	headers      map[string]*eui.ItemData
	order        []invGroupKey
	spacer       *eui.ItemData
	fontSize     int
	uiScale      float32
	fontSource   *text.GoTextFaceSource
	facePx       float64
	baseFace     *text.GoTextFace
	boldFace     *text.GoTextFace
	italicFace   *text.GoTextFace
	rowUnits     float32
	rowPx        float32
	iconSize     int
	clientWAvail float32
	clientHAvail float32
}

var invRender inventoryRenderState

type inventoryRowData struct {
	rowIndex  int
	key       invGroupKey
	id        uint16
	idx       int
	global    int
	label     string
	face      *text.GoTextFace
	slotText  string
	slotWidth float32
	icon      *ebiten.Image
	iconName  string
	group     string
	equipped  bool
}

var selectedInvID uint16
var selectedInvIdx int = -1
var lastInvClickID uint16
var lastInvClickIdx int
var lastInvClickTime time.Time

var TitleCaser = cases.Title(language.AmericanEnglish)
var foldCaser = cases.Fold()

var (
	invRegularSrc *text.GoTextFaceSource
	invBoldSrc    *text.GoTextFaceSource
	invItalicSrc  *text.GoTextFaceSource
)

type invGroupKey struct {
	id   uint16
	name string
	idx  int
}

// slotNames maps item slot constants to display strings.
var slotNames = []string{
	"invalid", // kItemSlotNotInventory
	"unknown", // kItemSlotNotWearable
	"forehead",
	"neck",
	"shoulder",
	"arms",
	"gloves",
	"finger",
	"coat",
	"cloak",
	"torso",
	"waist",
	"legs",
	"feet",
	"right",
	"left",
	"hands",
	"head",
}

func makeInventoryWindow() {
	if inventoryWin != nil {
		return
	}
	inventoryWin, inventoryList, _ = makeTextWindow("Inventory", eui.HZoneLeft, eui.VZoneMiddleTop, true)
	inventoryWin.Searchable = true
	inventoryWin.OnSearch = searchInventoryWindow
	inventoryWin.OnOpen = updateInventoryWindow
	// Ensure layout updates immediately on resize to avoid gaps.
	inventoryWin.OnResize = func() { updateInventoryWindow() }
	updateInventoryWindow()
}

func updateInventoryWindow() {
	if inventoryWin == nil || inventoryList == nil || !inventoryWin.IsOpen() {
		return
	}

	ensureInventoryFontSources()

	prevScroll := inventoryList.Scroll

	items := getInventory()
	usedSlots := 0
	counts := make(map[invGroupKey]int)
	first := make(map[invGroupKey]InventoryItem)
	anyEquipped := make(map[invGroupKey]bool)
	order := make([]invGroupKey, 0, len(items))
	for _, it := range items {
		usedSlots += it.Quantity
		key := invGroupKey{id: it.ID, name: it.Name}
		if it.IDIndex >= 0 {
			key.idx = it.IDIndex
			key.name = ""
		}
		if _, seen := counts[key]; !seen {
			order = append(order, key)
			first[key] = it
		}
		counts[key] += it.Quantity
		if it.Equipped {
			anyEquipped[key] = true
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		ai := order[i]
		aj := order[j]
		groupI := inventoryDisplayGroup(ai.id)
		groupJ := inventoryDisplayGroup(aj.id)
		if groupI != groupJ {
			return inventoryDisplayGroupOrder(groupI) < inventoryDisplayGroupOrder(groupJ)
		}
		nameI := officialName(ai, first[ai])
		nameJ := officialName(aj, first[aj])
		if nameI != nameJ {
			return nameI < nameJ
		}
		return first[ai].Index < first[aj].Index
	})

	fontSize := gs.InventoryFontSize
	if fontSize <= 0 {
		fontSize = gs.ConsoleFontSize
	}
	uiScale := eui.UIScale()

	src := eui.FontSource()
	geometryChanged := invRender.ensureFaces(int(math.Round(fontSize)), uiScale, src)

	clientW := inventoryWin.GetSize().X
	clientH := inventoryWin.GetSize().Y - inventoryWin.GetTitleSize()
	scale := eui.UIScale()
	if inventoryWin.NoScale {
		scale = 1
	}
	pad := (inventoryWin.Padding + inventoryWin.BorderPad) * scale
	clientWAvail := clientW - 2*pad
	if clientWAvail < 0 {
		clientWAvail = 0
	}
	clientHAvail := clientH - 2*pad
	if clientHAvail < 0 {
		clientHAvail = 0
	}
	invRender.clientWAvail = clientWAvail
	invRender.clientHAvail = clientHAvail

	rows := make([]inventoryRowData, 0, len(order))
	for rowIndex, key := range order {
		row := invRender.makeRowData(key, first[key], counts[key], anyEquipped[key])
		row.rowIndex = rowIndex
		rows = append(rows, row)
	}
	inventoryWin.Title = inventoryWindowTitle(usedSlots)

	if geometryChanged {
		invRender.rebuild(rows)
	} else {
		invRender.update(rows)
	}

	if inventoryWin != nil {
		sizeTextWindowList(inventoryList, clientWAvail, clientHAvail)
		inventoryList.Scroll = prevScroll
		searchInventoryWindow(inventoryWin.SearchText)
	}
}

// searchInventoryWindow highlights matching item rows and adds markers to the
// scrollbar. Selection remains highlighted even when it does not match.
func searchInventoryWindow(query string) {
	if inventoryList == nil {
		if inventoryWin != nil {
			inventoryWin.Refresh()
		}
		return
	}

	q := strings.ToLower(query)
	marks := make([]float32, 0)
	total := len(invRender.order)
	if q != "" && total > 0 {
		for i, key := range invRender.order {
			if row := invRender.rows[key]; row != nil && row.label != nil && strings.Contains(strings.ToLower(row.label.Text), q) {
				marks = append(marks, float32(i)/float32(total))
			}
		}
	}
	inventoryList.ScrollMarks = marks
	invRender.applySelection(eui.AccentColor())
	if inventoryWin != nil {
		inventoryWin.Refresh()
	}
}

func ensureInventoryFontSources() {
	if invRegularSrc == nil {
		invRegularSrc, _ = text.NewGoTextFaceSource(bytes.NewReader(notoSansRegular))
	}
	if invBoldSrc == nil {
		invBoldSrc, _ = text.NewGoTextFaceSource(bytes.NewReader(notoSansBold))
	}
	if invItalicSrc == nil {
		invItalicSrc, _ = text.NewGoTextFaceSource(bytes.NewReader(notoSansItalic))
	}
}

func (s *inventoryRenderState) ensureFaces(fontSize int, uiScale float32, src *text.GoTextFaceSource) bool {
	if src == nil {
		src = invRegularSrc
	}
	facePx := float64(float32(fontSize)*uiScale) + 2
	changed := s.baseFace == nil || s.fontSize != fontSize || s.uiScale != uiScale || s.fontSource != src || s.facePx != facePx
	if changed {
		s.baseFace = &text.GoTextFace{Source: src, Size: facePx}
		if invBoldSrc != nil {
			s.boldFace = &text.GoTextFace{Source: invBoldSrc, Size: facePx}
		} else {
			s.boldFace = nil
		}
		if invItalicSrc != nil {
			s.italicFace = &text.GoTextFace{Source: invItalicSrc, Size: facePx}
		} else {
			s.italicFace = nil
		}
		metrics := s.baseFace.Metrics()
		s.rowPx = float32(math.Ceil(metrics.HAscent + metrics.HDescent))
		s.rowUnits = s.rowPx / uiScale
		s.iconSize = int(s.rowUnits + 0.5)
		if s.iconSize < 1 {
			s.iconSize = 1
		}
	}
	s.fontSize = fontSize
	s.uiScale = uiScale
	s.fontSource = src
	s.facePx = facePx
	return changed
}

func (s *inventoryRenderState) makeRowData(key invGroupKey, it InventoryItem, qty int, equipped bool) inventoryRowData {
	id := key.id
	raw := it.Name
	if raw == "" && clImages != nil {
		raw = clImages.ItemName(uint32(id))
	}
	if raw == "" {
		raw = fmt.Sprintf("Item %d", id)
	}
	base := raw
	suffix := ""
	if p := strings.Index(base, " <"); p >= 0 {
		suffix = base[p:]
		base = base[:p]
	}
	qtySuffix := ""
	if qty > 1 {
		qtySuffix = fmt.Sprintf(" (%v)", qty)
	}
	label := TitleCaser.String(base) + suffix + qtySuffix

	var icon *ebiten.Image
	iconName := ""
	slot := -1
	if clImages != nil {
		if p := clImages.ItemWornPict(uint32(id)); p != 0 {
			if img := loadImage(uint16(p)); img != nil {
				icon = img
				iconName = fmt.Sprintf("item:%d", id)
			}
		}
		slot = clImages.ItemSlot(uint32(id))
	}

	face := s.baseFace
	if equipped {
		switch slot {
		case kItemSlotRightHand, kItemSlotLeftHand, kItemSlotBothHands:
			if s.boldFace != nil {
				face = s.boldFace
			}
		default:
			if s.italicFace != nil {
				face = s.italicFace
			}
		}
	}

	slotText := ""
	slotWidth := float32(0)
	if equipped && slot >= kItemSlotFirstReal && slot <= kItemSlotLastReal {
		slotText = fmt.Sprintf("[%v]", TitleCaser.String(slotNames[slot]))
		if face != nil {
			if w, _ := text.Measure(slotText, face, 0); w > 0 {
				slotWidth = float32(math.Ceil(w / float64(s.uiScale)))
			}
		}
	}

	idx := it.IDIndex
	if qty > 1 {
		idx = -1
	}

	return inventoryRowData{
		key:       key,
		id:        id,
		idx:       idx,
		global:    it.Index,
		label:     label,
		face:      face,
		slotText:  slotText,
		slotWidth: slotWidth,
		icon:      icon,
		iconName:  iconName,
		group:     inventoryDisplayGroup(id),
		equipped:  equipped,
	}
}

func inventoryDisplayGroup(id uint16) string {
	if group := gs.InventoryGroups.group(inventoryCustomGroupKey(id)); group != "" {
		return group
	}
	return ""
}

func inventoryDisplayGroupOrder(group string) int {
	for i, name := range gs.InventoryGroups.Names {
		if strings.EqualFold(name, group) {
			return i
		}
	}
	return len(gs.InventoryGroups.Names)
}

func (s *inventoryRenderState) ensureSpacer() {
	if s.spacer == nil {
		spacer, _ := eui.NewText()
		spacer.Text = ""
		spacer.Fixed = true
		s.spacer = spacer
	}
	s.spacer.Size = eui.Point{X: 1, Y: s.rowUnits}
	s.spacer.Position = eui.Point{}
	s.spacer.FontSize = float32(s.fontSize)
}

func (s *inventoryRenderState) createRow(data inventoryRowData) *inventoryRow {
	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	icon, backing := eui.NewImageItem(s.iconSize, s.iconSize)
	if data.icon != nil {
		backing.Deallocate()
		icon.Image = nil
	}
	icon.Filled = false
	icon.Border = 0
	icon.Margin = 4
	row.AddItem(icon)
	label, _ := eui.NewText()
	label.FontSize = float32(s.fontSize)
	row.AddItem(label)
	res := &inventoryRow{key: data.key, row: row, icon: icon, label: label}
	res.action = func() { handleInventoryClick(res.id, res.idx) }
	icon.Action = res.action
	label.Action = res.action
	s.updateRow(res, data)
	return res
}

func (s *inventoryRenderState) updateRow(row *inventoryRow, data inventoryRowData) {
	if row == nil {
		return
	}
	row.key = data.key
	row.id = data.id
	row.idx = data.idx
	row.global = data.global

	if row.row != nil {
		row.row.Fixed = true
		row.row.Size.X = s.clientWAvail
		row.row.Size.Y = s.rowUnits
		row.row.Filled = gs.AlternateRowBackgrounds && data.rowIndex%2 == 1
		row.row.Color = alternateRowColor()
	}

	if row.icon != nil {
		imageChanged := row.icon.Image != data.icon || row.icon.ImageName != data.iconName
		if imageChanged {
			row.icon.UpdateImage(data.icon)
		}
		row.icon.Size = eui.Point{X: float32(s.iconSize), Y: float32(s.iconSize)}
		row.icon.Position = eui.Point{}
		row.icon.Margin = 4
		row.icon.Filled = false
		row.icon.Border = 0
		row.icon.ImageName = data.iconName
		if imageChanged {
			row.icon.Dirty = true
		}
	}

	if row.label != nil {
		row.label.FontSize = float32(s.fontSize)
		row.label.Face = data.face
		row.label.Position = eui.Point{X: row.icon.Margin, Y: 0}
		avail := s.clientWAvail - float32(s.iconSize) - row.icon.Margin - data.slotWidth - eui.ScrollbarWidth()
		if avail < 0 {
			avail = 0
		}
		row.label.Size.X = avail
		row.label.Size.Y = s.rowUnits
		row.label.UpdateText(data.label)
		if data.equipped {
			end := utf8.RuneCountInString(data.label)
			if len(row.label.Underlines) != 1 || row.label.Underlines[0].Start != 0 || row.label.Underlines[0].End != end {
				row.label.Underlines = []eui.TextSpan{{Start: 0, End: end}}
			}
		} else if len(row.label.Underlines) != 0 {
			row.label.Underlines = nil
		}
	}

	if data.slotText != "" {
		newSlot := false
		if row.slot == nil {
			lt, _ := eui.NewText()
			lt.Fixed = true
			row.slot = lt
			row.slot.Action = row.action
			newSlot = true
		}
		row.slot.FontSize = float32(s.fontSize)
		row.slot.Face = data.face
		row.slot.Size.X = data.slotWidth
		row.slot.Size.Y = s.rowUnits
		row.slot.Position = eui.Point{}
		row.slot.UpdateText(data.slotText)
		if newSlot {
			row.row.AddItem(row.slot)
		}
	} else if row.slot != nil {
		row.row.RemoveItem(row.slot)
		row.slot = nil
	}

}

func (s *inventoryRenderState) rebuild(data []inventoryRowData) {
	inventoryList.Contents = nil
	inventoryRowRefs = make(map[*eui.ItemData]invRef, len(data))
	s.rows = make(map[invGroupKey]*inventoryRow, len(data))
	s.headers = make(map[string]*eui.ItemData)
	s.order = make([]invGroupKey, 0, len(data))
	rowItems := make([]*inventoryRow, 0, len(data))
	for _, d := range data {
		row := s.createRow(d)
		s.rows[d.key] = row
		s.order = append(s.order, d.key)
		rowItems = append(rowItems, row)
		inventoryRowRefs[row.row] = invRef{id: row.id, idx: row.idx, global: row.global}
	}
	s.ensureSpacer()
	for _, item := range s.groupedContents(data, rowItems) {
		inventoryList.AddItem(item)
	}
}

func (s *inventoryRenderState) update(data []inventoryRowData) {
	if s.rows == nil {
		s.rows = make(map[invGroupKey]*inventoryRow)
	}
	nextMap := make(map[invGroupKey]*inventoryRow, len(data))
	nextOrder := make([]invGroupKey, 0, len(data))
	rowItems := make([]*inventoryRow, 0, len(data))
	for _, d := range data {
		row := s.rows[d.key]
		if row == nil {
			row = s.createRow(d)
		} else {
			s.updateRow(row, d)
		}
		nextMap[d.key] = row
		nextOrder = append(nextOrder, d.key)
		rowItems = append(rowItems, row)
	}
	s.rows = nextMap
	s.order = nextOrder

	s.ensureSpacer()
	desired := s.groupedContents(data, rowItems)
	s.reconcileContents(desired)

	inventoryRowRefs = make(map[*eui.ItemData]invRef, len(rowItems))
	for _, row := range rowItems {
		inventoryRowRefs[row.row] = invRef{id: row.id, idx: row.idx, global: row.global}
	}
}

func (s *inventoryRenderState) groupHeader(group string, count int) *eui.ItemData {
	if s.headers == nil {
		s.headers = make(map[string]*eui.ItemData)
	}
	key := group
	if key == "" {
		key = "\x00"
	}
	header := s.headers[key]
	if header == nil {
		header = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		label, _ := eui.NewText()
		label.Face = mainFontBold
		label.TextColor = eui.AccentColor()
		label.ForceTextColor = true
		header.AddItem(label)
		if group != "" {
			button, events := eui.NewButton()
			button.Text = "Edit"
			setMaterialButtonIcon(button, "edit")
			name := group
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventClick {
					showEditInventoryGroupWindow(name)
				}
			}
			header.AddItem(button)
		}
		s.headers[key] = header
	}
	title := group
	if title == "" {
		title = "Ungrouped"
	}
	contentWidth := max(0, s.clientWAvail-eui.ScrollbarWidth())
	header.Size = eui.Point{X: contentWidth, Y: s.rowUnits + 4}
	if len(header.Contents) > 0 {
		label := header.Contents[0]
		label.UpdateText(fmt.Sprintf("%s (%d)", title, count))
		label.FontSize = float32(s.fontSize)
		label.Size = eui.Point{X: max(0, contentWidth-label.Position.X), Y: s.rowUnits + 4}
		if len(header.Contents) > 1 {
			button := header.Contents[1]
			button.Size = eui.Point{X: 52, Y: s.rowUnits + 2}
			label.Size.X = max(0, contentWidth-label.Position.X-button.Position.X-button.Size.X)
		}
	}
	return header
}

func (s *inventoryRenderState) groupedContents(data []inventoryRowData, rows []*inventoryRow) []*eui.ItemData {
	if len(gs.InventoryGroups.Names) == 0 {
		desired := make([]*eui.ItemData, 0, len(rows)+1)
		for _, row := range rows {
			desired = append(desired, row.row)
		}
		desired = append(desired, s.spacer)
		return desired
	}
	counts := make(map[string]int)
	for _, d := range data {
		counts[d.group]++
	}
	desired := make([]*eui.ItemData, 0, len(rows)+len(gs.InventoryGroups.Names)+2)
	for _, group := range gs.InventoryGroups.Names {
		header := s.groupHeader(group, counts[group])
		desired = append(desired, header)
		for i, d := range data {
			if strings.EqualFold(d.group, group) {
				desired = append(desired, rows[i].row)
			}
		}
	}
	if counts[""] > 0 {
		desired = append(desired, s.groupHeader("", counts[""]))
		for i, d := range data {
			if d.group == "" {
				desired = append(desired, rows[i].row)
			}
		}
	}
	desired = append(desired, s.spacer)
	return desired
}

func (s *inventoryRenderState) reconcileContents(desired []*eui.ItemData) {
	if len(inventoryList.Contents) == len(desired) {
		same := true
		for i := range desired {
			if inventoryList.Contents[i] != desired[i] {
				same = false
				break
			}
		}
		if same {
			return
		}
	}
	current := append([]*eui.ItemData(nil), inventoryList.Contents...)
	i := 0
	for _, target := range desired {
		if i >= len(current) {
			inventoryList.InsertItem(i, target)
			current = append(current[:i], append([]*eui.ItemData{target}, current[i:]...)...)
			i++
			continue
		}
		if current[i] == target {
			i++
			continue
		}
		idx := -1
		for j := i + 1; j < len(current); j++ {
			if current[j] == target {
				idx = j
				break
			}
		}
		if idx >= 0 {
			item := current[idx]
			inventoryList.RemoveItem(item)
			current = append(current[:idx], current[idx+1:]...)
			inventoryList.InsertItem(i, item)
			current = append(current[:i], append([]*eui.ItemData{item}, current[i:]...)...)
			i++
			continue
		}
		inventoryList.ReplaceItem(i, target)
		current[i] = target
		i++
	}
	for len(current) > len(desired) {
		item := current[len(current)-1]
		inventoryList.RemoveItem(item)
		current = current[:len(current)-1]
	}
}

func (s *inventoryRenderState) applySelection(accent eui.Color) {
	query := ""
	if inventoryWin != nil {
		query = strings.ToLower(inventoryWin.SearchText)
	}
	for i, key := range s.order {
		row := s.rows[key]
		if row == nil || row.row == nil {
			continue
		}
		matchesSearch := query != "" && row.label != nil && strings.Contains(strings.ToLower(row.label.Text), query)
		if row.id == selectedInvID && row.idx == selectedInvIdx || matchesSearch {
			row.row.Filled = true
			row.row.Color = accent
		} else {
			row.row.Filled = gs.AlternateRowBackgrounds && i%2 == 1
			if row.row.Filled {
				row.row.Color = alternateRowColor()
			} else {
				row.row.Color = eui.Color{}
			}
		}
	}
}

func handleInventoryClick(id uint16, idx int) {
	now := time.Now()
	if id == lastInvClickID && idx == lastInvClickIdx && now.Sub(lastInvClickTime) < 500*time.Millisecond {
		if ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
			enqueueCommand(fmt.Sprintf("/useitem %d", id))
			nextCommand()
		} else {
			toggleInventoryEquipAt(id, idx)
		}
		lastInvClickTime = time.Time{}
	} else {
		selectInventoryItem(id, idx)
		lastInvClickID = id
		lastInvClickIdx = idx
		lastInvClickTime = now
	}
}

func selectInventoryItem(id uint16, idx int) {
	if id == selectedInvID && idx == selectedInvIdx {
		return
	}
	selectedInvID = id
	selectedInvIdx = idx
	serverIdx := idx
	if serverIdx < 0 {
		serverIdx = 0
	}
	enqueueCommand(fmt.Sprintf("\\BE-SELECT %d %d", id, serverIdx))
	nextCommand()
	updateInventoryWindow()
}

// handleInventoryContextClick opens the inventory context menu if the mouse
// position is over an inventory row. Returns true if a menu was opened.
func handleInventoryContextClick(mx, my int) bool {
	if inventoryWin == nil || inventoryList == nil || !inventoryWin.IsOpen() {
		return false
	}
	pos := eui.Point{X: float32(mx), Y: float32(my)}
	for _, row := range inventoryList.Contents {
		r := row.DrawRect
		if pos.X >= r.X0 && pos.X <= r.X1 && pos.Y >= r.Y0 && pos.Y <= r.Y1 {
			if ref, ok := inventoryRowRefs[row]; ok {
				// Also select the item to provide a visual cue and
				// ensure subsequent actions can rely on current selection.
				selectInventoryItem(ref.id, ref.idx)
				openInventoryContextMenu(ref, pos)
				return true
			}
		}
	}
	return false
}

func openInventoryContextMenu(ref invRef, pos eui.Point) {
	// Close any existing context menus so only one is visible at a time.
	eui.CloseContextMenus()
	// Minimal overlay menu using the new EUI context menus: Equip/Unequip
	wearable := false
	equipped := false
	slotVal := -1
	displayName := ""
	examineName := ""
	if it, ok := inventoryItemByIndex(ref.global); ok {
		equipped = it.Equipped
		if clImages != nil {
			slot := clImages.ItemSlot(uint32(it.ID))
			if slot >= kItemSlotFirstReal && slot <= kItemSlotLastReal {
				wearable = true
				slotVal = int(slot)
			}
		}
		// Build a display name similar to the inventory list formatting.
		raw := it.Name
		if raw == "" && clImages != nil {
			raw = clImages.ItemName(uint32(it.ID))
		}
		if raw == "" {
			raw = fmt.Sprintf("Item %d", it.ID)
		}
		base := raw
		suffix := ""
		if p := strings.Index(base, " <"); p >= 0 {
			suffix = base[p:]
			base = base[:p]
		}
		displayName = TitleCaser.String(base) + suffix
		// Emulate classic client: show worn slot for equipped items.
		if equipped && slotVal >= 0 && slotVal < len(slotNames) {
			displayName += " [" + TitleCaser.String(slotNames[slotVal]) + "]"
		}
		examineName = base
	}
	// First, the non-selectable header: item name.
	options := []string{}
	if displayName != "" {
		options = append(options, "Item: "+displayName+":")
	}
	actions := []func(){}
	if wearable && !equipped {
		options = append(options, "Equip")
		actions = append(actions, func() {
			queueEquipCommand(ref.id, ref.idx)
		})
	}
	if wearable && equipped {
		options = append(options, "Unequip")
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/unequip %d", ref.id))
			nextCommand()
		})
	}
	// Always offer Examine when we know the item's name.
	if examineName != "" {
		options = append(options, "Use")
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/useitem %d", ref.id))
			nextCommand()
		})
		options = append(options, "Examine")
		actions = append(actions, func() {
			// Ensure the item is selected before examining
			selectInventoryItem(ref.id, ref.idx)
			enqueueCommand("/examine")
			nextCommand()
		})
	}
	// Offer Show (announce item name).
	if examineName != "" {
		options = append(options, "Show")
		actions = append(actions, func() {
			enqueueCommand("/show")
			nextCommand()
		})
	}
	// Offer Drop options: plain and Mine-protected.
	options = append(options, "Add to group…")
	actions = append(actions, func() {
		showCustomGroupPicker(&gs.InventoryGroups, inventoryCustomGroupKey(ref.id), "Item", pos, func() {
			inventoryDirty = true
			updateInventoryWindow()
		})
	})
	options = append(options, "Drop")
	actions = append(actions, func() {
		enqueueCommand("/drop")
		nextCommand()
	})
	options = append(options, "Drop (Mine)")
	actions = append(actions, func() {
		enqueueCommand("/drop /mine")
		nextCommand()
	})
	if len(options) == 0 {
		return
	}
	menu := eui.ShowContextMenu(options, pos.X, pos.Y, func(i int) {
		// Adjust for header occupying first index (if present)
		adj := i
		if displayName != "" {
			adj = i - 1
		}
		if adj >= 0 && adj < len(actions) {
			actions[adj]()
		}
	})
	if menu != nil && displayName != "" {
		// Mark first row as a non-interactive header
		menu.HeaderCount = 1
	}
}

// maybeQuoteName wraps a name in quotes if it contains whitespace.
func maybeQuoteName(s string) string {
	if strings.IndexFunc(s, func(r rune) bool { return r == ' ' || r == '\t' }) >= 0 {
		return fmt.Sprintf("\"%s\"", s)
	}
	return s
}

func officialName(k invGroupKey, it InventoryItem) string {
	name := it.Name
	if name == "" && clImages != nil {
		name = clImages.ItemName(uint32(k.id))
	}
	if name == "" {
		name = fmt.Sprintf("Item %d", k.id)
	}
	return foldCaser.String(name)
}
