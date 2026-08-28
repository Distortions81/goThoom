package eui

import "log"

// init loads the default theme and style. A font source must be provided
// by the application via SetFontSource or EnsureFontSource.
func init() {
	if err := LoadTheme(currentThemeName); err != nil {
		log.Printf("LoadTheme error: %v", err)
	}
	if err := LoadStyle(currentStyleName); err != nil {
		log.Printf("LoadStyle error: %v", err)
	}
}
