package main

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Character holds a saved character name and password hash. The hash is stored
// on disk using a reversible scrambling to avoid exposing the raw hash.
type Character struct {
	Name         string         `json:"name"`
	passHash     string         `json:"-"`
	Key          string         `json:"key"`
	DontRemember bool           `json:"-"`
	PictID       uint16         `json:"pict,omitempty"`
	ColorsHex    string         `json:"colors,omitempty"`
	Colors       []byte         `json:"-"`
	Profession   string         `json:"prof,omitempty"`
	Labels       map[string]int `json:"labels,omitempty"`
}

var characters []Character

const (
	charsFilePath = "characters.json"
	hashKey       = "3k6XsAgldtz1vRw3e9WpfUtXQdKQO4P7a7dxmda4KTNpEJWu0lk08QEcJTbeqisH"
	agratisPrefix = "Agratis "
)

type charactersFile struct {
	Version    int         `json:"version"`
	Characters []Character `json:"characters"`
}

func loadCharacters() {
	data, err := os.ReadFile(filepath.Join(dataDirPath, charsFilePath))
	if err != nil {
		return
	}

	var charList charactersFile
	if err := json.Unmarshal(data, &charList); err != nil {
		return
	}
	if charList.Version >= 1 {
		var filtered []Character
		for _, c := range charList.Characters {
			if strings.HasPrefix(c.Name, agratisPrefix) {
				continue
			}
			c.passHash = unscrambleHash(c.Name, c.Key)
			if charList.Version >= 2 && c.ColorsHex != "" {
				if b, ok := decodeHex(c.ColorsHex); ok && len(b) > 0 {
					cnt := int(b[0])
					if cnt > 0 && 1+cnt <= len(b) {
						c.Colors = append(c.Colors[:0], b[1:1+cnt]...)
					} else {
						c.Colors = append(c.Colors[:0], b...)
					}
				}
			}
			filtered = append(filtered, c)
		}
		characters = filtered
	}
}

func saveCharacters() {
	if isWASM {
		// Skip persistence in WASM.
		return
	}
	var persisted []Character
	for i := range characters {
		if characters[i].DontRemember || strings.HasPrefix(characters[i].Name, agratisPrefix) {
			continue
		}
		characters[i].Key = scrambleHash(characters[i].Name, characters[i].passHash)
		if len(characters[i].Colors) > 0 {
			buf := make([]byte, 1+len(characters[i].Colors))
			if len(characters[i].Colors) > 255 {
				buf[0] = 255
				copy(buf[1:], characters[i].Colors[:255])
			} else {
				buf[0] = byte(len(characters[i].Colors))
				copy(buf[1:], characters[i].Colors)
			}
			characters[i].ColorsHex = encodeHex(buf)
		} else {
			characters[i].ColorsHex = ""
		}
		persisted = append(persisted, characters[i])
	}

	var charList charactersFile
	charList.Version = 2
	charList.Characters = persisted
	data, err := json.MarshalIndent(charList, "", "  ")

	if err != nil {
		log.Printf("save characters: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dataDirPath, charsFilePath), data, 0644); err != nil {
		log.Printf("save characters: %v", err)
	}
}

// backfillCharactersFromPlayers populates missing appearance and profession
// information for saved characters using data loaded from GT_Players.json.
// If any character is updated, the characters file is saved.
func backfillCharactersFromPlayers() {
	playersMu.RLock()
	changed := false
	for i := range characters {
		p, ok := players[characters[i].Name]
		if !ok || p == nil {
			continue
		}
		if characters[i].PictID == 0 && p.PictID != 0 {
			characters[i].PictID = p.PictID
			changed = true
		}
		if len(characters[i].Colors) == 0 && len(p.Colors) > 0 {
			characters[i].Colors = append(characters[i].Colors[:0], p.Colors...)
			changed = true
		}
		if characters[i].Profession == "" && p.Class != "" {
			characters[i].Profession = p.Class
			changed = true
		}
	}
	playersMu.RUnlock()
	if changed {
		saveCharacters()
	}
}

// scrambleHash obscures a hex-encoded hash by XORing with a key derived from a
// hardcoded value and the character name.
func scrambleHash(name, h string) string {
	b, err := hex.DecodeString(h)
	if err != nil {
		return h
	}
	k := []byte(hashKey + name)
	for i := range b {
		b[i] ^= k[i%len(k)]
	}
	return hex.EncodeToString(b)
}

// unscrambleHash reverses the operation of scrambleHash.
func unscrambleHash(name, h string) string { return scrambleHash(name, h) }

// removeCharacter deletes a stored character by name.
func removeCharacter(name string) {
	for i, c := range characters {
		if c.Name == name {
			characters = append(characters[:i], characters[i+1:]...)
			saveCharacters()
			if gs.LastCharacter == name {
				gs.LastCharacter = ""
				saveSettings()
			}
			return
		}
	}
}

// setCharacterPassHash updates the stored password hash and remember flag for the
// given character name and persists the change to disk.
func setCharacterPassHash(charName, hash string, remember bool) {
	if charName == "" {
		return
	}
	for i := range characters {
		if characters[i].Name != charName {
			continue
		}
		characters[i].passHash = hash
		characters[i].DontRemember = !remember
		if !remember {
			characters[i].Key = ""
		}
		saveCharacters()
		return
	}
}
