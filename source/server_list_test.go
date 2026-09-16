package main

import "testing"

func TestServerListNormalizesWithoutRenumberingSavedSlots(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	gs.ServerAddress = "demo.example:5010"
	gs.ServerAddresses = []string{" demo.example:5010 ", "other.example:6000", "invalid", "DEMO.EXAMPLE:5010"}

	normalizeServerListSettings()
	if got, want := len(gs.ServerAddresses), 4; got != want {
		t.Fatalf("server slot count = %d, want %d (%v)", got, want, gs.ServerAddresses)
	}
	if got := gs.ServerAddresses[2]; got != "invalid" {
		t.Fatalf("invalid saved slot = %q, want it preserved for editing", got)
	}
	if !sameServerAddress(gs.ServerAddress, "demo.example:5010") {
		t.Fatalf("selected server = %q, want demo.example:5010", gs.ServerAddress)
	}
	if got := serverSlotForAddress("other.example:6000"); got != 2 {
		t.Fatalf("other server slot = %d, want 2", got)
	}
}

func TestServerSlotsAreAppendOnlyAndEditable(t *testing.T) {
	original := gs
	originalCharacters := characters
	t.Cleanup(func() {
		gs = original
		characters = originalCharacters
	})
	gs = gsdef
	normalizeServerListSettings()
	if !addServerAddress("demo.example:5010") {
		t.Fatal("could not add a valid custom server")
	}
	if addServerAddress("DEMO.EXAMPLE:5010") {
		t.Fatal("duplicate server address created another slot")
	}
	slot := serverSlotForAddress("demo.example:5010")
	if slot != 3 {
		t.Fatalf("added server slot = %d, want 3", slot)
	}
	characters = []Character{{Name: "Hero", ServerSlot: slot}}
	if !editServerSlot(slot, "renamed.example:6000") {
		t.Fatal("could not edit a server slot")
	}
	if got := serverSlotForAddress("renamed.example:6000"); got != slot {
		t.Fatalf("edited server moved from slot %d to %d", slot, got)
	}
	if characters[0].ServerSlot != slot {
		t.Fatalf("address edit changed character slot to %d", characters[0].ServerSlot)
	}
}

func TestInvalidServerSlotCanBeRepairedWithoutRenumbering(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	gs.ServerAddress = "invalid"
	gs.ServerAddresses = []string{"invalid", "other.example:5010"}

	if !editServerSlot(1, "repaired.example:6000") {
		t.Fatal("could not repair invalid server slot 1")
	}
	if got, want := len(gs.ServerAddresses), 2; got != want {
		t.Fatalf("server slot count = %d, want %d", got, want)
	}
	if got := gs.ServerAddresses[0]; got != "repaired.example:6000" {
		t.Fatalf("slot 1 = %q, want repaired address", got)
	}
	if got := gs.ServerAddress; got != "repaired.example:6000" {
		t.Fatalf("selected server = %q, want repaired address", got)
	}
}

func TestLegacyCustomServerListMigratesToEditableSlots(t *testing.T) {
	data := []byte(`{"version":4,"general":{"server_address":"custom.example:6000","server_addresses":["custom.example:6000"]}}`)
	loaded, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ServerAddresses) != len(builtInServerAddresses)+1 {
		t.Fatalf("migrated server slots = %v", loaded.ServerAddresses)
	}
	if !sameServerAddress(loaded.ServerAddresses[len(loaded.ServerAddresses)-1], "custom.example:6000") {
		t.Fatalf("custom server missing after migration: %v", loaded.ServerAddresses)
	}
}

func TestServerListDropdownIncludesEditor(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	options := append(serverAddresses(), editServerListOption)
	if got := options[len(options)-1]; got != editServerListOption {
		t.Fatalf("last server option = %q, want %q", got, editServerListOption)
	}
}

func TestServerListEditorShowsPresetAddresses(t *testing.T) {
	initFont()
	originalSettings := gs
	originalWindow := serverListWin
	originalContents := serverListContents
	serverListWin = nil
	serverListContents = nil
	gs = gsdef
	t.Cleanup(func() {
		if serverListWin != nil && serverListWin != originalWindow {
			serverListWin.RemoveWindow()
		}
		gs = originalSettings
		serverListWin = originalWindow
		serverListContents = originalContents
	})

	openServerListWindow()
	if got, want := len(serverListContents.Contents), len(builtInServerAddresses); got != want {
		t.Fatalf("server editor rows = %d, want %d", got, want)
	}
	for index, address := range builtInServerAddresses {
		input := serverListContents.Contents[index].Contents[1]
		if got := input.Text; got != address {
			t.Errorf("preset %d displayed address = %q, want %q", index+1, got, address)
		}
	}
}
