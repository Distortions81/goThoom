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
var renderedPlayerSelection string
var lastPlayerClickName string
var lastPlayerClickTime time.Time

type playerRowSignature struct {
	name              string
	displayName       string
	class             string
	gender            string
	colorsLen         uint8
	colors            [maxColors]byte
	pictID            uint16
	friendLabel       int
	offline           bool
	dead              bool
	local             bool
	sharee            bool
	sharing           bool
	sameClan          bool
	shareIcons        bool
	alternateRows     bool
	clientWidth       float32
	rowUnits          float32
	fontSize          float64
	uiScale           float32
	face              text.Face
	outlineColor      eui.Color
	alternateRowColor eui.Color
}

type cachedPlayerRow struct {
	row           *eui.ItemData
	profession    *eui.ItemData
	avatar        *eui.ItemData
	signature     playerRowSignature
	artworkLoaded bool
}

type playerHeaderSignature struct {
	title       string
	count       int
	width       float32
	rowUnits    float32
	fontSize    float64
	editable    bool
	accentColor eui.Color
	face        text.Face
}

type cachedPlayerHeader struct {
	row       *eui.ItemData
	signature playerHeaderSignature
}

var cachedPlayerRows = map[string]cachedPlayerRow{}
var cachedPlayerHeaders = map[string]cachedPlayerHeader{}

var playerArtworkViewport struct {
	scroll eui.Point
	size   eui.Point
	valid  bool
}

type playerListGroup int

const (
	playerGroupRecent playerListGroup = iota
	playerGroupOnline
	playerGroupOffline
)

const visiblePlayerGroupRadius = 180
const recentPlayerExpiryCheckInterval = 10 * time.Second

var lastRecentPlayerExpiryCheck time.Time

func shouldCheckRecentPlayerExpiry(now time.Time) bool {
	if !lastRecentPlayerExpiryCheck.IsZero() && now.Sub(lastRecentPlayerExpiryCheck) < recentPlayerExpiryCheckInterval && !now.Before(lastRecentPlayerExpiryCheck) {
		return false
	}
	lastRecentPlayerExpiryCheck = now
	return true
}

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
	if gs.GroupClanMembers && p.SameClan {
		return "clan"
	}
	return fmt.Sprintf("auto:%d", playerGroup(p, now, gs.ShowRecentPlayers))
}

