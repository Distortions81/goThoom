package main

import "testing"

func TestSmoothNameTagMotionDefaultsAndPersistence(t *testing.T) {
	if gsdef.SmoothNameTagMotion {
		t.Fatal("smooth nametag motion must default off")
	}
	missing, err := unmarshalSettingsDocument([]byte(`{"version":4}`), gsdef)
	if err != nil || missing.SmoothNameTagMotion {
		t.Fatalf("missing setting did not retain default: %v", err)
	}
	for _, enabled := range []bool{false, true} {
		want := cloneSettings(gsdef)
		want.SmoothNameTagMotion = enabled
		data, err := marshalSettingsDocument(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := unmarshalSettingsDocument(data, gsdef)
		if err != nil || got.SmoothNameTagMotion != enabled {
			t.Fatalf("setting %v did not round trip: %v", enabled, err)
		}
		profile, err := captureCharacterProfile("Motion Test", want)
		if err != nil {
			t.Fatal(err)
		}
		got, err = applyCharacterProfile(gsdef, profile)
		if err != nil || got.SmoothNameTagMotion != enabled {
			t.Fatalf("profile setting %v did not round trip: %v", enabled, err)
		}
	}
}
