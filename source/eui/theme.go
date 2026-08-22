package eui

import (
	"embed"
	"encoding/json"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path"
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
	Progress itemData
	Tab      itemData
}

type themeFile struct {
    Comment          string            `json:"Comment"`
    Colors           map[string]string `json:"Colors"`
    RecommendedStyle string            `json:"RecommendedStyle"`
}

// themeAccentRefs records which theme fields referred to the named "accent"
// color in the source JSON so they can be updated dynamically when the accent
// color changes.
var themeAccentRefs struct {
    WindowActive    bool
    ButtonClick     bool
    TextClick       bool
    CheckboxClick   bool
    RadioClick      bool
    InputClick      bool
    SliderClick     bool
    SliderFilled    bool
    DropdownClick   bool
    DropdownSelect  bool
    TabClick        bool
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
	// Try local filesystem first (relative to executable dir; see paths.go)
	file := filepath.Join("themes", "palettes", name+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		// Fallback to embedded palettes; embed paths must use forward slashes
		data, err = embeddedThemes.ReadFile(path.Join("themes", "palettes", name+".json"))
		if err != nil {
			return err
		}
	}

	// Reset named colors
	namedColors = map[string]Color{}

	oldTheme := currentTheme

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

    // Capture which fields referenced the named "accent" color so we can
    // update them when the user tweaks the accent via the color wheel.
    // We only track a small set of fields that are designed to be accent-driven.
    var refs struct {
        Window struct {
            ActiveColor string `json:"ActiveColor"`
        } `json:"Window"`
        Button struct {
            ClickColor string `json:"ClickColor"`
        } `json:"Button"`
        Text struct {
            ClickColor string `json:"ClickColor"`
        } `json:"Text"`
        Checkbox struct {
            ClickColor string `json:"ClickColor"`
        } `json:"Checkbox"`
        Radio struct {
            ClickColor string `json:"ClickColor"`
        } `json:"Radio"`
        Input struct {
            ClickColor string `json:"ClickColor"`
        } `json:"Input"`
        Slider struct {
            ClickColor   string `json:"ClickColor"`
            SliderFilled string `json:"SliderFilled"`
        } `json:"Slider"`
        Dropdown struct {
            ClickColor    string `json:"ClickColor"`
            SelectedColor string `json:"SelectedColor"`
        } `json:"Dropdown"`
        Tab struct {
            ClickColor string `json:"ClickColor"`
        } `json:"Tab"`
    }
    // Best-effort; ignore errors since not all fields are present in every palette
    _ = json.Unmarshal(data, &refs)
    // Helper to check if a string equals "accent" (case-insensitive, trimmed)
    isAccent := func(s string) bool { return strings.ToLower(strings.TrimSpace(s)) == "accent" }
    themeAccentRefs.WindowActive = isAccent(refs.Window.ActiveColor)
    themeAccentRefs.ButtonClick = isAccent(refs.Button.ClickColor)
    themeAccentRefs.TextClick = isAccent(refs.Text.ClickColor)
    themeAccentRefs.CheckboxClick = isAccent(refs.Checkbox.ClickColor)
    themeAccentRefs.RadioClick = isAccent(refs.Radio.ClickColor)
    themeAccentRefs.InputClick = isAccent(refs.Input.ClickColor)
    themeAccentRefs.SliderClick = isAccent(refs.Slider.ClickColor)
    themeAccentRefs.SliderFilled = isAccent(refs.Slider.SliderFilled)
    themeAccentRefs.DropdownClick = isAccent(refs.Dropdown.ClickColor)
    themeAccentRefs.DropdownSelect = isAccent(refs.Dropdown.SelectedColor)
    themeAccentRefs.TabClick = isAccent(refs.Tab.ClickColor)
    currentTheme = &th
    if extra.Slider.SliderFilled != "" {
        if col, err := resolveColor(extra.Slider.SliderFilled, tf.Colors, map[string]bool{"sliderfilled": true}); err == nil {
            namedColors["sliderfilled"] = col
            currentTheme.Slider.SelectedColor = col
        }
    }
    SetCurrentThemeName(name)
    applyStyleToTheme(currentTheme)
    updateThemeReferences(oldTheme, currentTheme)
    applyStyleToItems(currentTheme)
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

// updateThemeReferences replaces references to old theme with the new theme across
// all active windows and their item trees.
func updateThemeReferences(old, new *Theme) {
	for _, win := range windows {
		if win.Theme == old {
			win.Theme = new
		}
		updateItemThemeTree(win.Contents, old, new)
	}
}

// updateItemThemeTree walks an item tree and updates theme pointers.
func updateItemThemeTree(items []*itemData, old, new *Theme) {
	for _, it := range items {
		if it.Theme == old {
			it.Theme = new
		}
		if len(it.Contents) > 0 {
			updateItemThemeTree(it.Contents, old, new)
		}
		if len(it.Tabs) > 0 {
			updateItemThemeTree(it.Tabs, old, new)
		}
	}
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
    if namedColors != nil {
        namedColors["accent"] = AccentColor()
    }
    // If the active theme used the named accent for certain fields, update
    // those concrete colors so widgets reflect the new accent immediately.
    if currentTheme != nil {
        ac := AccentColor()
        if themeAccentRefs.WindowActive {
            currentTheme.Window.ActiveColor = ac
        }
        if themeAccentRefs.ButtonClick {
            currentTheme.Button.ClickColor = ac
        }
        if themeAccentRefs.TextClick {
            currentTheme.Text.ClickColor = ac
        }
        if themeAccentRefs.CheckboxClick {
            currentTheme.Checkbox.ClickColor = ac
        }
        if themeAccentRefs.RadioClick {
            currentTheme.Radio.ClickColor = ac
        }
        if themeAccentRefs.InputClick {
            currentTheme.Input.ClickColor = ac
        }
        if themeAccentRefs.SliderClick {
            currentTheme.Slider.ClickColor = ac
        }
        if themeAccentRefs.SliderFilled {
            currentTheme.Slider.SelectedColor = ac
        }
        if themeAccentRefs.DropdownClick {
            currentTheme.Dropdown.ClickColor = ac
        }
        if themeAccentRefs.DropdownSelect {
            currentTheme.Dropdown.SelectedColor = ac
        }
        if themeAccentRefs.TabClick {
            currentTheme.Tab.ClickColor = ac
        }
    }
    markAllDirty()
}

// SetAccentSaturation updates the saturation component of the accent color and
// reapplies it to the current theme.
func SetAccentSaturation(s float64) {
    accentSaturation = clamp(s, 0, 1)
    if namedColors != nil {
        namedColors["accent"] = AccentColor()
    }
    // Re-apply to theme fields which referenced accent
    if currentTheme != nil {
        ac := AccentColor()
        if themeAccentRefs.WindowActive {
            currentTheme.Window.ActiveColor = ac
        }
        if themeAccentRefs.ButtonClick {
            currentTheme.Button.ClickColor = ac
        }
        if themeAccentRefs.TextClick {
            currentTheme.Text.ClickColor = ac
        }
        if themeAccentRefs.CheckboxClick {
            currentTheme.Checkbox.ClickColor = ac
        }
        if themeAccentRefs.RadioClick {
            currentTheme.Radio.ClickColor = ac
        }
        if themeAccentRefs.InputClick {
            currentTheme.Input.ClickColor = ac
        }
        if themeAccentRefs.SliderClick {
            currentTheme.Slider.ClickColor = ac
        }
        if themeAccentRefs.SliderFilled {
            currentTheme.Slider.SelectedColor = ac
        }
        if themeAccentRefs.DropdownClick {
            currentTheme.Dropdown.ClickColor = ac
        }
        if themeAccentRefs.DropdownSelect {
            currentTheme.Dropdown.SelectedColor = ac
        }
        if themeAccentRefs.TabClick {
            currentTheme.Tab.ClickColor = ac
        }
    }
    markAllDirty()
}
