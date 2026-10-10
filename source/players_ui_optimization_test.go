package main

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func setupPlayersUpdateFixture(t testing.TB, count int) *Session {
	t.Helper()
	oldSettings, oldSessions, oldPlayers := gs, appSessions, players
	oldWindow, oldList := playersWin, playersList
	oldRows, oldHeaders := cachedPlayerRows, cachedPlayerHeaders
	oldRefs, oldGroupHeaders := playersRowRefs, playersGroupHeaders
	oldRenderSession, oldSelection := playersRenderSession, renderedPlayerSelection
	oldViewport, oldExpiry := playerArtworkViewport, nextRecentPlayersExpiry
	oldDirty := playersDirty
	oldFonts := [4]text.Face{mainFont, mainFontBold, mainFontItalic, mainFontBoldItalic}
	oldSource, oldBoldSource := eui.FontSource(), eui.BoldFontSource()
	t.Cleanup(func() {
		gs, appSessions, players = oldSettings, oldSessions, oldPlayers
		playersWin, playersList = oldWindow, oldList
		cachedPlayerRows, cachedPlayerHeaders = oldRows, oldHeaders
		playersRowRefs, playersGroupHeaders = oldRefs, oldGroupHeaders
		playersRenderSession, renderedPlayerSelection = oldRenderSession, oldSelection
		playerArtworkViewport, nextRecentPlayersExpiry = oldViewport, oldExpiry
		playersDirty = oldDirty
		mainFont, mainFontBold, mainFontItalic, mainFontBoldItalic = oldFonts[0], oldFonts[1], oldFonts[2], oldFonts[3]
		eui.SetFontSource(oldSource)
		eui.SetBoldFontSource(oldBoldSource)
	})
	if err := eui.Init(); err != nil {
		t.Fatal(err)
	}
	gs = gsdef
	gs.PlayerShareIcons = false
	gs.PlayerGroups = customGroups{Names: []string{"Friends", "Hunting"}, Assignments: make(map[string]string)}
	gs.ShowRecentPlayers = false
	gs.GroupClanMembers = true
	gs.PlayersFontSize = 12
	face := &text.GoTextFace{Source: eui.FontSource(), Size: 14}
	mainFont, mainFontBold, mainFontItalic, mainFontBoldItalic = face, face, face, face
	appSessions = newSessionManager(mustNewSession(primarySessionID))
	session := mustNewSession(2)
	session.setCharacterName("Fixture Hero")
	appSessions.slots[1], appSessions.selected = session, session.ID()
	players = make(map[string]*Player, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("Player %04d", i)
		players[name] = &Player{Name: name, Offline: i%3 == 0, SameClan: i%7 == 0, GlobalLabel: i % 8}
		if i%5 == 0 {
			gs.PlayerGroups.Assignments[playerCustomGroupKey(name)] = gs.PlayerGroups.Names[i%2]
		}
	}
	playersWin = eui.NewWindow()
	playersWin.Size = eui.Point{X: 360, Y: 480}
	playersWin.Open = true
	playersList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
	playersWin.AddItem(playersList)
	playersRenderSession = 0
	updatePlayersWindow()
	updatePlayersWindow()
	return session
}

