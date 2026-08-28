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

func TestRemoveCharacterClearsProfileEnablement(t *testing.T) {
	originalDir := dataDirPath
	originalCharacters := characters
	originalSettings := gs
	originalProfiles := characterProfiles
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		dataDirPath = originalDir
		characters = originalCharacters
		gs = originalSettings
		characterProfiles = originalProfiles
	})

	characters = []Character{{Name: "Hero"}}
	gs.LastCharacter = "Someone Else"
	characterProfiles = characterProfilesDocument{
		Version: characterProfilesVersion,
		Enabled: map[string]bool{"hero": true},
	}

	removeCharacter("Hero")
	if characterProfileEnabled("Hero") {
		t.Fatal("removed login retained per-character profile enablement")
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
