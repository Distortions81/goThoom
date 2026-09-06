package main

import (
	"fmt"
	"testing"

	"gothoom/eui"
)

func TestAlternatingRowColorSettingsDefaultsAndPersistence(t *testing.T) {
	if !gsdef.InventoryAlternatingRowColors || gsdef.ChatAlternatingRowColors || gsdef.ConsoleAlternatingRowColors || gsdef.PlayersAlternatingRowColors {
		t.Fatal("alternating row colors should default on only for inventory")
	}
	for mask := 0; mask < 16; mask++ {
		want := gsdef
		want.InventoryAlternatingRowColors = mask&1 != 0
		want.ChatAlternatingRowColors = mask&2 != 0
		want.ConsoleAlternatingRowColors = mask&4 != 0
		want.PlayersAlternatingRowColors = mask&8 != 0
		data, err := marshalSettingsDocument(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := unmarshalSettingsDocument(data, gsdef)
		if err != nil {
			t.Fatal(err)
		}
		if got.InventoryAlternatingRowColors != want.InventoryAlternatingRowColors || got.ChatAlternatingRowColors != want.ChatAlternatingRowColors || got.ConsoleAlternatingRowColors != want.ConsoleAlternatingRowColors || got.PlayersAlternatingRowColors != want.PlayersAlternatingRowColors {
			t.Fatalf("independent row colors did not survive persistence for mask %d", mask)
		}
	}
	for _, old := range []bool{false, true} {
		data := []byte(fmt.Sprintf(`{"version":%d,"interface":{"alternate_row_backgrounds":%t}}`, SETTINGS_VERSION, old))
		got, err := unmarshalSettingsDocument(data, gsdef)
		if err != nil {
			t.Fatal(err)
		}
		if got.InventoryAlternatingRowColors != old || got.ChatAlternatingRowColors || got.ConsoleAlternatingRowColors || got.PlayersAlternatingRowColors {
			t.Fatal("old global preference should migrate to inventory only")
		}
	}
}

func TestTextSearchRestoresIndependentAlternatingRowColors(t *testing.T) {
	initFont()
	originalSettings, originalChat, originalConsole := gs, chatWin, consoleWin
	t.Cleanup(func() { gs, chatWin, consoleWin = originalSettings, originalChat, originalConsole })
	chatWin, consoleWin = &eui.WindowData{}, &eui.WindowData{}
	for _, chatEnabled := range []bool{false, true} {
		gs.ChatAlternatingRowColors = chatEnabled
		gs.ConsoleAlternatingRowColors = !chatEnabled
		for _, tc := range []struct {
			win  *eui.WindowData
			want bool
		}{{chatWin, chatEnabled}, {consoleWin, !chatEnabled}} {
			list := &eui.ItemData{ParentWindow: tc.win, Contents: []*eui.ItemData{{Text: "first"}, {Text: "second"}}}
			applyTextWindowSearch(list, "second")
			if !list.Contents[1].Filled {
				t.Fatal("search match lost its highlight")
			}
			applyTextWindowSearch(list, "")
			if list.Contents[0].Filled || list.Contents[1].Filled != tc.want {
				t.Fatal("clearing search restored the wrong window's row colors")
			}
		}
	}
}
