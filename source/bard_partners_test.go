package main

import (
	"reflect"
	"strings"
	"testing"

	"gothoom/eui"
)

func bardVisiblePlayersFixture(t *testing.T, session *Session) {
	t.Helper()
	oldSize := mobileSizeFunc
	mobileSizeFunc = func(uint16) int { return 20 }
	t.Cleanup(func() { mobileSizeFunc = oldSize })
	session.setPlayerIndex(1)
	session.draw.current.mobiles = map[uint8]frameMobile{
		1: {Index: 1, H: 50},
		2: {Index: 2, H: 70},
		3: {Index: 3, H: 51},
		4: {Index: 4, H: 49},
		5: {Index: 5, H: 52},
		6: {Index: 6, H: 50, Persist: true},
		7: {Index: 7, H: 30000},
	}
	session.draw.current.descriptors = map[uint8]frameDescriptor{
		1: {Type: kDescPlayer, Name: "Flutist"},
		2: {Type: kDescPlayer, Name: "Zed"},
		3: {Type: kDescPlayer, Name: "Blue"},
		4: {Type: kDescPlayer, Name: "Amy"},
		5: {Type: kDescNPC, Name: "Shopkeeper"},
		6: {Type: kDescPlayer, Name: "Stale"},
		7: {Type: kDescPlayer, Name: "Offscreen"},
	}
}

func TestBardVisiblePlayersSortAndFilter(t *testing.T) {
	_, session := bardReadyPanel(t)
	bardVisiblePlayersFixture(t, session)
	var names []string
	for _, player := range bardVisiblePlayers(session) {
		names = append(names, player.name)
	}
	if !reflect.DeepEqual(names, []string{"Amy", "Blue", "Zed"}) {
		t.Fatalf("visible players not ordered by distance then name: %v", names)
	}
	delete(session.draw.current.mobiles, 1)
	if len(bardVisiblePlayers(session)) != 0 || len(bardVisiblePlayers(nil)) != 0 {
		t.Fatal("listed players without the selected character's current position")
	}
}

func bardPickerBoxes(win *eui.WindowData) []*eui.ItemData {
	var boxes []*eui.ItemData
	for _, item := range win.Contents[0].Contents[1].Contents {
		if item.ItemType == eui.ITEM_CHECKBOX {
			boxes = append(boxes, item)
		}
	}
	return boxes
}

func toggleBardPartner(box *eui.ItemData) {
	box.Checked = !box.Checked
	box.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Item: box, Checked: box.Checked})
}

func TestBardPartnerPickerDraftCancelAndOK(t *testing.T) {
	p, session := bardReadyPanel(t)
	bardVisiblePlayersFixture(t, session)
	p.partners.Text = "Blue"
	p.showPartnerPicker()
	boxes := bardPickerBoxes(p.partnerPicker)
	if len(boxes) != 3 || !boxes[1].Checked {
		t.Fatal("existing partner was not selected")
	}
	toggleBardPartner(boxes[0])
	if !boxes[2].Disabled || boxes[0].Disabled || boxes[1].Disabled {
		t.Fatal("two-partner limit must leave selected players available to uncheck")
	}
	if p.partners.Text != "Blue" {
		t.Fatal("checkbox changed applied partners before OK")
	}
	clickMacroEditorButton(t, p.partnerPicker, "Cancel")
	if p.partners.Text != "Blue" || p.partnerPicker != nil {
		t.Fatal("Cancel changed partners or left picker open")
	}
	p.showPartnerPicker()
	boxes = bardPickerBoxes(p.partnerPicker)
	toggleBardPartner(boxes[0])
	toggleBardPartner(boxes[1])
	if boxes[2].Disabled {
		t.Fatal("unchecking did not allow another partner")
	}
	toggleBardPartner(boxes[2])
	clickMacroEditorButton(t, p.partnerPicker, "OK")
	if p.partners.Text != "Amy, Zed" || !session.commands.idle() {
		t.Fatal("OK did not apply names locally")
	}
	p.partners.Text = "Absent, Blu"
	p.showPartnerPicker()
	boxes = bardPickerBoxes(p.partnerPicker)
	if len(boxes) != 5 || !boxes[3].Checked || !boxes[4].Checked || !strings.Contains(boxes[3].Text, "entered name") {
		t.Fatal("opening the picker lost existing names or prefixes")
	}
	clickMacroEditorButton(t, p.partnerPicker, "OK")
	if p.partners.Text != "Absent, Blu" {
		t.Fatal("OK silently removed existing names")
	}
}

func TestBardPartnerPickerStaleCharacterAndClose(t *testing.T) {
	p, session := bardReadyPanel(t)
	bardVisiblePlayersFixture(t, session)
	p.partners.Text = "Blue"
	p.showPartnerPicker()
	toggleBardPartner(bardPickerBoxes(p.partnerPicker)[0])
	session.setCharacterName("Other")
	clickMacroEditorButton(t, p.partnerPicker, "OK")
	if p.partners.Text != "Blue" || !p.statusProblem {
		t.Fatal("stale character picker changed partners")
	}
	p.showPartnerPicker()
	win := p.partnerPicker
	p.win.Close()
	if win.IsOpen() || p.partnerPicker != nil {
		t.Fatal("closing Bard left its picker open")
	}
}
