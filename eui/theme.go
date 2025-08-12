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
	file := filepath.Join("themes", "palettes", name+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		embeddedPath := filepath.ToSlash(file)
		data, err = embeddedThemes.ReadFile(embeddedPath)
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
	applyThemeToAll()
	markAllDirty()
	if ac, ok := namedColors["accent"]; ok {
		accentHue, accentSaturation, accentValue, accentAlpha = rgbaToHSVA(color.RGBA(ac))
	}
	applyAccentColor()
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
	applyAccentColor()
}

// SetAccentSaturation updates the saturation component of the accent color and
// reapplies it to the current theme.
func SetAccentSaturation(s float64) {
	accentSaturation = clamp(s, 0, 1)
	applyAccentColor()
}

// applyAccentColor composes the accent color from the stored HSV values and
// updates all widgets.
func applyAccentColor() {
	col := Color(hsvaToRGBA(accentHue, accentSaturation, accentValue, accentAlpha))
	namedColors["accent"] = col
	if currentTheme == nil {
		return
	}
	currentTheme.Window.ActiveColor = col
	currentTheme.Button.ClickColor = col
	currentTheme.Checkbox.ClickColor = col
	currentTheme.Radio.ClickColor = col
	currentTheme.Input.ClickColor = col
	currentTheme.Slider.ClickColor = col
	currentTheme.Slider.SelectedColor = col
	currentTheme.Dropdown.ClickColor = col
	currentTheme.Dropdown.SelectedColor = col
	currentTheme.Tab.ClickColor = col
	namedColors["sliderfilled"] = col
	applyThemeToAll()
	updateColorWheels(col)
	markAllDirty()
}

// applyThemeToAll updates all existing windows to use the current theme.
func applyThemeToAll() {
	if currentTheme == nil {
		return
	}
	for _, win := range windows {
		applyThemeToWindow(win)
	}
	for _, ov := range overlays {
		applyThemeToItem(ov)
	}
}

// applyThemeToWindow merges the current theme's window settings into the given
// window and recursively updates contained items.
func copyWindowStyle(dst, src *windowData) {
	dst.Padding = src.Padding
	dst.Margin = src.Margin
	dst.Border = src.Border
	dst.BorderPad = src.BorderPad
	dst.Fillet = src.Fillet
	dst.Outlined = src.Outlined
	if !dst.NoTitleSet {
		dst.NoTitle = src.NoTitle
	}
	if !dst.TitleHeightSet {
		dst.TitleHeight = src.TitleHeight
	}
	dst.DragbarSpacing = src.DragbarSpacing
	dst.ShowDragbar = src.ShowDragbar
}

func applyThemeToWindow(win *windowData) {
	if win == nil || currentTheme == nil {
		return
	}
	copyWindowStyle(win, &currentTheme.Window)
	stripWindowColors(win)
	win.Theme = currentTheme
	for _, item := range win.Contents {
		applyThemeToItem(item)
	}
}

// applyThemeToItem merges style data from the current theme based on item type
// and recursively processes child items.
func copyItemStyle(dst, src *itemData) {
	dst.Padding = src.Padding
	dst.Margin = src.Margin
	dst.Fillet = src.Fillet
	dst.Border = src.Border
	dst.BorderPad = src.BorderPad
	dst.Filled = src.Filled
	dst.Outlined = src.Outlined
	dst.ActiveOutline = src.ActiveOutline
	dst.AuxSize = src.AuxSize
	dst.AuxSpace = src.AuxSpace
	if src.MaxVisible != 0 {
		dst.MaxVisible = src.MaxVisible
	}
}

func applyThemeToItem(it *itemData) {
	if it == nil || currentTheme == nil {
		return
	}
	var src *itemData
	switch it.ItemType {
	case ITEM_FLOW:
		if len(it.Tabs) > 0 {
			src = &currentTheme.Tab
		}
	case ITEM_BUTTON:
		src = &currentTheme.Button
	case ITEM_TEXT:
		src = &currentTheme.Text
	case ITEM_CHECKBOX:
		src = &currentTheme.Checkbox
	case ITEM_RADIO:
		src = &currentTheme.Radio
	case ITEM_INPUT:
		src = &currentTheme.Input
	case ITEM_SLIDER:
		src = &currentTheme.Slider
	case ITEM_DROPDOWN:
		src = &currentTheme.Dropdown
	}
	if src != nil {
		copyItemStyle(it, src)
	}
	stripItemColors(it)
	it.Theme = currentTheme
	for _, child := range it.Contents {
		applyThemeToItem(child)
	}
	for _, tab := range it.Tabs {
		applyThemeToItem(tab)
	}
}

// updateColorWheels sets the WheelColor field of all color wheel widgets to
// the provided color.
func updateColorWheels(col Color) {
	for _, win := range windows {
		updateColorWheelList(win.Contents, col)
	}
	for _, ov := range overlays {
		updateColorWheelList(ov.Contents, col)
	}
}

func updateColorWheelList(items []*itemData, col Color) {
	for _, it := range items {
		if it.ItemType == ITEM_COLORWHEEL {
			it.WheelColor = col
		}
		updateColorWheelList(it.Contents, col)
		for _, tab := range it.Tabs {
			updateColorWheelList(tab.Contents, col)
		}
	}
}