func playerDisplayGroupTitle(group string) string {
	if strings.HasPrefix(group, "custom:") {
		return strings.TrimPrefix(group, "custom:")
	}
	if group == "clan" {
		return "Clan"
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
	if group == "clan" {
		return len(gs.PlayerGroups.Names)
	}
	var automatic int
	fmt.Sscanf(group, "auto:%d", &automatic)
	return len(gs.PlayerGroups.Names) + 1 + automatic
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
	case p.Sharee:
		return incoming
	case p.Sharing:
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
	applyPlayersSearch(query)
	if playersWin != nil {
		playersWin.Refresh()
	}
}

func applyPlayersSearch(query string) {
	if playersList == nil {
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

const playerShareIconRightMargin float32 = 8

func playerShareIndicatorReservation(contentWidth float32) float32 {
	if contentWidth <= 0 {
		return 0
	}
	return contentWidth + playerShareIconRightMargin
}

func playerListNameStyle(p Player) uint8 {
	style := styleRegular
	if p.Sharing {
		style |= styleBold
	}
	if p.SameClan {
		style |= styleItalic
	}
	if p.Sharee {
		style |= styleUnderline
	}
	return style
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

func playerRowFace(p Player) text.Face {
	style := playerListNameStyle(p)
	switch style & (styleBold | styleItalic) {
	case styleBoldItalic:
		return mainFontBoldItalic
	case styleBold:
		return mainFontBold
	case styleItalic:
		return mainFontItalic
	default:
		return mainFont
	}
}

func makePlayerRowSignature(p Player, myClan string, clientWidth, rowUnits float32, fontSize float64, ui float32) playerRowSignature {
	displayName := p.Name
	if sameRealClan(p.clan, myClan) {
		displayName += " *"
	}
	outline := eui.Color{}
	if p.FriendLabel > 0 {
		outline = labelColor(p.FriendLabel)
	}
	signature := playerRowSignature{
		name:              p.Name,
		displayName:       displayName,
		class:             p.Class,
		gender:            p.Gender,
		pictID:            p.PictID,
		friendLabel:       p.FriendLabel,
		offline:           p.Offline,
		dead:              p.Dead,
		local:             strings.EqualFold(p.Name, playerName),
		sharee:            p.Sharee,
		sharing:           p.Sharing,
		sameClan:          p.SameClan,
		shareIcons:        gs.PlayerShareIcons,
		alternateRows:     gs.AlternateRowBackgrounds,
		clientWidth:       clientWidth,
		rowUnits:          rowUnits,
		fontSize:          fontSize,
		uiScale:           ui,
		face:              playerRowFace(p),
		outlineColor:      outline,
		alternateRowColor: alternateRowColor(),
	}
	colorCount := min(len(p.Colors), len(signature.colors))
	signature.colorsLen = uint8(colorCount)
	copy(signature.colors[:], p.Colors[:colorCount])
	return signature
}

func makePlayerRow(p Player, signature playerRowSignature, rowIndex int) cachedPlayerRow {
	row := &eui.ItemData{
		ItemType: eui.ITEM_FLOW,
		FlowType: eui.FLOW_HORIZONTAL,
		Fixed:    true,
		Filled:   signature.alternateRows && rowIndex%2 == 1,
		Color:    signature.alternateRowColor,
	}
	if signature.friendLabel > 0 {
		row.Outlined = true
		row.Border = 3
		row.OutlineColor = signature.outlineColor
	}

	iconSize := int(signature.rowUnits + 0.5)
	profItem := eui.NewImageReferenceItem(iconSize, iconSize)
	profItem.Margin = 4
	profItem.Border = 0
	profItem.Filled = false
	profItem.Disabled = signature.offline
	name := signature.name
	profItem.Action = func() { handlePlayersClick(name) }
	row.AddItem(profItem)

	avItem := eui.NewImageReferenceItem(iconSize, iconSize)
	avItem.Margin = 4
	avItem.Border = 0
	avItem.Filled = false
	avItem.Disabled = signature.offline
	avItem.Action = func() { handlePlayersClick(name) }
	row.AddItem(avItem)

	nameItem, _ := eui.NewText()
	nameItem.Text = signature.displayName
	nameItem.FontSize = float32(signature.fontSize)
	nameItem.Face = signature.face
	if signature.sharee {
		nameItem.Underlines = []eui.TextSpan{{Start: 0, End: len([]rune(signature.name)), MatchTextColor: true}}
	}
	if (signature.dead && !signature.local) || signature.offline {
		nameItem.TextColor = eui.ColorVeryDarkGray
		nameItem.ForceTextColor = true
	}

	indicator := ""
	var shareIcon *ebiten.Image
	shareContentWidth := float32(0)
	if signature.shareIcons {
		indicator = playerSharingIndicator(p)
		shareIcon = playerSharingIcon(p)
		switch {
		case shareIcon != nil:
			bounds := shareIcon.Bounds()
			shareContentWidth = signature.rowUnits*float32(bounds.Dx())/float32(bounds.Dy()) + 4
		case indicator != "":
			if width, _ := text.Measure("↔", signature.face, 0); width > 0 {
				shareContentWidth = float32(math.Ceil(width/float64(signature.uiScale))) + 8
			}
		}
	}
	indicatorWidth := playerShareIndicatorReservation(shareContentWidth)
	nameItem.Size = eui.Point{
		X: signature.clientWidth - float32(iconSize*2) - 8 - indicatorWidth,
		Y: signature.rowUnits,
	}
	nameItem.Action = func() { handlePlayersClick(name) }
	row.AddItem(nameItem)

	if shareContentWidth > 0 {
		if shareIcon != nil {
			shareItem, backing := eui.NewImageItem(int(math.Ceil(float64(shareContentWidth))), int(math.Ceil(float64(signature.rowUnits))))
			backing.Deallocate()
			shareItem.Image = shareIcon
			shareItem.Filled = false
			shareItem.Border = 0
			shareItem.Disabled = signature.offline
			shareItem.SetTooltip(playerSharingTooltip(p))
			shareItem.Action = func() { handlePlayersClick(name) }
			row.AddItem(shareItem)
		} else {
			shareItem, _ := eui.NewText()
			shareItem.Text = indicator
			shareItem.FontSize = float32(signature.fontSize)
			shareItem.Face = signature.face
			shareItem.Size = eui.Point{X: shareContentWidth, Y: signature.rowUnits}
			shareItem.SetTooltip(playerSharingTooltip(p))
			shareItem.Action = func() { handlePlayersClick(name) }
			row.AddItem(shareItem)
		}
		spacer, _ := eui.NewText()
		spacer.Fixed = true
		spacer.Size = eui.Point{X: playerShareIconRightMargin, Y: signature.rowUnits}
		row.AddItem(spacer)
	}

	row.Action = func() { handlePlayersClick(name) }
	row.Size.Y = signature.rowUnits
	return cachedPlayerRow{row: row, profession: profItem, avatar: avItem, signature: signature}
}

func loadPlayerRowArtwork(cached cachedPlayerRow) (cachedPlayerRow, bool) {
	if cached.artworkLoaded {
		return cached, false
	}
	changed := false
	if pid := professionPictID(cached.signature.class); pid != 0 {
		if img := loadImage(pid); img != nil {
			cached.profession.Image = img
			cached.profession.ImageName = "prof:cl:" + fmt.Sprint(pid)
			cached.profession.Dirty = true
			changed = true
		}
	}
	state := uint8(0)
	if cached.signature.dead && !cached.signature.local {
		state = 32
	}
	colors := cached.signature.colors[:cached.signature.colorsLen]
	var avatar *ebiten.Image
	if cached.signature.pictID != 0 {
		if mobile := loadMobileFrame(cached.signature.pictID, state, colors); mobile != nil {
			avatar = mobile
		} else {
			avatar = loadImage(cached.signature.pictID)
		}
	}
	if avatar == nil {
		if pictID := defaultMobilePictID(genderFromString(cached.signature.gender)); pictID != 0 {
			if mobile := loadMobileFrame(pictID, state, nil); mobile != nil {
				avatar = mobile
			} else {
				avatar = loadImage(pictID)
			}
		}
	}
	if avatar != nil {
		cached.avatar.Image = avatar
		cached.avatar.Dirty = true
		changed = true
	}
	cached.artworkLoaded = true
	return cached, changed
}

// loadVisiblePlayerArtwork materializes cached sprite references only for rows
// intersecting the viewport, plus a small overscan margin. Rows retain artwork
// once visited so scrolling never causes load/unload churn.
func loadVisiblePlayerArtwork(force bool) bool {
	if playersWin == nil || playersList == nil || !playersWin.IsOpen() {
		return false
	}
	viewportSize := playersList.GetSize()
	if !force && playerArtworkViewport.valid && playerArtworkViewport.scroll == playersList.Scroll && playerArtworkViewport.size == viewportSize {
		return false
	}
	playerArtworkViewport.scroll = playersList.Scroll
	playerArtworkViewport.size = viewportSize
	playerArtworkViewport.valid = true

	overscan := float32(0)
	for _, cached := range cachedPlayerRows {
		overscan = max(overscan, cached.signature.rowUnits*2)
		break
	}
	viewTop := max(float32(0), playersList.Scroll.Y-overscan)
	viewBottom := playersList.Scroll.Y + viewportSize.Y + overscan
	y := float32(0)
	changed := false
	for _, item := range playersList.Contents {
		if item == nil {
			continue
		}
		top := y + item.Position.Y
		bottom := top + item.GetSize().Y
		if bottom >= viewTop && top <= viewBottom {
			if name := playersRowRefs[item]; name != "" {
				cached, ok := cachedPlayerRows[name]
				if ok {
					var loaded bool
					cached, loaded = loadPlayerRowArtwork(cached)
					cachedPlayerRows[name] = cached
					changed = changed || loaded
				}
			}
		}
		y += item.GetSize().Y + item.Position.Y
		if y > viewBottom {
			break
		}
	}
	return changed
}

func reusablePlayerHeader(key string, signature playerHeaderSignature, next map[string]cachedPlayerHeader) *eui.ItemData {
	cached, ok := cachedPlayerHeaders[key]
	if !ok || cached.signature != signature {
		cached = cachedPlayerHeader{
			row:       makePlayerGroupHeader(signature.title, signature.count, signature.width, signature.rowUnits, signature.fontSize, signature.editable),
			signature: signature,
		}
	}
	next[key] = cached
	return cached.row
}

func updatePlayersWindow() {
	if playersWin == nil || playersList == nil || !playersWin.IsOpen() {
		return
	}

	accent := eui.AccentColor()

	prevScroll := playersList.Scroll
	previousListSize := playersList.GetSize()

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
		// Sharing is a relationship with another player. Never display or count
		// stale/transient sharing flags on the local player's own record.
		if isLocalPlayerName(p.Name) {
			p.Sharee = false
			p.Sharing = false
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
	nextTitle := playersWindowTitle(onlineCount, shareeCount, shareCount)
	titleChanged := playersWin.Title != nextTitle
	playersWin.Title = nextTitle

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

	// Rebuild ordering while retaining rows whose appearance and layout did not
	// change. This avoids allocating image placeholders and looking up the same
	// profession/mobile artwork on every player-data refresh.
	playersRowRefs = map[*eui.ItemData]string{}
	playersGroupHeaders = map[*eui.ItemData]bool{}
	nextRows := make(map[string]cachedPlayerRow, len(exiles))
	nextHeaders := make(map[string]cachedPlayerHeader, len(groupCounts)+len(gs.PlayerGroups.Names))
	nextContents := make([]*eui.ItemData, 0, len(exiles)+len(groupCounts)+len(gs.PlayerGroups.Names))
	nextRecentPlayersExpiry = time.Time{}
	var selectedRow *eui.ItemData

	rowIndex := 0
	lastGroup := ""
	for _, customGroup := range gs.PlayerGroups.Names {
		if groupCounts["custom:"+customGroup] != 0 {
			continue
		}
		headerSignature := playerHeaderSignature{
			title:       customGroup,
			width:       clientWAvail,
			rowUnits:    rowUnits,
			fontSize:    fontSize,
			editable:    true,
			accentColor: accent,
			face:        mainFontBold,
		}
		header := reusablePlayerHeader("empty:"+customGroup, headerSignature, nextHeaders)
		nextContents = append(nextContents, header)
		playersGroupHeaders[header] = true
	}
	for _, p := range exiles {
		group := playerDisplayGroup(p, now)
		if group != lastGroup {
			custom := strings.HasPrefix(group, "custom:")
			headerSignature := playerHeaderSignature{
				title:       playerDisplayGroupTitle(group),
				count:       groupCounts[group],
				width:       clientWAvail,
				rowUnits:    rowUnits,
				fontSize:    fontSize,
				editable:    custom,
				accentColor: accent,
				face:        mainFontBold,
			}
			header := reusablePlayerHeader("group:"+group, headerSignature, nextHeaders)
			nextContents = append(nextContents, header)
			playersGroupHeaders[header] = true
			lastGroup = group
		}
		if group == fmt.Sprintf("auto:%d", playerGroupRecent) {
			expires := p.LastOnScreen.Add(recentPlayerWindow)
			if nextRecentPlayersExpiry.IsZero() || expires.Before(nextRecentPlayersExpiry) {
				nextRecentPlayersExpiry = expires
			}
		}
		signature := makePlayerRowSignature(p, myClan, clientWAvail, rowUnits, fontSize, ui)
		cached, ok := cachedPlayerRows[p.Name]
		if !ok || cached.signature != signature {
			cached = makePlayerRow(p, signature, rowIndex)
		}
		row := cached.row
		nextRows[p.Name] = cached
		nextContents = append(nextContents, row)
		playersRowRefs[row] = p.Name

		// Track selected row for highlight after search.
		if p.Name == selectedPlayerName {
			selectedRow = row
		}
		rowIndex++
	}
	cachedPlayerRows = nextRows
	cachedPlayerHeaders = nextHeaders
	contentsChanged := len(playersList.Contents) != len(nextContents)
	if !contentsChanged {
		for i := range nextContents {
			if playersList.Contents[i] != nextContents[i] {
				contentsChanged = true
				break
			}
		}
	}
	if contentsChanged {
		playersList.SetItems(nextContents)
	}

	// Size the list below any docked toolbar rows.
	sizeTextWindowList(playersList, clientWAvail, clientHAvail)
	layoutChanged := playersList.GetSize() != previousListSize
	playersList.Scroll = prevScroll
	selectionChanged := renderedPlayerSelection != selectedPlayerName
	if playersWin.SearchText != "" || contentsChanged || selectionChanged {
		applyPlayersSearch(playersWin.SearchText)
	}
	if selectedRow != nil {
		selectedRow.Filled = true
		selectedRow.Color = accent
	}
	renderedPlayerSelection = selectedPlayerName
	artworkChanged := loadVisiblePlayerArtwork(contentsChanged || layoutChanged)
	if contentsChanged || layoutChanged || titleChanged || selectionChanged || artworkChanged {
		playersWin.Refresh()
	}
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
		setMaterialButtonIcon(button, "edit")
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
	if started, allowDefault := legacyMacroTriggerClick(event, int64(acknowledgedFrameSnapshot())); started && !allowDefault {
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
