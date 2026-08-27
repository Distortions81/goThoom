//go:build !test

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

var playersWin *eui.WindowData
var playersList *eui.ItemData
var playersDirty bool
var playersRowRefs = map[*eui.ItemData]string{}
var playersGroupHeaders = map[*eui.ItemData]bool{}
var nextRecentPlayersExpiry time.Time
var selectedPlayerName string
var lastPlayerClickName string
var lastPlayerClickTime time.Time

type playerListGroup int

const (
	playerGroupRecent playerListGroup = iota
	playerGroupOnline
	playerGroupOffline
)

const visiblePlayerGroupRadius = 180

func nearbyVisiblePlayerGroupKeys() []string {
	stateMu.Lock()
	defer stateMu.Unlock()
	self, ok := state.mobiles[playerIndex]
	if !ok {
		return nil
	}
	radiusSquared := visiblePlayerGroupRadius * visiblePlayerGroupRadius
	keys := make([]string, 0)
	for index, mobile := range state.mobiles {
		if index == playerIndex || mobile.Persist {
			continue
		}
		desc, ok := state.descriptors[index]
		if !ok || desc.Type == kDescNPC || desc.Name == "" || !mobileActuallyVisible(mobile, desc) {
			continue
		}
		dx := int(mobile.H) - int(self.H)
		dy := int(mobile.V) - int(self.V)
		if dx*dx+dy*dy <= radiusSquared {
			keys = append(keys, playerCustomGroupKey(desc.Name))
		}
	}
	sort.Strings(keys)
	return keys
}

func playerGroup(p Player, now time.Time, showRecent bool) playerListGroup {
	if p.Offline {
		return playerGroupOffline
	}
	if showRecent && !p.LastOnScreen.IsZero() {
		age := now.Sub(p.LastOnScreen)
		if age >= 0 && age < recentPlayerWindow {
			return playerGroupRecent
		}
	}
	return playerGroupOnline
}

func playerGroupTitle(group playerListGroup) string {
	switch group {
	case playerGroupRecent:
		return "Recently On Screen"
	case playerGroupOnline:
		return "Online"
	default:
		return "Offline"
	}
}

func playerDisplayGroup(p Player, now time.Time) string {
	if group := gs.PlayerGroups.group(playerCustomGroupKey(p.Name)); group != "" {
		return "custom:" + group
	}
	return fmt.Sprintf("auto:%d", playerGroup(p, now, gs.ShowRecentPlayers))
}

func playerDisplayGroupTitle(group string) string {
	if strings.HasPrefix(group, "custom:") {
		return strings.TrimPrefix(group, "custom:")
	}
	var automatic int
	fmt.Sscanf(group, "auto:%d", &automatic)
	return playerGroupTitle(playerListGroup(automatic))
}

func playerDisplayGroupOrder(group string) int {
	if strings.HasPrefix(group, "custom:") {
		name := strings.TrimPrefix(group, "custom:")
		for i, candidate := range gs.PlayerGroups.Names {
			if strings.EqualFold(candidate, name) {
				return i
			}
		}
	}
	var automatic int
	fmt.Sscanf(group, "auto:%d", &automatic)
	return len(gs.PlayerGroups.Names) + automatic
}

//go:embed data/icons/share-out.png
var shareOutPNG []byte

//go:embed data/icons/share-both.png
var shareBothPNG []byte

var (
	shareIconOnce sync.Once
	shareOutIcon  *ebiten.Image
	shareInIcon   *ebiten.Image
	shareBothIcon *ebiten.Image
)

// playerSharingIcons builds the small sharing icons once. The incoming icon is
// a mirror of the outgoing icon; mutual sharing has its own filled-person icon.
func playerSharingIcons() (outgoing, incoming, mutual *ebiten.Image) {
	shareIconOnce.Do(func() {
		if src, err := png.Decode(bytes.NewReader(shareOutPNG)); err == nil {
			shareOutIcon = newManagedImageFromImage(src)
			shareInIcon = mirrorManagedImage(shareOutIcon)
		} else {
			logError("decode outgoing sharing icon: %v", err)
		}
		if src, err := png.Decode(bytes.NewReader(shareBothPNG)); err == nil {
			shareBothIcon = newManagedImageFromImage(src)
		} else {
			logError("decode mutual sharing icon: %v", err)
		}
	})
	return shareOutIcon, shareInIcon, shareBothIcon
}

func playerSharingIcon(p Player) *ebiten.Image {
	outgoing, incoming, mutual := playerSharingIcons()
	switch {
	case p.Sharee && p.Sharing:
		return mutual
	case p.Sharee && !p.Sharing:
		return incoming
	case p.Sharing && !p.Sharee:
		return outgoing
	default:
		return nil
	}
}

