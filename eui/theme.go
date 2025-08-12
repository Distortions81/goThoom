package eui

import (
	"embed"
	"encoding/json"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed themes/palettes/*.json
var embeddedThemes embed.FS

// Theme bundles all style information for windows and widgets.
type Theme struct {
	Window   windowData
	Button   itemData
	Text     itemData
	Checkbox itemData
	Radio    itemData
	Input    itemData
	Slider   itemData
	Dropdown itemData
	Tab      itemData
}

type themeFile struct {
	Comment          string            `json:"Comment"`
	Colors           map[string]string `json:"Colors"`
	RecommendedStyle string            `json:"RecommendedStyle"`
}

// resolveColor recursively resolves string references to colors after the
// theme JSON has been parsed. Color strings may reference other named colors
// from the same file.
func resolveColor(s string, colors map[string]string, seen map[string]bool) (Color, error) {
	s = strings.TrimSpace(s)
	key := strings.ToLower(s)
	if c, ok := namedColors[key]; ok {
		return c, nil
	}
	if val, ok := colors[key]; ok {
		if seen[key] {
			return Color{}, fmt.Errorf("color reference cycle for %s", key)
		}
		seen[key] = true
		c, err := resolveColor(val, colors, seen)
		if err != nil {
			return Color{}, err
		}
		namedColors[key] = c
		return c, nil
	}
	var c Color
	if err := c.UnmarshalJSON([]byte(strconv.Quote(s))); err != nil {
		return Color{}, err
	}
	return c, nil
}

// LoadTheme reads a theme JSON file from the themes directory and
// sets it as the current theme without modifying existing windows.
func LoadTheme(name string) error {
	file := filepath.Join(os.Getenv("PWD")+"/themes", "palettes", name+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		data, err = embeddedThemes.ReadFile(filepath.Join("themes", "palettes", name+".json"))
		if err != nil {
			return err
		}
	}

	// Reset named colors
	namedColors = map[string]Color{}

	var tf themeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return err
	}
	for n, v := range tf.Colors {
		c, err := resolveColor(v, tf.Colors, map[string]bool{strings.ToLower(n): true})
		if err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		namedColors[strings.ToLower(n)] = c
	}

	// Start with the compiled in defaults
	th := *baseTheme
	if err := json.Unmarshal(data, &th); err != nil {
		return err
	}
	// Extract additional color fields not present in Theme struct
	var extra struct {
		Slider struct {
			SliderFilled string `json:"SliderFilled"`
		} `json:"Slider"`
	}
	_ = json.Unmarshal(data, &extra)
	currentTheme = &th
	if extra.Slider.SliderFilled != "" {
		if col, err := resolveColor(extra.Slider.SliderFilled, tf.Colors, map[string]bool{"sliderfilled": true}); err == nil {
			namedColors["sliderfilled"] = col
			currentTheme.Slider.SelectedColor = col
		}
	}
	currentThemeName = name
	applyStyleToTheme(currentTheme)
	markAllDirty()
	if ac, ok := namedColors["accent"]; ok {
		accentHue, accentSaturation, accentValue, accentAlpha = rgbaToHSVA(color.RGBA(ac))
	}
	if tf.RecommendedStyle != "" {
		_ = LoadStyle(tf.RecommendedStyle)
	}
	refreshThemeMod()
	return nil
}

// listThemes returns the available theme names from the themes directory
func listThemes() ([]string, error) {
	entries, err := fs.ReadDir(embeddedThemes, "themes/palettes")
	if err != nil {
		entries, err = os.ReadDir("themes/palettes")
		if err != nil {
			return nil, err
		}
	}
	names := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// SaveTheme writes the current theme to a JSON file with the given name.
func SaveTheme(name string) error {
	if name == "" {
		return fmt.Errorf("theme name required")
	}
	data, err := json.MarshalIndent(currentTheme, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll("themes/palettes", 0755); err != nil {
		return err
	}
	file := filepath.Join("themes", "palettes", name+".json")
	if err := os.WriteFile(file, data, 0644); err != nil {
		return err
	}
	return nil
}

// SetAccentColor updates the accent color in the current theme and applies it
// to all windows and widgets.
func SetAccentColor(c Color) {
	accentHue, _, accentValue, accentAlpha = rgbaToHSVA(color.RGBA(c))
}

// SetAccentSaturation updates the saturation component of the accent color and
// reapplies it to the current theme.
func SetAccentSaturation(s float64) {
	accentSaturation = clamp(s, 0, 1)
}
