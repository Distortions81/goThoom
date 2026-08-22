package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gothoom/eui"
)

func TestLegacyMacroLibraryEmbeddedCorpusHasDisplayNames(t *testing.T) {
	if len(legacyMacroBundledLibrary) != 13 {
		t.Fatalf("bundled entries = %d, want 13", len(legacyMacroBundledLibrary))
	}
	for _, entry := range legacyMacroBundledLibrary {
		if entry.Filename == "" || entry.SourceURL == "" || entry.EmbeddedPath == "" {
			t.Fatalf("incomplete bundled entry: %#v", entry)
		}
		source, err := legacyMacroLibrarySource(entry)
		if err != nil {
			t.Fatalf("read %q: %v", entry.Filename, err)
		}
		if strings.TrimSpace(source) == "" {
			t.Fatalf("bundled source %q is empty", entry.Filename)
		}
		if name := legacyMacroLibraryDisplayName(entry.Filename, source); name == entry.Filename {
			t.Fatalf("bundled source %q has no // Name: display name", entry.Filename)
		}
	}
}

func TestLegacyMacroLibraryDiscoversNameConventionAndFilenameFallback(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	entries, err := legacyMacroLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(legacyMacroBundledLibrary) {
		t.Fatalf("initial library entries = %d, want %d", len(entries), len(legacyMacroBundledLibrary))
	}
	keys := legacyMacroLibraryEntryForTest(entries, "keys.mac")
	if keys.Name != "Keys" {
		t.Fatalf("keys display name = %q, want Keys", keys.Name)
	}
	if !keys.Bundled {
		t.Fatal("materialized keys source was not marked bundled")
	}
	materialized, err := os.ReadFile(keys.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(materialized), "// Name: Keys") {
		preview := string(materialized)
		if len(preview) > 160 {
			preview = preview[:160]
		}
		t.Fatalf("materialized source has no name convention: %q", preview)
	}

	namedPath := filepath.Join(legacyMacroLibraryPath(), "anything.mac")
	if err := os.WriteFile(namedPath, []byte("// Name: My Favorite Macro\nf1 \"/wave\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fallbackPath := filepath.Join(legacyMacroLibraryPath(), "no-title.mac")
	if err := os.WriteFile(fallbackPath, []byte("f2 \"/bow\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err = legacyMacroLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	if got := legacyMacroLibraryEntryForTest(entries, "anything.mac").Name; got != "My Favorite Macro" {
		t.Fatalf("named source display name = %q", got)
	}
	if got := legacyMacroLibraryEntryForTest(entries, "no-title.mac").Name; got != "no-title.mac" {
		t.Fatalf("fallback source display name = %q", got)
	}
}

func TestLegacyMacroLibrarySelectionsDoNotRewriteMacroRoots(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	if _, err := legacyMacroLibraryEntries(); err != nil {
		t.Fatal(err)
	}
	globalPath := filepath.Join(legacyMacroLibraryPath(), "global.mac")
	if err := os.WriteFile(globalPath, []byte("// Name: Global Test\nf3 \"/global\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	playerPath := filepath.Join(legacyMacroLibraryPath(), "player.mac")
	if err := os.WriteFile(playerPath, []byte("// Name: Player Test\nf4 \"/player\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(legacyMacrosDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(legacyMacrosDir(), "Default")
	defaultText := "f1 \"/default\\r\"\n"
	if err := os.WriteFile(defaultPath, []byte(defaultText), 0o644); err != nil {
		t.Fatal(err)
	}
	characterPath := filepath.Join(legacyMacrosDir(), "Gaia")
	characterText := "include \"Default\"\nf2 \"/character\\r\"\n"
	if err := os.WriteFile(characterPath, []byte(characterText), 0o644); err != nil {
		t.Fatal(err)
	}

	global, err := setLegacyMacroLibraryEntryEnabled("global.mac", legacyMacroLibraryGlobal, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if !global.Enabled || !global.Changed || global.SourcePath != globalPath {
		t.Fatalf("global selection result = %#v", global)
	}
	player, err := setLegacyMacroLibraryEntryEnabled("player.mac", legacyMacroLibraryPlayer, "Gaia", true)
	if err != nil {
		t.Fatal(err)
	}
	if !player.Enabled || !player.Changed || player.SourcePath != playerPath {
		t.Fatalf("player selection result = %#v", player)
	}

	for path, want := range map[string]string{defaultPath: defaultText, characterPath: characterText} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("%q was rewritten:\n%s", path, got)
		}
	}

	selectionData, err := os.ReadFile(legacyMacroLibrarySelectionPath())
	if err != nil {
		t.Fatal(err)
	}
	var selection legacyMacroLibrarySelection
	if err := json.Unmarshal(selectionData, &selection); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(strings.Join(selection.Global, ","), "global.mac") || !strings.EqualFold(strings.Join(selection.Players["Gaia"], ","), "player.mac") {
		t.Fatalf("stored selections = %#v", selection)
	}

	globalIDs, err := legacyMacroLibraryEnabledIDs(legacyMacroLibraryGlobal, "")
	if err != nil || !globalIDs[legacyMacroLibraryIDKey("global.mac")] {
		t.Fatalf("global enabled IDs = %#v, %v", globalIDs, err)
	}
	playerIDs, err := legacyMacroLibraryEnabledIDs(legacyMacroLibraryPlayer, "Gaia")
	if err != nil || !playerIDs[legacyMacroLibraryIDKey("player.mac")] {
		t.Fatalf("player enabled IDs = %#v, %v", playerIDs, err)
	}

	if err := loadLegacyMacrosForCharacter("Gaia"); err != nil {
		t.Fatalf("load selected macros: %v", err)
	}
	if !legacyMacroSourcesContain("global.mac") || !legacyMacroSourcesContain("player.mac") {
		t.Fatalf("selected sources were not loaded: %#v", legacyMacroSourcesSnapshot())
	}

	if _, err := setLegacyMacroLibraryEntryEnabled("global.mac", legacyMacroLibraryGlobal, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := setLegacyMacroLibraryEntryEnabled("player.mac", legacyMacroLibraryPlayer, "Gaia", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(globalPath); err != nil {
		t.Fatalf("disabling removed global source: %v", err)
	}
	if _, err := os.Stat(playerPath); err != nil {
		t.Fatalf("disabling removed player source: %v", err)
	}
}

func TestLegacyMacroLibraryMissingSelectionReportsDiagnostic(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	if err := os.MkdirAll(legacyMacroLibraryPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	selection := legacyMacroLibrarySelection{Global: []string{"missing.mac"}}
	data, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyMacroLibrarySelectionPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	err = loadLegacyMacrosForCharacter("Gaia")
	if err == nil || !strings.Contains(err.Error(), "selected library macro \"missing.mac\" does not exist") {
		t.Fatalf("load error = %v", err)
	}
	program := legacyMacroProgramSnapshot()
	if len(program.Diagnostics) != 1 || !strings.Contains(program.Diagnostics[0].Message, "missing.mac") {
		t.Fatalf("diagnostics = %#v", program.Diagnostics)
	}
}

func TestLegacyMacroLibraryRejectsPathLikeIDsAndCharacters(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	if _, err := legacyMacroLibraryEntries(); err != nil {
		t.Fatal(err)
	}
	if _, err := setLegacyMacroLibraryEntryEnabled("../outside.mac", legacyMacroLibraryGlobal, "", true); err == nil {
		t.Fatal("path-like macro ID was accepted")
	}
	if _, err := setLegacyMacroLibraryEntryEnabled("keys.mac", legacyMacroLibraryPlayer, "../outside", true); err == nil {
		t.Fatal("path-like player name was accepted")
	}
}

func TestLegacyMacroLibraryLayoutLeavesRoomForRows(t *testing.T) {
	originalWin := legacyMacroLibraryWin
	originalRoot := legacyMacroLibraryRoot
	originalList := legacyMacroLibraryList
	originalButtons := legacyMacroLibraryButtons
	t.Cleanup(func() {
		legacyMacroLibraryWin = originalWin
		legacyMacroLibraryRoot = originalRoot
		legacyMacroLibraryList = originalList
		legacyMacroLibraryButtons = originalButtons
	})

	legacyMacroLibraryWin = eui.NewWindow()
	legacyMacroLibraryWin.Size = eui.Point{X: 720, Y: 560}
	legacyMacroLibraryRoot = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	legacyMacroLibraryList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Scrollable: true, Fixed: true}
	legacyMacroLibraryButtons = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}

	legacyMacroLibraryLayout()
	if legacyMacroLibraryRoot.Size.Y <= legacyMacroLibraryList.Size.Y {
		t.Fatalf("root height = %f, list height = %f", legacyMacroLibraryRoot.Size.Y, legacyMacroLibraryList.Size.Y)
	}
	if legacyMacroLibraryList.Size.Y <= 24 {
		t.Fatalf("list height = %f, want room for rows", legacyMacroLibraryList.Size.Y)
	}
	reservedHeight := legacyMacroLibraryRoot.Size.Y - legacyMacroLibraryList.Size.Y
	const wantReservedHeight = legacyMacroLibraryButtonsHeight + legacyMacroLibraryBottomGap
	if reservedHeight < wantReservedHeight {
		t.Fatalf("reserved height = %f, want at least %d", reservedHeight, wantReservedHeight)
	}
}

func legacyMacroLibraryEntryForTest(entries []legacyMacroLibraryEntry, id string) legacyMacroLibraryEntry {
	for _, entry := range entries {
		if entry.ID == id {
			return entry
		}
	}
	panic("missing legacy macro library entry " + id)
}

func legacyMacroSourcesContain(name string) bool {
	for _, source := range legacyMacroSourcesSnapshot() {
		if filepath.Base(source.Path) == name {
			return true
		}
	}
	return false
}