func playersWindowTitle(online, sharedTo, sharingToUs int) string {
	return fmt.Sprintf("Players   Online: %d   Shared: %d   Sharing: %d", online, sharedTo, sharingToUs)
}

// searchPlayersWindow highlights player rows whose names contain query and
// adds matching-row markers to the scrollbar.
func searchPlayersWindow(query string) {
	if playersList == nil {
		if playersWin != nil {
			playersWin.Refresh()
		}
		return
	}

	q := strings.ToLower(query)
	total := len(playersList.Contents)
	marks := make([]float32, 0)
	accent := eui.AccentColor()
	playerIndex := 0
	for i, row := range playersList.Contents {
		if playersGroupHeaders[row] {
			row.Focused = false
			continue
		}
		row.Focused = false
		name := playersRowRefs[row]
		alternate := playerIndex%2 == 1
		playerIndex++
		if q != "" && strings.Contains(strings.ToLower(name), q) {
			row.Filled = true
			row.Color = accent
			if total > 0 {
				marks = append(marks, float32(i)/float32(total))
			}
			continue
		}
		row.Filled = gs.AlternateRowBackgrounds && alternate
		if row.Filled {
			row.Color = alternateRowColor()
		} else {
			row.Color = eui.Color{}
		}
	}
	playersList.ScrollMarks = marks
	if playersWin != nil {
		playersWin.Refresh()
	}
}

func playerSharingIndicator(p Player) string {
	switch {
	case p.Sharee && p.Sharing:
		return "↔"
	case p.Sharee:
		return "→"
	case p.Sharing:
		return "←"
	default:
		return ""
	}
}

func playerSharingTooltip(p Player) string {
	switch {
	case p.Sharee && p.Sharing:
		return "You share with each other"
	case p.Sharee:
		return "You share to this player"
	case p.Sharing:
		return "This player shares to you"
	default:
		return ""
	}
}

// defaultMobilePictID returns a fallback CL_Images mobile pict ID for the
// given gender when a player's specific PictID is unknown. Values are chosen
// to match classic client defaults (peasant male/female). For neutral/other,
// we fall back to the male peasant.
func defaultMobilePictID(g genderIcon) uint16 {
	switch g {
	case genderMale:
		return 447
	case genderFemale:
		return 456
	default:
		return 22
	}
}

