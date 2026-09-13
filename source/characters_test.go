package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScrambleHashBlank(t *testing.T) {
	if s := scrambleHash("name", ""); s != "" {
		t.Fatalf("scrambleHash on blank hash = %q, want empty", s)
	}
	if s := unscrambleHash("name", ""); s != "" {
		t.Fatalf("unscrambleHash on blank hash = %q, want empty", s)
	}
}

func TestScrambleHashBlankString(t *testing.T) {
	if s := scrambleHash("", ""); s != "" {
		t.Fatalf("scrambleHash on blank name and hash = %q, want empty", s)
	}
	if s := unscrambleHash("", ""); s != "" {
		t.Fatalf("unscrambleHash on blank name and hash = %q, want empty", s)
	}
}

func TestScrambleHashRoundTrip(t *testing.T) {
	const name = "char"
	const hash = "0123456789abcdef0123456789abcdef"
	enc := scrambleHash(name, hash)
	if enc == hash {
		t.Fatalf("scrambleHash(%q, %q) = %q, expected different", name, hash, enc)
	}
	dec := unscrambleHash(name, enc)
	if dec != hash {
		t.Fatalf("unscrambleHash returned %q, want %q", dec, hash)
	}
}

func TestSaveLoadCharactersAppearanceProfession(t *testing.T) {
	dir := t.TempDir()
	orig := dataDirPath
	origCharacters := characters
	dataDirPath = dir
	defer func() {
		dataDirPath = orig
		characters = origCharacters
	}()

	characters = []Character{{Name: "Hero", PictID: 123, Colors: []byte{1, 2, 3}, Profession: "fighter"}}
	saveCharacters()

	characters = nil
	loadCharacters()
	if len(characters) != 1 {
		t.Fatalf("expected 1 character, got %d", len(characters))
	}
	c := characters[0]
	if c.PictID != 123 {
		t.Fatalf("expected pict 123, got %d", c.PictID)
	}
	if c.Profession != "fighter" {
		t.Fatalf("expected profession fighter, got %q", c.Profession)
	}
	if len(c.Colors) != 3 || c.Colors[0] != 1 || c.Colors[1] != 2 || c.Colors[2] != 3 {
		t.Fatalf("unexpected colors: %v", c.Colors)
	}
}

func TestSaveLoadCharacterWithoutRememberingPassword(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	origCharacters := characters
	dataDirPath = dir
	t.Cleanup(func() {
		dataDirPath = origDir
		characters = origCharacters
	})

	characters = []Character{{
		Name:         "Hero",
		passHash:     "0123456789abcdef0123456789abcdef",
		Key:          "stale-key",
		DontRemember: true,
		PictID:       123,
		Profession:   "fighter",
	}}
	saveCharacters()

	data, err := os.ReadFile(filepath.Join(dir, charsFilePath))
	if err != nil {
		t.Fatalf("read characters: %v", err)
	}
	var saved charactersFile
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("decode characters: %v", err)
	}
	if len(saved.Characters) != 1 {
		t.Fatalf("saved characters = %d, want 1", len(saved.Characters))
	}
	if saved.Characters[0].Key != "" {
		t.Fatalf("saved password key = %q, want empty", saved.Characters[0].Key)
	}

	characters = nil
	loadCharacters()
	if len(characters) != 1 {
		t.Fatalf("loaded characters = %d, want 1", len(characters))
	}
	if characters[0].passHash != "" || !characters[0].DontRemember {
		t.Fatalf("loaded password state = hash %q, dontRemember %v", characters[0].passHash, characters[0].DontRemember)
	}
	if characters[0].PictID != 123 || characters[0].Profession != "fighter" {
		t.Fatalf("character metadata was not preserved: %+v", characters[0])
	}
}

func TestLoadCharacterRejectsInvalidSavedPasswordHash(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	origCharacters := characters
	dataDirPath = dir
	t.Cleanup(func() {
		dataDirPath = origDir
		characters = origCharacters
	})

	data := []byte(`{"version":2,"characters":[{"name":"Hero","key":"not-a-valid-hash"}]}`)
	if err := os.WriteFile(filepath.Join(dir, charsFilePath), data, 0o644); err != nil {
		t.Fatalf("write characters: %v", err)
	}
	loadCharacters()
	if len(characters) != 1 {
		t.Fatalf("loaded characters = %d, want 1", len(characters))
	}
	if characters[0].passHash != "" || characters[0].Key != "" || !characters[0].DontRemember {
		t.Fatalf("invalid password hash was retained: %+v", characters[0])
	}
}

func TestRemoveCharacterClearsLastCharacter(t *testing.T) {
	originalDir := dataDirPath
	originalCharacters := characters
	originalSettings := gs
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		characters = originalCharacters
		gs = originalSettings
	})

	characters = []Character{{Name: "Hero"}}
	gs.LastCharacter = "Hero"

	removeCharacter("Hero")
	if gs.LastCharacter != "" {
		t.Fatalf("removed login remained the last character: %q", gs.LastCharacter)
	}
}

