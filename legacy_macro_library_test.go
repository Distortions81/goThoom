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
		metadata := parseLegacyMacroLibraryMetadata(entry.Filename, source)
		if metadata.Name == entry.Filename || metadata.Tags == "" || metadata.Description == "" || metadata.Author == "" || metadata.Website == "" || metadata.Update == "" {
			t.Fatalf("bundled source %q has incomplete metadata: %#v", entry.Filename, metadata)
		}
	}
}

func TestLegacyMacroMetadataStopsAtMacroCode(t *testing.T) {
	metadata := parseLegacyMacroLibraryMetadata("example.mac", strings.Join([]string{
		"// Metadata",
		"// Name: Real Name",
		"f1 \"/wave\\r\"",
		"// Name: Mentioned Later",
	}, "\n"))
	if metadata.Name != "Real Name" {
		t.Fatalf("metadata name = %q, want Real Name", metadata.Name)
	}
}

func TestLegacyMacroLibraryDecodesMacRomanMetadata(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	if _, err := legacyMacroLibraryEntries(); err != nil {
		t.Fatal(err)
	}
	raw := append([]byte("// Metadata\n// Name: Caf"), 0x8e)
	raw = append(raw, []byte(" Dance\n// Desc: Caf")...)
	raw = append(raw, 0x8e)
	raw = append(raw, []byte(" helper.\nf1 \"/dance\\r\"\n")...)
	path := filepath.Join(legacyMacroLibraryPath(), "macroman.mac")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := legacyMacroLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	entry := legacyMacroLibraryEntryForTest(entries, "macroman.mac")
	if entry.Name != "Café Dance" || entry.Description != "Café helper." {
		t.Fatalf("MacRoman metadata = %#v", entry)
	}
}

