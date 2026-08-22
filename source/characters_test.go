package main

import "testing"

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
	dataDirPath = dir
	defer func() { dataDirPath = orig }()

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