func TestOrderedPlayerListPreservesGroupsAndOrdering(t *testing.T) {
	oldSettings := gs
	t.Cleanup(func() { gs = oldSettings })
	now := time.Unix(1000, 0)
	for _, recent := range []bool{false, true} {
		for _, clan := range []bool{false, true} {
			t.Run(fmt.Sprintf("recent=%v/clan=%v", recent, clan), func(t *testing.T) {
				gs.ShowRecentPlayers, gs.GroupClanMembers = recent, clan
				gs.PlayerGroups = customGroups{
					Names:       []string{" Hunting ", "HUNTING", "Friends", ""},
					Assignments: map[string]string{"player 0000": "hunting", "player 0001": "FRIENDS", "player 0002": "missing"},
				}
				ps := make([]Player, 128)
				for i := range ps {
					ps[i] = Player{Name: fmt.Sprintf("Player %04d", (i*37)%128), Offline: i%3 == 0, SameClan: i%5 == 0, FriendLabel: i % 8}
					switch i % 5 {
					case 0:
						ps[i].LastOnScreen = now.Add(-time.Second)
					case 1:
						ps[i].LastOnScreen = now.Add(-recentPlayerWindow)
					case 2:
						ps[i].LastOnScreen = now.Add(time.Second)
					}
				}
				ps[10].Name, ps[11].Name = "duplicate", "duplicate"
				ps[10].Offline, ps[11].Offline = false, false
				ps[10].SameClan, ps[11].SameClan = false, false
				ps[10].FriendLabel, ps[11].FriendLabel = 3, 3
				ps[10].LastOnScreen, ps[11].LastOnScreen = time.Time{}, time.Time{}
				ps[12].Name, ps[13].Name = "alpha", "Alpha"
				ps[14].Name, ps[15].IsNPC = "", true
				before := append([]Player(nil), ps...)
				want := append([]Player(nil), ps...)
				// Preserve the old comparator independently of the new classification.
				groupOf := func(p Player) string {
					if group := gs.PlayerGroups.group(playerCustomGroupKey(p.Name)); group != "" {
						return "custom:" + group
					}
					if gs.GroupClanMembers && p.SameClan {
						return "clan"
					}
					return fmt.Sprintf("auto:%d", playerGroup(p, now, gs.ShowRecentPlayers))
				}
				sort.Slice(want, func(i, j int) bool {
					a, b := want[i], want[j]
					groupA, groupB := groupOf(a), groupOf(b)
					if groupA != groupB {
						return playerDisplayGroupOrder(groupA) < playerDisplayGroupOrder(groupB)
					}
					if a.FriendLabel == 0 && b.FriendLabel != 0 {
						return false
					}
					if b.FriendLabel == 0 && a.FriendLabel != 0 {
						return true
					}
					if a.FriendLabel != b.FriendLabel {
						return a.FriendLabel < b.FriendLabel
					}
					return a.Name < b.Name
				})
				// Give the optimized path the same unnormalized input configuration.
				gs.PlayerGroups = customGroups{
					Names:       []string{" Hunting ", "HUNTING", "Friends", ""},
					Assignments: map[string]string{"player 0000": "hunting", "player 0001": "FRIENDS", "player 0002": "missing"},
				}
				got := orderedPlayerList(ps, now)
				if !reflect.DeepEqual(gs.PlayerGroups.Names, []string{"Hunting", "Friends"}) || gs.PlayerGroups.Assignments["player 0002"] != "" {
					t.Fatal("player grouping did not normalize custom groups")
				}
				for i, entry := range got {
					if !reflect.DeepEqual(*entry.player, want[i]) || entry.group != groupOf(want[i]) || entry.order != playerDisplayGroupOrder(entry.group) {
						t.Fatalf("entry %d = %+v in %q, want %+v in %q", i, *entry.player, entry.group, want[i], groupOf(want[i]))
					}
				}
				if !reflect.DeepEqual(ps, before) {
					t.Fatal("classifying players modified the source snapshot")
				}
			})
		}
	}
}

func TestPlayersUpdateKeepsUnchangedNamedTitleClean(t *testing.T) {
	session := setupPlayersUpdateFixture(t, 12)
	wantTitle := sessionPanelTitle(session, playersWindowTitle(8, 0, 0))
	if playersWin.Title != wantTitle {
		t.Fatalf("title = %q, want %q", playersWin.Title, wantTitle)
	}
	playersWin.Dirty = false
	updatePlayersWindow()
	if playersWin.Dirty {
		t.Fatal("unchanged named Players window requested another layout and repaint")
	}
	session.setCharacterName("Renamed Hero")
	updatePlayersWindow()
	if !playersWin.Dirty || playersWin.Title != sessionPanelTitle(session, playersWindowTitle(8, 0, 0)) {
		t.Fatal("character rename did not update and repaint the title")
	}
	playersWin.Dirty = false
	players["Player 0000"].Offline = false
	updatePlayersWindow()
	if !playersWin.Dirty || playersWin.Title != sessionPanelTitle(session, playersWindowTitle(9, 0, 0)) {
		t.Fatal("changed online count did not update and repaint the title")
	}
}

func TestPlayersUpdateRetainsSelectionAndExcludesUnnamedNPCs(t *testing.T) {
	session := setupPlayersUpdateFixture(t, 12)
	session.setSelectedPlayer("Player 0001")
	updatePlayersWindow()
	selected := cachedPlayerRows["Player 0001"].row
	title, count := playersWin.Title, len(playersList.Contents)
	players["unnamed"] = &Player{}
	players["npc"] = &Player{Name: "Creature", IsNPC: true}
	playersWin.Dirty = false
	updatePlayersWindow()
	if len(playersList.Contents) != count || playersWin.Title != title || playersWin.Dirty {
		t.Fatal("unnamed/NPC records changed the visible list or requested a repaint")
	}
	if cachedPlayerRows["Player 0001"].row != selected || !selected.Filled || session.selectedPlayerSnapshot() != "Player 0001" || renderedPlayerSelection != "Player 0001" {
		t.Fatal("unchanged refresh changed the selected player or row")
	}
}

func BenchmarkPlayersWindowUnchangedUpdate(b *testing.B) {
	for _, count := range []int{100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			setupPlayersUpdateFixture(b, count)
			b.ReportAllocs()
			for b.Loop() {
				updatePlayersWindow()
			}
		})
	}
}
