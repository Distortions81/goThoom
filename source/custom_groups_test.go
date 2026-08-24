package main

import (
	"reflect"
	"testing"
)

func TestCustomGroupsAddAssignAndRemove(t *testing.T) {
	var groups customGroups
	if got := groups.add(" Friends "); got != "Friends" {
		t.Fatalf("add group = %q, want Friends", got)
	}
	if got := groups.add("friends"); got != "Friends" || len(groups.Names) != 1 {
		t.Fatalf("duplicate add = %q, names=%v", got, groups.Names)
	}
	groups.assign("person", "friends")
	if got := groups.group("person"); got != "Friends" {
		t.Fatalf("assigned group = %q, want Friends", got)
	}
	groups.remove("FRIENDS")
	if len(groups.Names) != 0 || groups.group("person") != "" {
		t.Fatalf("remove left names=%v assignments=%v", groups.Names, groups.Assignments)
	}
}

func TestCustomGroupsSettingsRoundTrip(t *testing.T) {
	want := gsdef
	want.PlayerGroups.add("Friends")
	want.PlayerGroups.assign("alice", "Friends")
	want.InventoryGroups.add("Tools")
	want.InventoryGroups.assign("123", "Tools")

	data, err := marshalSettingsDocument(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unmarshalSettingsDocument(data, gsdef)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.PlayerGroups, want.PlayerGroups) || !reflect.DeepEqual(got.InventoryGroups, want.InventoryGroups) {
		t.Fatalf("custom groups did not round trip: got players=%v inventory=%v", got.PlayerGroups, got.InventoryGroups)
	}
}

func TestNearbyVisiblePlayerGroupKeysUsesRangeAndCurrentMobiles(t *testing.T) {
	stateMu.Lock()
	originalState := state
	originalPlayerIndex := playerIndex
	playerIndex = 1
	state = drawState{
		mobiles: map[uint8]frameMobile{
			1: {Index: 1, H: 10, V: 10},
			2: {Index: 2, H: 40, V: 30},
			3: {Index: 3, H: 300, V: 300},
			4: {Index: 4, H: 20, V: 20, Persist: true},
		},
		descriptors: map[uint8]frameDescriptor{
			1: {Index: 1, Type: kDescPlayer, Name: "Self"},
			2: {Index: 2, Type: kDescPlayer, Name: "Nearby"},
			3: {Index: 3, Type: kDescPlayer, Name: "Far Away"},
			4: {Index: 4, Type: kDescPlayer, Name: "Carried Edge"},
		},
	}
	stateMu.Unlock()
	t.Cleanup(func() {
		stateMu.Lock()
		state = originalState
		playerIndex = originalPlayerIndex
		stateMu.Unlock()
	})

	got := nearbyVisiblePlayerGroupKeys()
	if !reflect.DeepEqual(got, []string{"nearby"}) {
		t.Fatalf("nearby visible players = %v, want [nearby]", got)
	}
}
