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
	// AutoReload enables automatic reloading of theme and style files
	// when they are modified on disk, only use this for quickly iterating when designing your own themes.
	AutoReload bool
)

func init() {
	modCheckTime = time.Now()
	refreshThemeMod()
	refreshStyleMod()
}

func refreshThemeMod() {
	path := filepath.Join("themes", "palettes", currentThemeName+".json")
	if info, err := os.Stat(path); err == nil {
		themeModTime = info.ModTime()
	} else {
		themeModTime = time.Time{}
	}
}

func refreshStyleMod() {
	path := filepath.Join("themes", "styles", currentStyleName+".json")
	if info, err := os.Stat(path); err == nil {
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
	path := filepath.Join("themes", "palettes", currentThemeName+".json")
	if info, err := os.Stat(path); err == nil {
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

	path = filepath.Join("themes", "styles", currentStyleName+".json")
	if info, err := os.Stat(path); err == nil {
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
