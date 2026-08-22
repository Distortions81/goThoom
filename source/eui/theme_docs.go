package eui

import (
	_ "embed"
	"errors"
	"log"
	"os"
	"path/filepath"
)

//go:embed themes/README.md
var themeReadme []byte

func ensureThemeDocs() {
	// README
	path := filepath.Join("themes", "README.md")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll("themes", 0755); err == nil {
			if err := os.WriteFile(path, themeReadme, 0644); err != nil {
				log.Printf("write theme README: %v", err)
			}
		}
	}

	// example palette
	palette := filepath.Join("themes", "palettes", "Example.json")
	if _, err := os.Stat(palette); errors.Is(err, os.ErrNotExist) {
		if data, err := embeddedThemes.ReadFile("themes/palettes/Example.json"); err == nil {
			if err := os.MkdirAll(filepath.Dir(palette), 0755); err == nil {
				if err := os.WriteFile(palette, data, 0644); err != nil {
					log.Printf("write example palette: %v", err)
				}
			}
		}
	}

	// example style
	style := filepath.Join("themes", "styles", "Example.json")
	if _, err := os.Stat(style); errors.Is(err, os.ErrNotExist) {
		if data, err := embeddedStyles.ReadFile("themes/styles/Example.json"); err == nil {
			if err := os.MkdirAll(filepath.Dir(style), 0755); err == nil {
				if err := os.WriteFile(style, data, 0644); err != nil {
					log.Printf("write example style: %v", err)
				}
			}
		}
	}
}
