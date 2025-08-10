package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
)

const charactersFilePath = "data/characters.json"

// Character holds a saved character name and password hash.
type Character struct {
	Name     string `json:"name"`
	PassHash string `json:"passHash"`
}

var characters []Character

func charactersPath() string {
	return "data/characters.json"
}

// loadCharacters reads the characters.json file if it exists.
func loadCharacters() {

	//handle older client location
	os.Rename("characters.json", charactersFilePath)

	data, err := os.ReadFile(charactersPath())
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &characters)
}

// saveCharacters writes the current character list to characters.json.
func saveCharacters() {
	data, err := json.MarshalIndent(characters, "", "  ")
	if err != nil {
		log.Printf("save characters: %v", err)
		return
	}
	if err := os.WriteFile(charactersPath(), data, 0644); err != nil {
		log.Printf("save characters: %v", err)
	}
}

// rememberCharacter stores the given name and password hash.
func rememberCharacter(name, pass string) {
	h := md5.Sum([]byte(pass))
	hash := hex.EncodeToString(h[:])
	for i := range characters {
		if characters[i].Name == name {
			characters[i].PassHash = hash
			saveCharacters()
			gs.LastCharacter = name
			saveSettings()
			return
		}
	}
	characters = append(characters, Character{Name: name, PassHash: hash})
	saveCharacters()
	gs.LastCharacter = name
	saveSettings()
}

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
