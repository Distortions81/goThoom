package eui

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed themes/styles/*.json
var embeddedStyles embed.FS

// StyleTheme controls spacing and padding used by widgets.
type StyleNumbers struct {
	Window   float32
	Button   float32
	Text     float32
	Checkbox float32
	Radio    float32
	Input    float32
	Slider   float32
	Dropdown float32
	Tab      float32
}

type StyleBools struct {
	Window   bool
	Button   bool
	Text     bool
	Checkbox bool
	Radio    bool
	Input    bool
	Slider   bool
	Dropdown bool
	Tab      bool
}

type StyleTheme struct {
	SliderValueGap   float32
	DropdownArrowPad float32
	TextPadding      float32

	Fillet        StyleNumbers
	Border        StyleNumbers
	BorderPad     StyleNumbers
	Filled        StyleBools
	Outlined      StyleBools
	ActiveOutline StyleBools
}

var defaultStyle = &StyleTheme{
	SliderValueGap:   16,
	DropdownArrowPad: 8,
	TextPadding:      4,
	Fillet: StyleNumbers{
		Window:   4,
		Button:   8,
		Text:     0,
		Checkbox: 8,
		Radio:    8,
		Input:    4,
		Slider:   4,
		Dropdown: 4,
		Tab:      4,
	},
	Border: StyleNumbers{
		Window:   0,
		Button:   0,
		Text:     0,
		Checkbox: 0,
		Radio:    0,
		Input:    0,
		Slider:   0,
		Dropdown: 0,
		Tab:      0,
	},
	BorderPad: StyleNumbers{
		Window:   0,
		Button:   4,
		Text:     4,
		Checkbox: 4,
		Radio:    4,
		Input:    2,
		Slider:   2,
		Dropdown: 2,
		Tab:      2,
	},
	Filled: StyleBools{
		Window:   true,
		Button:   true,
		Text:     false,
		Checkbox: true,
		Radio:    true,
		Input:    true,
		Slider:   true,
		Dropdown: true,
		Tab:      true,
	},
	Outlined: StyleBools{
		Window:   false,
		Button:   false,
		Text:     false,
		Checkbox: false,
		Radio:    false,
		Input:    false,
		Slider:   false,
		Dropdown: false,
		Tab:      false,
	},
	ActiveOutline: StyleBools{
		Tab: true,
	},
}

var (
	currentStyle     = defaultStyle
	currentStyleName = "RoundHybrid"
)

func LoadStyle(name string) error {
	// Try local filesystem first (relative to executable dir; see paths.go)
	file := filepath.Join("themes", "styles", name+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		// Fallback to embedded styles; embed paths must use forward slashes
		data, err = embeddedStyles.ReadFile(path.Join("themes", "styles", name+".json"))
		if err != nil {
			return err
		}
	}
	if err := json.Unmarshal(data, currentStyle); err != nil {
		return err
	}
	SetCurrentStyleName(name)
	if currentTheme != nil {
		applyStyleToTheme(currentTheme)
		applyStyleToItems(currentTheme)
		markAllDirty()
	}
	refreshStyleMod()
	return nil
}

func applyStyleToTheme(th *Theme) {
	if th == nil || currentStyle == nil {
		return
	}
	th.Window.Fillet = currentStyle.Fillet.Window
	th.Window.Border = currentStyle.Border.Window
	th.Window.BorderPad = currentStyle.BorderPad.Window
	th.Window.Outlined = currentStyle.Outlined.Window

	th.Button.Fillet = currentStyle.Fillet.Button
	th.Button.Border = currentStyle.Border.Button
	th.Button.BorderPad = currentStyle.BorderPad.Button
	th.Button.Filled = currentStyle.Filled.Button
	th.Button.Outlined = currentStyle.Outlined.Button

	th.Text.Fillet = currentStyle.Fillet.Text
	th.Text.Border = currentStyle.Border.Text
	th.Text.BorderPad = currentStyle.BorderPad.Text
	th.Text.Filled = currentStyle.Filled.Text
	th.Text.Outlined = currentStyle.Outlined.Text

	th.Checkbox.Fillet = currentStyle.Fillet.Checkbox
	th.Checkbox.Border = currentStyle.Border.Checkbox
	th.Checkbox.BorderPad = currentStyle.BorderPad.Checkbox
	th.Checkbox.Filled = currentStyle.Filled.Checkbox
	th.Checkbox.Outlined = currentStyle.Outlined.Checkbox

	th.Radio.Fillet = currentStyle.Fillet.Radio
	th.Radio.Border = currentStyle.Border.Radio
	th.Radio.BorderPad = currentStyle.BorderPad.Radio
	th.Radio.Filled = currentStyle.Filled.Radio
	th.Radio.Outlined = currentStyle.Outlined.Radio

	th.Input.Fillet = currentStyle.Fillet.Input
	th.Input.Border = currentStyle.Border.Input
	th.Input.BorderPad = currentStyle.BorderPad.Input
	th.Input.Filled = currentStyle.Filled.Input
	th.Input.Outlined = currentStyle.Outlined.Input

	th.Slider.Fillet = currentStyle.Fillet.Slider
	th.Slider.Border = currentStyle.Border.Slider
	th.Slider.BorderPad = currentStyle.BorderPad.Slider
	th.Slider.Filled = currentStyle.Filled.Slider
	th.Slider.Outlined = currentStyle.Outlined.Slider

	th.Dropdown.Fillet = currentStyle.Fillet.Dropdown
	th.Dropdown.Border = currentStyle.Border.Dropdown
	th.Dropdown.BorderPad = currentStyle.BorderPad.Dropdown
	th.Dropdown.Filled = currentStyle.Filled.Dropdown
	th.Dropdown.Outlined = currentStyle.Outlined.Dropdown

	th.Tab.Fillet = currentStyle.Fillet.Tab
	th.Tab.Border = currentStyle.Border.Tab
	th.Tab.BorderPad = currentStyle.BorderPad.Tab
	th.Tab.Filled = currentStyle.Filled.Tab
	th.Tab.Outlined = currentStyle.Outlined.Tab
	th.Tab.ActiveOutline = currentStyle.ActiveOutline.Tab
}

// applyStyleToItems updates the style-related fields of all existing windows
// and items that use the provided theme so changes take effect immediately.
func applyStyleToItems(th *Theme) {
	for _, win := range windows {
		if win == nil || (win.Theme != nil && win.Theme != th) {
			continue
		}
		win.Fillet = th.Window.Fillet
		win.Border = th.Window.Border
		win.BorderPad = th.Window.BorderPad
		win.Outlined = th.Window.Outlined
		updateItemStyleTree(win.Contents, th)
	}
}

// updateItemStyleTree recursively applies theme style values to an item tree.
func updateItemStyleTree(items []*itemData, th *Theme) {
	for _, it := range items {
		if it == nil || (it.Theme != nil && it.Theme != th) {
			continue
		}
		switch it.ItemType {
		case ITEM_BUTTON:
			it.Fillet = th.Button.Fillet
			it.Border = th.Button.Border
			it.BorderPad = th.Button.BorderPad
			it.Filled = th.Button.Filled
			it.Outlined = th.Button.Outlined
		case ITEM_TEXT:
			it.Fillet = th.Text.Fillet
			it.Border = th.Text.Border
			it.BorderPad = th.Text.BorderPad
			it.Filled = th.Text.Filled
			it.Outlined = th.Text.Outlined
		case ITEM_CHECKBOX:
			it.Fillet = th.Checkbox.Fillet
			it.Border = th.Checkbox.Border
			it.BorderPad = th.Checkbox.BorderPad
			it.Filled = th.Checkbox.Filled
			it.Outlined = th.Checkbox.Outlined
		case ITEM_RADIO:
			it.Fillet = th.Radio.Fillet
			it.Border = th.Radio.Border
			it.BorderPad = th.Radio.BorderPad
			it.Filled = th.Radio.Filled
			it.Outlined = th.Radio.Outlined
		case ITEM_INPUT:
			it.Fillet = th.Input.Fillet
			it.Border = th.Input.Border
			it.BorderPad = th.Input.BorderPad
			it.Filled = th.Input.Filled
			it.Outlined = th.Input.Outlined
		case ITEM_SLIDER:
			it.Fillet = th.Slider.Fillet
			it.Border = th.Slider.Border
			it.BorderPad = th.Slider.BorderPad
			it.Filled = th.Slider.Filled
			it.Outlined = th.Slider.Outlined
		case ITEM_DROPDOWN:
			it.Fillet = th.Dropdown.Fillet
			it.Border = th.Dropdown.Border
			it.BorderPad = th.Dropdown.BorderPad
			it.Filled = th.Dropdown.Filled
			it.Outlined = th.Dropdown.Outlined
		case ITEM_PROGRESS:
			it.Fillet = th.Progress.Fillet
			it.Border = th.Progress.Border
			it.BorderPad = th.Progress.BorderPad
			it.Filled = th.Progress.Filled
			it.Outlined = th.Progress.Outlined
		case ITEM_FLOW:
			if len(it.Tabs) > 0 {
				it.Fillet = th.Tab.Fillet
				it.Border = th.Tab.Border
				it.BorderPad = th.Tab.BorderPad
				it.Filled = th.Tab.Filled
				it.Outlined = th.Tab.Outlined
				it.ActiveOutline = th.Tab.ActiveOutline
			}
		}
		if len(it.Contents) > 0 {
			updateItemStyleTree(it.Contents, th)
		}
		if len(it.Tabs) > 0 {
			updateItemStyleTree(it.Tabs, th)
		}
	}
}

// listStyles returns the available style theme names from the themes directory
func listStyles() ([]string, error) {
	entries, err := fs.ReadDir(embeddedStyles, "themes/styles")
	if err != nil {
		entries, err = os.ReadDir("themes/styles")
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
