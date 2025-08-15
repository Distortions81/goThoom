//go:build !test

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

var playersWin *eui.WindowData
var playersList *eui.ItemData
var playersDirty bool

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
	sort.Slice(ps, func(i, j int) bool { return ps[i].Name < ps[j].Name })
	exiles := make([]Player, 0, len(ps))
	shareCount, shareeCount := 0, 0
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

	header := fmt.Sprintf("Players Online: %d", len(exiles))
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
		name := p.Name
		tags := make([]string, 0, 2)
		if p.Sharing {
			tags = append(tags, "sharing")
		}
		if p.Sharee {
			tags = append(tags, "sharee")
		}
		if len(tags) > 0 {
			name = fmt.Sprintf("%s [%s]", name, strings.Join(tags, "+"))
		}

		// Build row flow: [avatar/default/blank] [profession/blank] [name]
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}

		// Icon sized to row height, with a small right margin.
		iconSize := int(rowUnits + 0.5)

		// Avatar: try live PictID first; else use default by gender; else blank.
		{
			avItem, _ := eui.NewImageItem(iconSize, iconSize)
			avItem.Margin = 4
			avItem.Border = 0
			avItem.Filled = false
			var img *ebiten.Image
			// Prefer mobile frame; use dead pose when fallen.
			state := uint8(0)
			if p.Dead {
				state = 32 // kPoseDead
			}
			if p.PictID != 0 {
				if m := loadMobileFrame(p.PictID, state, p.Colors); m != nil {
					img = m
				} else if im := loadImage(p.PictID); im != nil {
					img = im
				}
			}
			if img == nil {
				// Fallback to default character image per gender (like classic client)
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
			row.AddItem(avItem)
		}

		// Profession sprite if available; else add a blank image to preserve spacing.
		{
			profItem, _ := eui.NewImageItem(iconSize, iconSize)
			profItem.Margin = 4
			profItem.Border = 0
			profItem.Filled = false
			if pid := professionPictID(p.Class); pid != 0 {
				if img := loadImage(pid); img != nil {
					profItem.Image = img
					profItem.ImageName = "prof:cl:" + fmt.Sprint(pid)
				}
			}
			row.AddItem(profItem)
		}

		// Gender icon removed per request; columns remain avatar + profession only.

		// Name text constrained to the row height.
		t, _ := eui.NewText()
		t.Text = name
		t.FontSize = float32(fontSize)
		if p.Dead {
			// Dim the name for fallen players
			t.TextColor = eui.NewColor(180, 180, 180, 255)
		}
		// Reserve space for two icons (avatar + profession) + margins.
		t.Size = eui.Point{X: clientWAvail - float32(iconSize*2) - 8, Y: rowUnits}
		row.AddItem(t)

		// Ensure the row's height matches the content.
		row.Size.Y = rowUnits
		playersList.AddItem(row)
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
