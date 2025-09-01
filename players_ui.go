//go:build !test

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

var playersWin *eui.WindowData
var playersList *eui.ItemData
var playersDirty bool
var playersRowRefs = map[*eui.ItemData]string{}
var playersCtxWin *eui.WindowData
var selectedPlayerName string
var lastPlayerClickName string
var lastPlayerClickTime time.Time

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

	// Gather current players and filter to non-NPCs with names.
	ps := getPlayers()
	// Sort: online (recently seen and not explicitly offline) first,
	// then by label/color group, then by name.
	sort.Slice(ps, func(i, j int) bool {
		staleI := time.Since(ps[i].LastSeen) > 5*time.Minute
		staleJ := time.Since(ps[j].LastSeen) > 5*time.Minute
		offI := ps[i].Offline || staleI
		offJ := ps[j].Offline || staleJ
		if offI != offJ {
			return !offI && offJ
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
		if !(p.Offline || time.Since(p.LastSeen) > 5*time.Minute) {
			onlineCount++
		}
	}

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

	// Rebuild contents: header + one row per player
	// Layout per row: [avatar (or default/blank)] [profession (or blank)] [name]
	playersList.Contents = nil
	playersRowRefs = map[*eui.ItemData]string{}

	header := fmt.Sprintf("Players Online: %d", onlineCount)
	// Include simple share summary when relevant.
	if shareCount > 0 || shareeCount > 0 {
		parts := make([]string, 0, 2)
		if shareCount > 0 {
			parts = append(parts, fmt.Sprintf("sharing %d", shareCount))
		}
		if shareeCount > 0 {
			parts = append(parts, fmt.Sprintf("sharees %d", shareeCount))
		}
		header = fmt.Sprintf("%s — %s", header, strings.Join(parts, ", "))
	}
	ht, _ := eui.NewText()
	ht.Text = header
	ht.FontSize = float32(fontSize)
	ht.Size = eui.Point{X: clientWAvail, Y: rowUnits}
	playersList.AddItem(ht)

	for _, p := range exiles {
		offline := p.Offline || time.Since(p.LastSeen) > 5*time.Minute
		name := p.Name
		tags := make([]string, 0, 3)
		if p.Sharee {
			tags = append(tags, "<")
		}
		if p.Sharing {
			tags = append(tags, ">")
		}
		if sameRealClan(p.clan, myClan) {
			tags = append(tags, "*")
		}
		if len(tags) > 0 {
			name = fmt.Sprintf("%s %s", name, strings.Join(tags, "--"))
		}

		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}

		if p.FriendLabel > 0 {
			row.Outlined = true
			row.Border = 3
			row.OutlineColor = labelColor(p.FriendLabel)
		}

		// Highlight if selected.
		if p.Name == selectedPlayerName {
			row.Filled = true
			if playersWin != nil && playersWin.Theme != nil {
				row.Color = playersWin.Theme.Button.SelectedColor
			}
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
		t.Size = eui.Point{X: clientWAvail - float32(iconSize*2) - 8, Y: rowUnits}
		// Click selects this player.
		n := p.Name
		t.Action = func() { handlePlayersClick(n) }
		row.AddItem(t)

		// Also allow clicking the row background to select.
		n = p.Name
		row.Action = func() { handlePlayersClick(n) }

		row.Size.Y = rowUnits
		playersList.AddItem(row)
		playersRowRefs[row] = p.Name
	}

	// Size flows to client area like other text windows.
	if playersList.Parent != nil {
		playersList.Parent.Size.X = clientWAvail
		playersList.Parent.Size.Y = clientHAvail
	}
	playersList.Size.X = clientWAvail
	playersList.Size.Y = clientHAvail
	playersWin.Refresh()
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
		options = append(options, "Label")
		n := displayName
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