func TestBackfillCharactersFromPlayers(t *testing.T) {
	dir := t.TempDir()
	origDir := dataDirPath
	dataDirPath = dir
	defer func() { dataDirPath = origDir }()

	playersMu.Lock()
	origPlayers := players
	players = map[string]*Player{
		"Hero": {Name: "Hero", PictID: 77, Colors: []byte{4, 5}, Class: "mystic"},
	}
	playersMu.Unlock()
	defer func() {
		playersMu.Lock()
		players = origPlayers
		playersMu.Unlock()
	}()

	origChars := characters
	characters = []Character{{Name: "Hero"}}
	backfillCharactersFromPlayers()
	if len(characters) != 1 {
		t.Fatalf("expected 1 character, got %d", len(characters))
	}
	c := characters[0]
	if c.PictID != 77 {
		t.Fatalf("expected pict 77, got %d", c.PictID)
	}
	if c.Profession != "mystic" {
		t.Fatalf("expected profession mystic, got %q", c.Profession)
	}
	if len(c.Colors) != 2 || c.Colors[0] != 4 || c.Colors[1] != 5 {
		t.Fatalf("unexpected colors: %v", c.Colors)
	}

	characters = origChars
}

func TestLoadLegacyCharactersAssignsFirstServerSlot(t *testing.T) {
	dir := t.TempDir()
	originalDir, originalCharacters, originalSettings := dataDirPath, characters, gs
	dataDirPath = dir
	gs = gsdef
	gs.ServerAddresses = []string{"first.example:5010", "second.example:5010"}
	gs.ServerAddress = gs.ServerAddresses[1]
	t.Cleanup(func() {
		dataDirPath, characters, gs = originalDir, originalCharacters, originalSettings
	})

	data := []byte(`{"version":2,"characters":[{"name":"Legacy Hero"}]}`)
	if err := os.WriteFile(filepath.Join(dir, charsFilePath), data, 0o644); err != nil {
		t.Fatal(err)
	}
	loadCharacters()
	if len(characters) != 1 || characters[0].ServerSlot != 1 {
		t.Fatalf("legacy character slots = %+v, want slot 1", characters)
	}

	savedData, err := os.ReadFile(filepath.Join(dir, charsFilePath))
	if err != nil {
		t.Fatal(err)
	}
	var saved charactersFile
	if err := json.Unmarshal(savedData, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Characters) != 1 || saved.Characters[0].ServerSlot != 1 {
		t.Fatalf("migrated character file = %+v, want slot 1", saved.Characters)
	}
}

func TestCharactersAreScopedAndMovableByServerSlot(t *testing.T) {
	originalCharacters, originalSettings, originalDir := characters, gs, dataDirPath
	dataDirPath = t.TempDir()
	gs = gsdef
	gs.ServerAddresses = []string{"first.example:5010", "second.example:5010"}
	characters = []Character{
		{Name: "Hero", ServerSlot: 1, passHash: "first"},
		{Name: "Hero", ServerSlot: 2, passHash: "second"},
	}
	t.Cleanup(func() {
		characters, gs, dataDirPath = originalCharacters, originalSettings, originalDir
	})

	first, ok := characterForServerSlot(1, "hero")
	if !ok || first.passHash != "first" {
		t.Fatalf("slot 1 character = %+v, %v", first, ok)
	}
	second, ok := characterForServerSlot(2, "HERO")
	if !ok || second.passHash != "second" {
		t.Fatalf("slot 2 character = %+v, %v", second, ok)
	}
	if err := moveCharacterToServerSlot(1, 2, "Hero"); err == nil {
		t.Fatal("moved a duplicate character name into slot 2")
	}

	characters = characters[:1]
	if err := moveCharacterToServerSlot(1, 2, "Hero"); err != nil {
		t.Fatalf("move character: %v", err)
	}
	if _, ok := characterForServerSlot(1, "Hero"); ok {
		t.Fatal("moved character remained in slot 1")
	}
	if moved, ok := characterForServerSlot(2, "Hero"); !ok || moved.ServerSlot != 2 {
		t.Fatalf("moved character = %+v, %v", moved, ok)
	}
}

func TestPrepareEditCharacterSelectsItsServerSlot(t *testing.T) {
	originalCharacters, originalSettings := characters, gs
	originalName, originalPass, originalPassPrev := editCharName, editCharPass, editCharPassPrev
	originalRemember := editCharRemember
	originalSlot, originalSourceSlot := editCharServerSlot, editCharOriginalServerSlot
	originalDropdown := editCharServerDropdown
	gs = gsdef
	gs.ServerAddresses = []string{"first.example:5010", "second.example:5010"}
	characters = []Character{{Name: "Hero", ServerSlot: 2, DontRemember: true}}
	editCharServerDropdown = nil
	t.Cleanup(func() {
		characters, gs = originalCharacters, originalSettings
		editCharName, editCharPass, editCharPassPrev = originalName, originalPass, originalPassPrev
		editCharRemember = originalRemember
		editCharServerSlot, editCharOriginalServerSlot = originalSlot, originalSourceSlot
		editCharServerDropdown = originalDropdown
	})

	if err := prepareEditCharacterForServerSlotAndSession(nil, 2, "Hero"); err != nil {
		t.Fatalf("prepare edit character: %v", err)
	}
	if editCharServerSlot != 2 || editCharOriginalServerSlot != 2 {
		t.Fatalf("edit slots = destination %d, source %d; want 2, 2", editCharServerSlot, editCharOriginalServerSlot)
	}
}
