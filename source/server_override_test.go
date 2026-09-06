package main

import "testing"

func TestServerOverrideValidation(t *testing.T) {
	old := serverAddressOverride
	t.Cleanup(func() { serverAddressOverride = old })
	for _, addr := range []string{"127.0.0.1:5010", "demo.example:6000", "[::1]:5010", " demo.example:5010 "} {
		if err := setServerAddressOverride(addr); err != nil {
			t.Fatalf("%q: %v", addr, err)
		}
	}
	for _, addr := range []string{"", "localhost", ":5010", "localhost:0", "localhost:65536", "localhost:abc", "http://localhost:5010", "bad host:5010"} {
		before := serverAddressOverride
		if err := setServerAddressOverride(addr); err == nil {
			t.Fatalf("accepted %q", addr)
		}
		if serverAddressOverride != before {
			t.Fatal("invalid argument replaced override")
		}
	}
}

func TestServerOverrideSurvivesSettingsChangesWithoutBeingSaved(t *testing.T) {
	oldSettings, oldHost, oldOverride := gs, host, serverAddressOverride
	t.Cleanup(func() { gs, host, serverAddressOverride = oldSettings, oldHost, oldOverride })
	serverAddressOverride = "127.0.0.1:5010"
	for _, saved := range []string{"server.example:5010", "other.example:6000", ""} {
		gs.ServerAddress = saved
		applyServerAddressSetting()
		if host != serverAddressOverride {
			t.Fatalf("effective address = %q", host)
		}
		want := saved
		if want == "" {
			want = gsdef.ServerAddress
		}
		if gs.ServerAddress != want {
			t.Fatalf("saved address = %q, want %q", gs.ServerAddress, want)
		}
	}
	serverAddressOverride = ""
	applyServerAddressSetting()
	if host != gs.ServerAddress {
		t.Fatal("normal launch did not use saved address")
	}
}
