package eui

import (
	_ "embed"
	"log"
)

//go:embed fonts/NotoSans-Regular-Mini.ttf
var defaultTTF []byte

func init() {
	if err := EnsureFontSource(defaultTTF); err != nil {
		log.Printf("default font load error: %v", err)
	}
	if err := LoadTheme(currentThemeName); err != nil {
		log.Printf("LoadTheme error: %v", err)
	}
	if err := LoadStyle(currentStyleName); err != nil {
		log.Printf("LoadStyle error: %v", err)
	}
}