func updatePlayersWindow() {
	if playersWin == nil || playersList == nil {
		return
	}

	accent := eui.AccentColor()

	prevScroll := playersList.Scroll

	// Gather current players and filter to non-NPCs with names.
	ps := getPlayers()
	now := time.Now()
	// Sort by section, then by label/color group and name.
	sort.Slice(ps, func(i, j int) bool {
		groupI := playerDisplayGroup(ps[i], now)
		groupJ := playerDisplayGroup(ps[j], now)
		if groupI != groupJ {
			return playerDisplayGroupOrder(groupI) < playerDisplayGroupOrder(groupJ)
		}
		// Same online/offline status: sort by label group.
		li := ps[i].FriendLabel
		lj := ps[j].FriendLabel
		// Treat unlabeled (0) as after labeled groups.
		if li == 0 && lj != 0 {
			return false
		}
		if lj == 0 && li != 0 {
			return true
		}
		if li != lj {
			return li < lj
		}
		// Final tie-breaker: by name.
		return ps[i].Name < ps[j].Name
	})
	exiles := make([]Player, 0, len(ps))
	groupCounts := make(map[string]int)
	shareCount, shareeCount := 0, 0
	onlineCount := 0
	for _, p := range ps {
		if p.Name == "" || p.IsNPC {
			continue
		}
		if p.Sharing {
			shareCount++
		}
		if p.Sharee {
			shareeCount++
		}
		exiles = append(exiles, p)
		groupCounts[playerDisplayGroup(p, now)]++
		if !p.Offline {
			onlineCount++
		}
	}
	playersWin.Title = playersWindowTitle(onlineCount, shareeCount, shareCount)

	myClan := ""
	if playerName != "" {
		playersMu.RLock()
		if me, ok := players[playerName]; ok {
			myClan = me.clan
		}
		playersMu.RUnlock()
	}

	// Compute client area for sizing children similar to updateTextWindow.
	clientW := playersWin.GetSize().X
	clientH := playersWin.GetSize().Y - playersWin.GetTitleSize()
	s := eui.UIScale()
	if playersWin.NoScale {
		s = 1
	}
	pad := (playersWin.Padding + playersWin.BorderPad) * s
	clientWAvail := clientW - 2*pad
	if clientWAvail < 0 {
		clientWAvail = 0
	}
	clientHAvail := clientH - 2*pad
	if clientHAvail < 0 {
		clientHAvail = 0
	}

	// Determine row height from font metrics (ascent+descent).
	fontSize := gs.PlayersFontSize
	if fontSize <= 0 {
		fontSize = gs.ConsoleFontSize
	}
	ui := eui.UIScale()
	facePx := float64(float32(fontSize) * ui)
	var goFace *text.GoTextFace
	if src := eui.FontSource(); src != nil {
		goFace = &text.GoTextFace{Source: src, Size: facePx}
	} else {
		goFace = &text.GoTextFace{Size: facePx}
	}
	metrics := goFace.Metrics()
	linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2) // +2 px padding
	rowUnits := float32(linePx) / ui

	// Rebuild contents: one row per player.
	// Layout per row: [avatar (or default/blank)] [profession (or blank)] [name]
	playersList.Contents = nil
	playersRowRefs = map[*eui.ItemData]string{}
	playersGroupHeaders = map[*eui.ItemData]bool{}
	nextRecentPlayersExpiry = time.Time{}
	var selectedRow *eui.ItemData

	rowIndex := 0
	lastGroup := ""
	for _, customGroup := range gs.PlayerGroups.Names {
		if groupCounts["custom:"+customGroup] != 0 {
			continue
		}
		header := makePlayerGroupHeader(customGroup, 0, clientWAvail, rowUnits, fontSize, true)
		playersList.AddItem(header)
		playersGroupHeaders[header] = true
	}
	for _, p := range exiles {
		group := playerDisplayGroup(p, now)
		if group != lastGroup {
			custom := strings.HasPrefix(group, "custom:")
			header := makePlayerGroupHeader(playerDisplayGroupTitle(group), groupCounts[group], clientWAvail, rowUnits, fontSize, custom)
			playersList.AddItem(header)
			playersGroupHeaders[header] = true
			lastGroup = group
		}
		if group == fmt.Sprintf("auto:%d", playerGroupRecent) {
			expires := p.LastOnScreen.Add(recentPlayerWindow)
			if nextRecentPlayersExpiry.IsZero() || expires.Before(nextRecentPlayersExpiry) {
				nextRecentPlayersExpiry = expires
			}
		}
		offline := p.Offline
		name := p.Name
		if sameRealClan(p.clan, myClan) {
			name += " *"
		}

		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true, Filled: gs.AlternateRowBackgrounds && rowIndex%2 == 1, Color: alternateRowColor()}

		if p.FriendLabel > 0 {
			row.Outlined = true
			row.Border = 3
			row.OutlineColor = labelColor(p.FriendLabel)
		}

		// Track selected row for highlight after search.
		if p.Name == selectedPlayerName {
			selectedRow = row
		}

		iconSize := int(rowUnits + 0.5)

		{
			profItem, _ := eui.NewImageItem(iconSize, iconSize)
			profItem.Margin = 4
			profItem.Border = 0
			profItem.Filled = false
			profItem.Disabled = offline
			if pid := professionPictID(p.Class); pid != 0 {
				if img := loadImage(pid); img != nil {
					profItem.Image = img
					profItem.ImageName = "prof:cl:" + fmt.Sprint(pid)
				}
			}
			// Click selects this player.
			n := p.Name
			profItem.Action = func() { handlePlayersClick(n) }
			row.AddItem(profItem)
		}

		{
			avItem, _ := eui.NewImageItem(iconSize, iconSize)
			avItem.Margin = 4
			avItem.Border = 0
			avItem.Filled = false
			avItem.Disabled = offline
			var img *ebiten.Image
			state := uint8(0)
			if p.Dead && !strings.EqualFold(p.Name, playerName) {
				state = 32
			}
			if p.PictID != 0 {
				if m := loadMobileFrame(p.PictID, state, p.Colors); m != nil {
					img = m
				} else if im := loadImage(p.PictID); im != nil {
					img = im
				}
			}
			if img == nil {
				gid := defaultMobilePictID(genderFromString(p.Gender))
				if gid != 0 {
					if m := loadMobileFrame(gid, state, nil); m != nil {
						img = m
					} else if im := loadImage(gid); im != nil {
						img = im
					}
				}
			}
			if img != nil {
				avItem.Image = img
			}
			// Always add avatar slot, even if blank, to keep alignment.
			// Click selects this player.
			n := p.Name
			avItem.Action = func() { handlePlayersClick(n) }
			row.AddItem(avItem)
		}

		t, _ := eui.NewText()
		t.Text = name
		t.FontSize = float32(fontSize)
		face := mainFont
		if p.Sharing && p.Sharee {
			face = mainFontBoldItalic
		} else if p.Sharing {
			face = mainFontBold
		} else if p.Sharee {
			face = mainFontItalic
		}
		t.Face = face
		if (p.Dead && !strings.EqualFold(p.Name, playerName)) || offline {
			t.TextColor = eui.ColorVeryDarkGray
			t.ForceTextColor = true
		}
		indicator := playerSharingIndicator(p)
		shareIcon := playerSharingIcon(p)
		indicatorWidth := float32(0)
		if shareIcon != nil {
			bounds := shareIcon.Bounds()
			indicatorWidth = rowUnits*float32(bounds.Dx())/float32(bounds.Dy()) + 4
		} else if indicator != "" {
			if w, _ := text.Measure("↔", face, 0); w > 0 {
				indicatorWidth = float32(math.Ceil(w/float64(ui))) + 8
			}
		}
		t.Size = eui.Point{X: clientWAvail - float32(iconSize*2) - 8 - indicatorWidth, Y: rowUnits}
		// Click selects this player.
		n := p.Name
		t.Action = func() { handlePlayersClick(n) }
		row.AddItem(t)

		if indicatorWidth > 0 {
			if shareIcon != nil {
				shareItem, _ := eui.NewImageItem(int(math.Ceil(float64(indicatorWidth))), int(math.Ceil(float64(rowUnits))))
				shareItem.Image = shareIcon
				shareItem.Filled = false
				shareItem.Border = 0
				shareItem.Disabled = offline
				shareItem.SetTooltip(playerSharingTooltip(p))
				shareItem.Action = func() { handlePlayersClick(n) }
				row.AddItem(shareItem)
			} else {
				shareItem, _ := eui.NewText()
				shareItem.Text = indicator
				shareItem.FontSize = float32(fontSize)
				shareItem.Face = face
				shareItem.Size = eui.Point{X: indicatorWidth, Y: rowUnits}
				shareItem.SetTooltip(playerSharingTooltip(p))
				shareItem.Action = func() { handlePlayersClick(n) }
				row.AddItem(shareItem)
			}
		}

		// Also allow clicking the row background to select.
		n = p.Name
		row.Action = func() { handlePlayersClick(n) }

		row.Size.Y = rowUnits
		playersList.AddItem(row)
		playersRowRefs[row] = p.Name
		rowIndex++
	}

	// Size the list below any docked toolbar rows.
	sizeTextWindowList(playersList, clientWAvail, clientHAvail)
	playersList.Scroll = prevScroll
	searchPlayersWindow(playersWin.SearchText)
	if selectedRow != nil {
		selectedRow.Filled = true
		selectedRow.Color = accent
	}
	playersWin.Refresh()
}

