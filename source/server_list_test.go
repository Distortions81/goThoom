package main

import "testing"

func TestServerListKeepsBuiltInServersAndDeduplicatesCustomServers(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	gs.ServerAddress = "demo.example:5010"
	gs.ServerAddresses = []string{" demo.example:5010 ", "other.example:6000", "invalid"}

	normalizeServerListSettings()
	if got, want := len(gs.ServerAddresses), 2; got != want {
		t.Fatalf("custom server count = %d, want %d (%v)", got, want, gs.ServerAddresses)
	}
	if !sameServerAddress(gs.ServerAddress, "demo.example:5010") {
		t.Fatalf("selected server = %q, want demo.example:5010", gs.ServerAddress)
	}
	addresses := serverAddresses()
	if len(addresses) < len(builtInServerAddresses)+2 {
		t.Fatalf("server addresses = %v, want built-in and custom entries", addresses)
	}
	for _, builtIn := range builtInServerAddresses {
		if !isBuiltInServerAddress(builtIn) {
			t.Fatalf("%q was not recognized as built-in", builtIn)
		}
	}
}

func TestServerListCannotRemoveBuiltInServer(t *testing.T) {
	original := gs
	t.Cleanup(func() { gs = original })
	gs = gsdef
	if removeServerAddress(builtInServerAddresses[0]) {
		t.Fatal("removed a built-in server")
	}
	if !addServerAddress("demo.example:5010") {
		t.Fatal("could not add a valid custom server")
	}
	if !removeServerAddress("demo.example:5010") {
		t.Fatal("could not remove a custom server")
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
