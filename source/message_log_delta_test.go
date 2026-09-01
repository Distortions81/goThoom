package main

import "testing"

func TestMessageWindowStateSyncsOnlyDeltaAndRetainedTrim(t *testing.T) {
	log := messageLog{max: 3}
	log.AddTyped("one", "a")
	log.AddTyped("two", "b")

	var state messageWindowState
	messages, types, firstChanged := state.Sync(&log, "3:04PM", false, false)
	if firstChanged != 0 || !state.reset || state.dropped != 0 {
		t.Fatalf("initial sync changed=%d reset=%v dropped=%d", firstChanged, state.reset, state.dropped)
	}
	if len(messages) != 2 || messages[0] != "one" || messages[1] != "two" || types[1] != "b" {
		t.Fatalf("initial sync = %#v %#v", messages, types)
	}

	log.AddTyped("three", "c")
	messages, _, firstChanged = state.Sync(&log, "3:04PM", false, false)
	if firstChanged != 2 || state.reset || state.dropped != 0 || len(messages) != 3 || messages[2] != "three" {
		t.Fatalf("append sync changed=%d reset=%v dropped=%d messages=%#v", firstChanged, state.reset, state.dropped, messages)
	}

	log.AddTyped("four", "d")
	messages, types, firstChanged = state.Sync(&log, "3:04PM", false, false)
	if firstChanged != 2 || state.reset || state.dropped != 1 {
		t.Fatalf("trim sync changed=%d reset=%v dropped=%d", firstChanged, state.reset, state.dropped)
	}
	if len(messages) != 3 || messages[0] != "two" || messages[2] != "four" || types[2] != "d" {
		t.Fatalf("trimmed sync = %#v %#v", messages, types)
	}
}

func TestMessageWindowStateForcesFullSyncForTimestampChange(t *testing.T) {
	log := messageLog{max: 3}
	log.Add("one")
	var state messageWindowState
	state.Sync(&log, "3:04PM", false, false)

	messages, _, firstChanged := state.Sync(&log, "15:04", true, false)
	if firstChanged != 0 || !state.reset || len(messages) != 1 || messages[0] == "one" {
		t.Fatalf("timestamp resync changed=%d reset=%v messages=%#v", firstChanged, state.reset, messages)
	}
}
