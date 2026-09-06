package eui

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func isolateThemeTest(t *testing.T) {
	t.Helper()
	oldTheme, oldStyle := currentTheme, currentStyle
	oldColors, oldRefs := namedColors, themeAccentRefs
	oldWindows, oldDirectory := windows, themeDirectory
	oldThemeName, oldStyleName := currentThemeName, currentStyleName
	h, s, v, a := accentHue, accentSaturation, accentValue, accentAlpha
	windows = nil
	if currentTheme != nil {
		copy := *currentTheme
		currentTheme = &copy
	}
	themeDirectory = t.TempDir()
	t.Cleanup(func() {
		currentTheme, currentStyle = oldTheme, oldStyle
		namedColors, themeAccentRefs = oldColors, oldRefs
		windows, themeDirectory = oldWindows, oldDirectory
		accentHue, accentSaturation, accentValue, accentAlpha = h, s, v, a
		SetCurrentThemeName(oldThemeName)
		SetCurrentStyleName(oldStyleName)
		refreshThemeMod()
		refreshStyleMod()
	})
}

func TestReadableTextAcrossAccentColors(t *testing.T) {
	for r := 0; r <= 255; r += 17 {
		for g := 0; g <= 255; g += 17 {
			for b := 0; b <= 255; b += 17 {
				bg := NewColor(uint8(r), uint8(g), uint8(b), 255)
				for _, preferred := range []Color{bg, ColorWhite, ColorBlack, NewColor(140, 150, 160, 255)} {
					got := readableTextColor(preferred, bg)
					if ratio := textContrast(got, bg); ratio < 4.5 {
						t.Fatalf("foreground %v on %v has contrast %.2f", got, bg, ratio)
					}
					if textContrast(preferred, bg) >= 4.5 && got != preferred {
						t.Fatalf("readable palette color %v was replaced by %v", preferred, got)
					}
				}
			}
		}
	}
}

func TestBundledPalettesHaveReadableSurfaces(t *testing.T) {
	isolateThemeTest(t)
	entries, err := embeddedThemes.ReadDir("themes/palettes")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()[:len(entry.Name())-len(".json")]
		if name == "Example" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if err := LoadTheme(name); err != nil {
				t.Fatal(err)
			}
			// Every recommendation must name an actual style, even though LoadTheme
			// tolerates an unavailable recommendation in user-authored palettes.
			data, err := embeddedThemes.ReadFile("themes/palettes/" + entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			var metadata themeFile
			if err := json.Unmarshal(data, &metadata); err != nil {
				t.Fatal(err)
			}
			if _, err := embeddedStyles.ReadFile("themes/styles/" + metadata.RecommendedStyle + ".json"); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []struct {
				name  string
				style *itemData
			}{
				{"Button", &currentTheme.Button}, {"Input", &currentTheme.Input}, {"Dropdown", &currentTheme.Dropdown}, {"Tab", &currentTheme.Tab},
			} {
				for _, background := range []Color{kind.style.Color, kind.style.HoverColor, currentTheme.Window.BGColor} {
					if ratio := textContrast(kind.style.TextColor, background); ratio < 4.5 {
						t.Errorf("%s normal/hover/window text contrast %.2f", kind.name, ratio)
					}
				}
				disabled := disabledStyle(kind.style)
				if ratio := textContrast(disabled.TextColor, disabled.Color); ratio < 3 {
					t.Errorf("%s disabled caption contrast %.2f", kind.name, ratio)
				}
			}
			for _, accent := range []Color{ColorBlack, ColorWhite, ColorYellow, ColorBlue} {
				_, saturation, _, _ := rgbaToHSVA(color.RGBA(accent))
				SetAccentSaturation(saturation)
				SetAccentColor(accent)
				if ratio := textContrast(readableTextColor(currentTheme.Button.TextColor, currentTheme.Button.ClickColor), currentTheme.Button.ClickColor); ratio < 4.5 {
					t.Errorf("custom accent contrast %.2f", ratio)
				}
			}
		})
	}
}

