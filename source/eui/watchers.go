package eui

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	themeModTime time.Time
	styleModTime time.Time
	modCheckTime time.Time
	themePath    string
	stylePath    string
	// AutoReload enables automatic reloading of theme and style files
	// when they are modified on disk, only use this for quickly iterating when designing your own themes.
	AutoReload bool
)

func init() {
	modCheckTime = time.Now()
	updateThemePath()
	updateStylePath()
	refreshThemeMod()
	refreshStyleMod()
}

func updateThemePath() {
	themePath = filepath.Join(os.Getenv("PWD"), "themes", "palettes", currentThemeName+".json")
}

func refreshThemeMod() {
	if info, err := os.Stat(themePath); err == nil {
		themeModTime = info.ModTime()
	} else {
		themeModTime = time.Time{}
	}
}

func updateStylePath() {
	stylePath = filepath.Join(os.Getenv("PWD"), "themes", "styles", currentStyleName+".json")
}

func refreshStyleMod() {
	if info, err := os.Stat(stylePath); err == nil {
		styleModTime = info.ModTime()
	} else {
		styleModTime = time.Time{}
	}
}

func checkThemeStyleMods() {
	if !AutoReload {
		return
	}
	if time.Since(modCheckTime) < 500*time.Millisecond {
		return
	}
	modCheckTime = time.Now()
	if info, err := os.Stat(themePath); err == nil {
		if info.ModTime().After(themeModTime) {
			log.Println("Palette reload")
			if err := LoadTheme(currentThemeName); err != nil {
				log.Printf("Auto reload theme error: %v", err)
			}
			themeModTime = info.ModTime()
		}
	} else {
		log.Println("Unable to stat " + currentThemeName + ": " + err.Error())
	}

	if info, err := os.Stat(stylePath); err == nil {
		if info.ModTime().After(styleModTime) {
			log.Println("Style theme reload")
			if err := LoadStyle(currentStyleName); err != nil {
				log.Printf("Auto reload style error: %v", err)
			}
			styleModTime = info.ModTime()
		}
	} else {
		log.Println("Unable to stat " + currentStyleName + ": " + err.Error())
	}

}