func makePlayerGroupHeader(group string, count int, width, rowUnits float32, fontSize float64, editable bool) *eui.ItemData {
	header := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	contentWidth := max(0, width-eui.ScrollbarWidth())
	header.Size = eui.Point{X: contentWidth, Y: rowUnits + 4}
	label, _ := eui.NewText()
	label.Text = fmt.Sprintf("%s (%d)", group, count)
	label.FontSize = float32(fontSize)
	label.Face = mainFontBold
	label.TextColor = eui.AccentColor()
	label.ForceTextColor = true
	label.Size = eui.Point{X: max(0, contentWidth-label.Position.X), Y: rowUnits + 4}
	header.AddItem(label)
	if editable {
		button, events := eui.NewButton()
		button.Text = "Edit"
		button.Size = eui.Point{X: 52, Y: rowUnits + 2}
		label.Size.X = max(0, contentWidth-label.Position.X-button.Position.X-button.Size.X)
		name := group
		events.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventClick {
				showEditPlayerGroupWindow(name)
			}
		}
		header.AddItem(button)
	}
	return header
}

// handlePlayersContextClick opens a context menu for the player row under the
// mouse, mirroring the inventory menu behavior. Returns true if a menu opened.
func handlePlayersContextClick(mx, my int) bool {
	if playersWin == nil || playersList == nil || !playersWin.IsOpen() {
		return false
	}
	pos := eui.Point{X: float32(mx), Y: float32(my)}
	for _, row := range playersList.Contents {
		r := row.DrawRect
		if pos.X >= r.X0 && pos.X <= r.X1 && pos.Y >= r.Y0 && pos.Y <= r.Y1 {
			if name, ok := playersRowRefs[row]; ok {
				// Select the player before opening the context menu
				selectPlayer(name)
				openPlayersContextMenu(name, pos)
				return true
			}
		}
	}
	return false
}

