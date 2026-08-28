package main

import (
	"reflect"
	"testing"
)

func TestFilterCommandPaletteActionsMatchesAllTermsAndRanksPrefix(t *testing.T) {
	actions := []commandPaletteAction{
		{label: "Window: Scripts", detail: "Open script manager"},
		{label: "Setting: Stop Spamming Scripts", detail: "scripts.stop_spamming_scripts = true"},
		{label: "Script: Rank Decoder", detail: "Show in the Scripts window"},
	}

	got := filterCommandPaletteActions(actions, "script rank")
	if len(got) != 1 || got[0].label != "Script: Rank Decoder" {
		t.Fatalf("filter result = %#v, want Rank Decoder only", got)
	}

	got = filterCommandPaletteActions(actions, "window")
	if len(got) != 2 || got[0].label != "Window: Scripts" {
		t.Fatalf("prefix-ranked result = %#v", got)
	}
}

func TestFindSettingEntryUsesReadableFullName(t *testing.T) {
	entry, err := findSettingEntry("audio.master-volume")
	if err != nil {
		t.Fatal(err)
	}
	if entry.field != "MasterVolume" {
		t.Fatalf("field = %q, want MasterVolume", entry.field)
	}

	if _, err := findSettingEntry("enabled"); err == nil {
		t.Fatal("ambiguous short setting name unexpectedly resolved")
	}
}

func TestParseSettingValueSupportsFriendlyBooleansAndJSON(t *testing.T) {
	boolField := reflect.ValueOf(true)
	got, err := parseSettingValue(boolField, "toggle")
	if err != nil {
		t.Fatal(err)
	}
	if got.Bool() {
		t.Fatal("toggle did not invert boolean")
	}

	floatField := reflect.ValueOf(float64(0))
	got, err = parseSettingValue(floatField, "0.75")
	if err != nil {
		t.Fatal(err)
	}
	if got.Float() != 0.75 {
		t.Fatalf("float = %v, want 0.75", got.Float())
	}

	stringField := reflect.ValueOf("")
	got, err = parseSettingValue(stringField, "America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "America/Denver" {
		t.Fatalf("string = %q", got.String())
	}
}

func TestCommandSettingSpecialsUseReadableValues(t *testing.T) {
	entry, err := findSettingEntry("rendering.artwork_upscale_style")
	if err != nil {
		t.Fatal(err)
	}
	field := reflect.ValueOf(gsdef).FieldByName(entry.field)
	got, err := parseCommandSettingValue(entry, field, "crisp")
	if err != nil {
		t.Fatal(err)
	}
	if int(got.Int()) != artworkUpscaleCrisp {
		t.Fatalf("mode = %d, want crisp", got.Int())
	}
	if value := formatCommandSettingValue(entry, got); value != `"crisp"` {
		t.Fatalf("formatted mode = %s", value)
	}
}