func TestPartialStyleDoesNotInheritPreviousStyle(t *testing.T) {
	isolateThemeTest(t)
	dir := filepath.Join(themeDirectory, "styles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Partial.json"), []byte(`{"Fillet":{"Button":5}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"HighContrast", "Rounded"} {
		if err := LoadStyle(name); err != nil {
			t.Fatal(err)
		}
		if err := LoadStyle("Partial"); err != nil {
			t.Fatal(err)
		}
		want := baseStyle
		want.Fillet.Button = 5
		if *currentStyle != want {
			t.Fatalf("partial style inherited values from %s", name)
		}
	}
	before := *currentStyle
	if err := os.WriteFile(filepath.Join(dir, "Broken.json"), []byte(`{"TextPadding":99,"Fillet":{"Button":"invalid"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadStyle("Broken"); err == nil {
		t.Fatal("invalid style was accepted")
	}
	if *currentStyle != before || CurrentStyleName() != "Partial" {
		t.Fatal("failed style load changed the active style")
	}
}

func TestInputFillFollowsPaletteWithoutReplacingOverrides(t *testing.T) {
	isolateThemeTest(t)
	if err := LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	old := currentTheme
	inherited, _ := NewInput()
	custom, _ := NewInput()
	custom.Color = NewColor(70, 20, 60, 255)
	custom.OutlineColor = NewColor(90, 100, 80, 255)
	if err := LoadTheme("AccentLight"); err != nil {
		t.Fatal(err)
	}
	updateItemThemeTree([]*itemData{inherited, custom}, old, currentTheme)
	if inherited.Color != currentTheme.Input.Color {
		t.Fatal("input retained the previous palette's fill")
	}
	if inherited.OutlineColor != currentTheme.Input.OutlineColor || custom.OutlineColor != NewColor(90, 100, 80, 255) {
		t.Fatal("palette switch failed to update inherited outlines or preserve custom outlines")
	}
	if custom.Color != NewColor(70, 20, 60, 255) {
		t.Fatal("explicit input fill was replaced")
	}
}

func TestUnfilledControlUsesWindowContrast(t *testing.T) {
	isolateThemeTest(t)
	if err := LoadTheme("AccentLight"); err != nil {
		t.Fatal(err)
	}
	item, _ := NewButton()
	item.Filled = false
	if got := item.surfaceTextColor(currentTheme.Button.TextColor, ColorBlack, false); got != currentTheme.Button.TextColor {
		t.Fatal("unfilled button used the unpainted highlight color")
	}
}

func TestRetiredThemeNamesAndLocalOverrides(t *testing.T) {
	isolateThemeTest(t)
	for old, replacement := range paletteAliases {
		if err := LoadTheme(old); err != nil || CurrentThemeName() != replacement {
			t.Fatalf("palette %s -> %s: %v", old, CurrentThemeName(), err)
		}
	}
	for old, replacement := range styleAliases {
		if err := LoadStyle(old); err != nil || CurrentStyleName() != replacement {
			t.Fatalf("style %s -> %s: %v", old, CurrentStyleName(), err)
		}
	}
	if err := LoadTheme("NeonNight"); err != nil || CurrentThemeName() != "NeonNight" {
		t.Fatalf("NeonNight lost its identity: %v", err)
	}
	for _, dir := range []string{"palettes", "styles"} {
		if err := os.MkdirAll(filepath.Join(themeDirectory, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(themeDirectory, dir, "Example.json"), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, list := range []func() ([]string, error){ListThemes, ListStyles} {
		names, err := list()
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if name == "Example" || paletteAliases[name] != "" || styleAliases[name] != "" {
				t.Errorf("template or retired name %s appeared in selector", name)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(themeDirectory, "palettes", "Black.json"), []byte(`{"Window":{"BGColor":"#123456ff"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadTheme("Black"); err != nil || CurrentThemeName() != "Black" || currentTheme.Window.BGColor != NewColor(18, 52, 86, 255) {
		t.Fatalf("custom old-name palette did not take precedence: %v", err)
	}
	if err := os.WriteFile(filepath.Join(themeDirectory, "styles", "RoundFlat.json"), []byte(`{"TextPadding":19}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LoadStyle("RoundFlat"); err != nil || CurrentStyleName() != "RoundFlat" || currentStyle.TextPadding != 19 {
		t.Fatalf("custom old-name style did not take precedence: %v", err)
	}
}

func TestNonTextWidgetsRenderWithoutCaptionStyle(t *testing.T) {
	isolateThemeTest(t)
	if err := LoadTheme("AccentLight"); err != nil {
		t.Fatal(err)
	}
	wheel, _ := NewColorWheel()
	for _, item := range []*itemData{wheel, {ItemType: ITEM_IMAGE}} {
		if item.themeStyle() != nil {
			t.Fatal("test requires a widget without a caption style")
		}
		canvas := ebiten.NewImage(48, 48)
		item.Size = point{48, 48}
		item.drawItemInternal(point{}, point{}, point{48, 48}, rect{0, 0, 48, 48}, canvas)
		canvas.Deallocate()
	}
}

func TestExistingControlsFollowEveryPaletteAfterNeonNight(t *testing.T) {
	isolateThemeTest(t)
	if err := LoadTheme("NeonNight"); err != nil {
		t.Fatal(err)
	}
	win := NewWindow()
	constructors := []func() (*itemData, *EventHandler){NewButton, NewInput, NewText, NewDropdown, NewSlider, NewCheckbox, NewRadio, NewProgressBar}
	for _, makeControl := range constructors {
		control, _ := makeControl()
		win.AddItem(control)
	}
	windows = []*windowData{win}
	names, err := ListThemes()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range append(names, "NeonNight", "AccentDark") {
		if err := LoadTheme(name); err != nil {
			t.Fatal(err)
		}
		for i, makeControl := range constructors {
			fresh, _ := makeControl()
			existing := win.Contents[i]
			if existing.Theme != currentTheme || existing.OutlineColor != fresh.OutlineColor || existing.Color != fresh.Color {
				t.Fatalf("%s control %v retained colors from a previous palette", name, existing.ItemType)
			}
		}
		if currentTheme.Progress.SelectedColor != namedColors["accent"] {
			t.Fatalf("%s progress retained a fixed accent", name)
		}
		SetAccentSaturation(0.3)
		SetAccentColor(NewColor(150, 100, 180, 255))
		if currentTheme.Progress.SelectedColor != AccentColor() {
			t.Fatal("progress ignored custom accent")
		}
	}
}