// handlePlayersClick selects a player on single-click. If we later add
// double-click behavior, we can use lastPlayerClick* similar to inventory.
func handlePlayersClick(name string) {
	event := legacyMacroPlayerClickEvent(name)
	if started, allowDefault := legacyMacroTriggerClick(event, int64(ackFrame)); started && !allowDefault {
		legacyMacroMarkInputConsumed("click")
		return
	}
	if legacyMacroHandlePlayerModifierClick(name, event.Modifiers) {
		return
	}
	now := time.Now()
	if name == lastPlayerClickName && now.Sub(lastPlayerClickTime) < 500*time.Millisecond {
		// Reserved for double-click behavior in the future.
		lastPlayerClickTime = time.Time{}
		return
	}
	selectPlayer(name)
	lastPlayerClickName = name
	lastPlayerClickTime = now
}

func selectPlayer(name string) {
	if selectedPlayerName == name {
		return
	}
	selectedPlayerName = name
	updatePlayersWindow()
}

func openPlayersContextMenu(name string, pos eui.Point) {
	// Close any existing context menus.
	eui.CloseContextMenus()

	displayName := name
	options := []string{}
	actions := []func(){}

	// If the player has a label color/group, show that as a disabled header
	// line at the top of the menu. Otherwise, fall back to showing the
	// player's name as the header.
	headerCount := 0
	if displayName != "" {
		if p := getPlayer(displayName); p != nil && p.FriendLabel > 0 {
			idx := p.FriendLabel
			colorName := ""
			if idx > 0 && idx <= len(defaultLabelNames) {
				colorName = defaultLabelNames[idx-1]
			}
			groupName := labelName(idx)
			header := ""
			if colorName != "" && groupName != "" && !strings.EqualFold(colorName, groupName) {
				header = fmt.Sprintf("%s — %s", colorName, groupName)
			} else if groupName != "" {
				header = groupName
			} else if colorName != "" {
				header = colorName
			}
			if header != "" {
				options = append(options, header)
				headerCount = 1
			}
		}
		if headerCount == 0 {
			options = append(options, displayName)
			headerCount = 1
		}
	}

	// Thank: immediate thank.
	if displayName != "" {
		options = append(options, "Thank")
		n := displayName
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/thank %s", maybeQuoteName(n)))
			nextCommand()
		})
	}

	// Curse: immediate curse directed at this player.
	if displayName != "" {
		options = append(options, "Curse")
		n := displayName
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/curse %s", maybeQuoteName(n)))
			nextCommand()
		})
	}

	// Anon Thank / Anon Curse: prefill so user can type a message.
	options = append(options, "Anon Thank…")
	actions = append(actions, func() {
		n := displayName
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/anonthank %s", maybeQuoteName(n)))
			nextCommand()
		})
	})
	options = append(options, "Anon Curse…")
	actions = append(actions, func() {
		n := displayName
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/anoncurse %s", maybeQuoteName(n)))
			nextCommand()
		})
	})

	// Share / Unshare with this player.
	if displayName != "" {
		options = append(options, "Share")
		n := displayName
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/share %s", maybeQuoteName(n)))
			nextCommand()
		})
		options = append(options, "Unshare")
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/unshare %s", maybeQuoteName(n)))
			nextCommand()
		})
	}

	// Info on this player.
	if displayName != "" {
		options = append(options, "Info")
		n := displayName
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/info %s", maybeQuoteName(n)))
			nextCommand()
		})
	}

	// Pull / Push this player.
	if displayName != "" {
		options = append(options, "Pull")
		n := displayName
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/pull %s", maybeQuoteName(n)))
			nextCommand()
		})
		options = append(options, "Push")
		actions = append(actions, func() {
			enqueueCommand(fmt.Sprintf("/push %s", maybeQuoteName(n)))
			nextCommand()
		})
	}

	if displayName != "" {
		options = append(options, "Add to group…")
		n := displayName
		actions = append(actions, func() {
			showCustomGroupPicker(&gs.PlayerGroups, playerCustomGroupKey(n), "Player", pos, func() { playersDirty = true })
		})
		options = append(options, "Label")
		actions = append(actions, func() { showLabelMenu(n, pos, false) })
		options = append(options, "Label (Global)")
		actions = append(actions, func() { showLabelMenu(n, pos, true) })
	}

	if len(options) == 0 {
		return
	}
	menu := eui.ShowContextMenu(options, pos.X, pos.Y, func(i int) {
		adj := i - headerCount
		if adj >= 0 && adj < len(actions) {
			actions[adj]()
		}
	})
	if menu != nil && headerCount > 0 {
		menu.HeaderCount = headerCount
	}
}