func TestLegacyMacroLibraryEmbeddedInfoUsesEmbeddedSource(t *testing.T) {
	originalWASM := isWASM
	isWASM = true
	t.Cleanup(func() { isWASM = originalWASM })

	entries, err := legacyMacroLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	entry := legacyMacroLibraryEntryForTest(entries, "official-example.mac")
	info, err := collectLegacyMacroLibraryInfo(entry)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Commands) == 0 || len(info.Hotkeys) == 0 {
		t.Fatalf("embedded info = %#v, want commands and hotkeys", info)
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
	metadataReference, err := os.ReadFile(filepath.Join(legacyMacroLibraryPath(), legacyMacroLibraryMetadataName))
	if err != nil {
		t.Fatalf("read installed metadata reference: %v", err)
	}
	if !strings.Contains(string(metadataReference), "// Metadata") {
		t.Fatalf("installed metadata reference is incomplete: %q", metadataReference)
	}
	if err := os.WriteFile(filepath.Join(legacyMacroLibraryPath(), legacyMacroLibraryMetadataName), []byte("old guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := legacyMacroLibraryEntries(); err != nil {
		t.Fatal(err)
	}
	metadataReference, err = os.ReadFile(filepath.Join(legacyMacroLibraryPath(), legacyMacroLibraryMetadataName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(metadataReference), "old guide") || !strings.Contains(string(metadataReference), "Separate each tag with a") {
		t.Fatalf("installed metadata reference was not refreshed: %q", metadataReference)
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
	if err := os.WriteFile(namedPath, []byte("// Metadata\n// Name: My Favorite Macro\n// Version: 1.2.3\n// Tags: healer, sharing\n// Desc: Keeps the group healthy.\n// Author: Gaia\n// License: MIT (2026)\n// Website: https://example.com/macros\n// Update: favorite-macro.mac\nf1 \"/wave\\r\"\n"), 0o644); err != nil {
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
	named := legacyMacroLibraryEntryForTest(entries, "anything.mac")
	if got := named.Name; got != "My Favorite Macro" {
		t.Fatalf("named source display name = %q", got)
	}
	if named.Version != "1.2.3" || named.Tags != "healer, sharing" || named.Description != "Keeps the group healthy." || named.Author != "Gaia" || named.License != "MIT (2026)" || named.Website != "https://example.com/macros" || named.Update != "favorite-macro.mac" {
		t.Fatalf("named source metadata = %#v", named)
	}
	if got := legacyMacroLibraryEntryForTest(entries, "no-title.mac").Name; got != "no-title.mac" {
		t.Fatalf("fallback source display name = %q", got)
	}
}

func TestLegacyMacroLibraryUpdatesPristineSourcesButPreservesEdits(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	entries, err := legacyMacroLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	keys := legacyMacroLibraryEntryForTest(entries, "keys.mac")
	keysText, err := os.ReadFile(keys.Path)
	if err != nil {
		t.Fatal(err)
	}
	keysText = append(keysText, []byte("\n// my local change\n")...)
	if err := os.WriteFile(keys.Path, keysText, 0o644); err != nil {
		t.Fatal(err)
	}

	official := legacyMacroLibraryEntryForTest(entries, "official-example.mac")
	officialText, err := os.ReadFile(official.Path)
	if err != nil {
		t.Fatal(err)
	}
	oldOfficial := string(officialText)
	for _, comparison := range []string{
		"if i < numbounces",
		"if i < seed",
		"if charind < numchar",
		"if wordind < numword",
	} {
		oldOfficial = strings.Replace(oldOfficial, comparison, strings.Replace(comparison, " < ", " &lt; ", 1), 1)
	}
	if err := os.WriteFile(official.Path, []byte(oldOfficial), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := legacyMacroLibraryEntries(); err != nil {
		t.Fatal(err)
	}
	gotKeys, err := os.ReadFile(keys.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotKeys) != string(keysText) {
		t.Fatal("user-edited bundled macro was overwritten")
	}
	gotOfficial, err := os.ReadFile(official.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gotOfficial), "&lt;") {
		t.Fatal("pristine pre-manifest source was not updated")
	}
}

func TestLegacyMacroLibraryRecoversDamagedManifestWithoutOverwritingMacros(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	if err := os.MkdirAll(legacyMacroLibraryPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(legacyMacroLibraryPath(), legacyMacroLibraryManifestName)
	if err := os.WriteFile(manifestPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	keysPath := filepath.Join(legacyMacroLibraryPath(), "keys.mac")
	custom := []byte("// my customized keys\nf1 \"/wave\\r\"\n")
	if err := os.WriteFile(keysPath, custom, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := legacyMacroLibraryEntries(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(keysPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Fatal("customized macro was overwritten while recovering manifest")
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("recovered manifest is invalid: %v", err)
	}
}

func TestLegacyMacroLibraryInfoListsCommandsAndHotkeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "details.mac")
	text := strings.Join([]string{
		"\"/wave\"",
		"{",
		"}",
		"'brb'",
		"{",
		"}",
		"control-shift-f1",
		"{",
		"}",
		"click2",
		"{",
		"}",
		"wheelup",
		"{",
		"}",
		"helper",
		"{",
		"}",
	}, "\n")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := legacyMacroLibraryInfoText(legacyMacroLibraryEntry{
		ID:          "details.mac",
		Path:        path,
		Version:     "1.2.3",
		Tags:        "fighter, sunstone",
		Description: "A useful combat helper.",
		Author:      "Gorvin",
		License:     "MIT (2026)",
		Website:     "https://example.com/macros",
		Update:      "details.mac",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Commands:",
		"Description: A useful combat helper.",
		"Version: 1.2.3",
		"Tags: fighter, sunstone",
		"Author: Gorvin",
		"License: MIT (2026)",
		"Website: https://example.com/macros",
		"Update: details.mac",
		"/wave",
		"brb (replacement)",
		"Hotkeys:",
		"Control-Shift-F1",
		"Right Click",
		"Wheel Up",
	} {
		if !strings.Contains(info, want) {
			t.Fatalf("macro info %q does not contain %q", info, want)
		}
	}
	if strings.Contains(info, "\nhelper\n") {
		t.Fatalf("macro info includes internal helper: %q", info)
	}
}

func TestLegacyMacroLibraryRowLabelUsesAvailableSpace(t *testing.T) {
	if got := legacyMacroLibraryRowLabel("Keys", "Common bindings", 400); got != "Keys — Common bindings" {
		t.Fatalf("wide row label = %q", got)
	}
	got := legacyMacroLibraryRowLabel("Keys", strings.Repeat("description ", 12), 120)
	if !strings.HasPrefix(got, "Keys — ") || !strings.HasSuffix(got, "...") {
		t.Fatalf("narrow row label = %q, want abbreviated description", got)
	}
}

func TestLegacyMacroLibraryInfoUsesThreeColumns(t *testing.T) {
	columns := legacyMacroLibraryInfoColumns(legacyMacroLibraryInfo{
		Metadata: []string{"Description: Useful"},
		Commands: []string{"/wave"},
		Hotkeys:  []string{"F1"},
	})
	if len(columns.Contents) != 3 {
		t.Fatalf("info column count = %d, want 3", len(columns.Contents))
	}
	for index, want := range []string{"About", "Commands", "Hotkeys"} {
		column := columns.Contents[index]
		if len(column.Contents) == 0 || column.Contents[0].Text != want {
			t.Fatalf("info column %d heading = %#v, want %q", index, column.Contents, want)
		}
	}
}

func TestLegacyMacroLibraryDiagnosticsIncludesParseAndRuntimeErrors(t *testing.T) {
	legacyMacrosMu.Lock()
	originalProgram := legacyMacrosProgram
	originalRuntime := legacyMacrosRuntime
	legacyMacrosProgram = legacyMacroProgram{Diagnostics: []legacyMacroDiagnostic{{
		Location: legacyMacroLocation{Path: "parse.mac", Line: 2, Column: 3},
		Message:  "parse error",
	}}}
	legacyMacrosRuntime = &legacyMacroRuntime{diagnostics: []legacyMacroDiagnostic{{
		Location: legacyMacroLocation{Path: "run.mac", Line: 4, Column: 5},
		Message:  "runtime error",
	}}}
	legacyMacrosMu.Unlock()
	t.Cleanup(func() {
		legacyMacrosMu.Lock()
		legacyMacrosProgram = originalProgram
		legacyMacrosRuntime = originalRuntime
		legacyMacrosMu.Unlock()
	})

	diagnostics := legacyMacroLibraryDiagnostics()
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v, want parse and runtime errors", diagnostics)
	}
	message := strings.Join([]string{diagnostics[0].Error(), diagnostics[1].Error()}, "\n")
	if !strings.Contains(message, "parse.mac:2:3: parse error") || !strings.Contains(message, "run.mac:4:5: runtime error") {
		t.Fatalf("diagnostics message = %q", message)
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
