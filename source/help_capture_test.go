package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

// Run alone through build-scripts/build_help.sh. Uses real window constructors
// with defaults and invented examples; never logs in or reads player profiles.
func TestCaptureHelp(t *testing.T) {
	dir := os.Getenv("GOTHOOM_CAPTURE_HELP")
	if dir == "" {
		t.Skip("set GOTHOOM_CAPTURE_HELP to export the illustrated UI reference")
	}
	oldDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = oldDir })
	gs = gsdef
	status.NeedPiper, status.NeedPiperFem, status.NeedPiperMale = true, true, true
	initFont()
	eui.SetScreenSize(1200, 1000)
	eui.SetUIScale(1)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	if err := eui.LoadStyle("Breeze"); err != nil {
		t.Fatal(err)
	}
	isolateScriptWorld(t)
	isolateScriptScopeSelection(t)
	name, playerName, gs.LastCharacter = "Alpha", "", "Alpha"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	g := &helpCaptureGame{dir: dir, t: t}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
	manifest := struct {
		Version   int          `json:"version"`
		CLVersion int          `json:"clVersion"`
		Screens   []helpScreen `json:"screens"`
	}{appVersion, clVersion, g.screens}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "capture.json"), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

type helpControl struct {
	Label    string   `json:"label"`
	Help     string   `json:"help,omitempty"`
	Kind     string   `json:"kind"`
	Value    string   `json:"value,omitempty"`
	Options  []string `json:"options,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	Rect     [4]int   `json:"rect"`
}
type helpScreen struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Route    string        `json:"route"`
	Width    int           `json:"width"`
	Height   int           `json:"height"`
	Controls []helpControl `json:"controls"`
}
type helpCaptureGame struct {
	dir     string
	t       *testing.T
	done    bool
	err     error
	screens []helpScreen
}

func (g *helpCaptureGame) Layout(_, _ int) (int, int) { return 1200, 1000 }
func (g *helpCaptureGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *helpCaptureGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	loadMaterialIcons()
	makeSettingsWindow()
	tabs := settingsWin.Contents[0]
	for index, tab := range tabs.Tabs {
		tabs.ActiveTab = index
		if !g.capture(screen, settingsWin, "settings-"+helpSlug(tab.Name), "Settings: "+tab.Name, "Settings → "+tab.Name) {
			return
		}
		if tab.Name == "Performance" {
			var visit func([]*eui.ItemData)
			visit = func(items []*eui.ItemData) {
				for _, it := range items {
					if len(it.Tabs) > 0 {
						for j, sub := range it.Tabs {
							it.ActiveTab = j
							if !g.capture(screen, settingsWin, "performance-"+helpSlug(sub.Name), "Performance: "+sub.Name, "Settings → Performance → "+sub.Name) {
								return
							}
						}
						it.ActiveTab = 0
					} else {
						visit(it.Contents)
					}
				}
			}
			visit(tab.Contents)
		}
	}
	settingsWin.Close()
	makeTileLayoutWindow()
	if !g.capture(screen, tileLayoutWin, "tiled-layout", "Arrange tiled windows", "Settings → Display → Window Layout") {
		return
	}
	tileLayoutWin.RemoveWindow()
	tileLayoutWin = nil
	gs.MessagesToConsole = false
	gs.TiledLayout = TiledLayoutMessagesBelow
	makeTileLayoutWindow()
	if !g.capture(screen, tileLayoutWin, "tiled-separate", "Separate message panes", "Settings → Display → Window Layout") {
		return
	}
	tileLayoutWin.Close()
	gs = gsdef
	makeAddCharacterWindow()
	if !g.capture(screen, addCharWin, "add-character", "Add a character", "Login → Add Character") {
		return
	}
	addCharWin.Close()
	makeMixerWindow()
	if !g.capture(screen, mixerWin, "mixer", "Audio mixer", "Toolbar → Audio") {
		return
	}
	mixerWin.Close()
	makeNotificationsWindow()
	if !g.capture(screen, notificationsWin, "notifications", "Notification settings", "Settings → Audio → Notification Settings") {
		return
	}
	notificationsWin.Close()
	makeBubbleWindow()
	if !g.capture(screen, bubbleWin, "speech-bubbles", "Speech bubble settings", "Settings → Bubbles → Message Bubbles") {
		return
	}
	bubbleWin.Close()
	makeSnapshotWindow()
	snapshotName.Text = "A day in the Lands"
	if !g.capture(screen, snapshotWin, "snapshot", "Take a screenshot", "Tools → Snap") {
		return
	}
	snapshotWin.Close()

	makeFilePathsWindow()
	for kind, display := range filePathsDisplays {
		display.Text = "Default user data location — " + storagePathName(kind)
		display.SetTooltip(display.Text)
	}
	if !g.capture(screen, filePathsWin, "file-paths", "Change file locations", "Settings → Files → File Paths") {
		return
	}
	filePathsWin.Close()
	openHotkeyEditor(-1)
	hotkeyNameInput.Text = "Info about selected player"
	hotkeyComboText.Text = "Ctrl-I"
	hotkeyCmdInputs[0].Text = "/info @selected.player"
	if !g.capture(screen, hotkeyEditWin, "hotkey-editor", "Create a hotkey", "Actions → Hotkeys → +") {
		return
	}
	hotkeyEditWin.Close()
	openShortcutEditor("global")
	shortcutShortInp.Text = "pp"
	shortcutFullInp.Text = "/ponder "
	if !g.capture(screen, shortcutEditWin, "shortcut-editor", "Create a text shortcut", "Actions → Shortcuts → Add For All") {
		return
	}
	shortcutEditWin.Close()
	makeJoystickWindow()
	if !g.capture(screen, joystickWin, "gamepad", "Gamepad (work in progress)", "Settings → Controls → Gamepad") {
		return
	}
	joystickWin.Close()

	macroPath := filepath.Join(legacyMacroLibraryPath(), "basic-commands.mac")
	if err := os.MkdirAll(filepath.Dir(macroPath), 0755); err != nil {
		g.err = err
		return
	}
	macroText := `// Name: Basic Commands
// Desc: A few everyday commands and keys.

// Look around without retyping the command.
"/lookaround" "/look\r"
'pp' "/ponder "
f6 "/look\r"
control-click2 "/info @click.name\r"
`
	if err := os.WriteFile(macroPath, []byte(macroText), 0644); err != nil {
		g.err = err
		return
	}
	makeLegacyMacroLibraryWindow()
	if !g.capture(screen, legacyMacroLibraryWin, "legacy-macros", "Legacy macro library", "Actions → Legacy Macros") {
		return
	}
	legacyMacroLibraryWin.Close()
	macroEntry := legacyMacroLibraryEntry{ID: "basic-commands.mac", Path: macroPath, Name: "Basic Commands"}
	for _, capture := range []struct {
		keys             bool
		id, title, route string
	}{
		{false, "macro-commands", "Edit macro command triggers", "Actions → Legacy Macros → Commands"},
		{true, "macro-keybinds", "Edit macro keybindings", "Actions → Legacy Macros → Keybinds"},
	} {
		win := openLegacyTriggerEditor(macroEntry, capture.keys)
		if win == nil {
			g.err = fmt.Errorf("could not open %s", capture.id)
			return
		}
		if !g.capture(screen, win, capture.id, capture.title, capture.route) {
			return
		}
		win.Close()
	}
	const owner = "yes-boats"
	src, err := scriptScripts.ReadFile(bundledScriptDir + "/yes_boats.go")
	if err != nil {
		g.err = err
		return
	}
	scriptPackages[owner] = scriptInfo{id: owner, name: "Yes Boats", src: src}
	scriptDisplayNames[owner] = "Yes Boats"
	scriptPaths[owner] = "Scripts/yes_boats.go"
	scriptDescriptions[owner] = "Whisper yes when a ferryman offers your character a ride."
	scriptAuthors[owner] = "Example"
	scriptAPIVersions[owner] = 2
	scriptCategories[owner] = "Quality Of Life"
	scriptEnabledFor[owner] = scriptScope{Chars: map[string]bool{"Alpha": true}}
	scriptDisabled[owner] = true
	makescriptsWindow()
	if !g.capture(screen, scriptsWin, "scripts", "Enable scripts for a character", "Actions → Scripts") {
		return
	}
	scriptsWin.Close()
	selectscript(owner)
	if !g.capture(screen, scriptInfoWin, "script-info", "Review a script", "Actions → Scripts → Info") {
		return
	}
	scriptInfoWin.Close()
	openScriptPermissionsWindow(owner)
	if !g.capture(screen, scriptPermissionsWin, "permissions", "Grant script permissions", "Actions → Scripts → Info → Permissions") {
		return
	}
	scriptPermissionsWin.Close()
}
func helpSlug(s string) string {
	return strings.Trim(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, strings.ToLower(s)), "-")
}
func (g *helpCaptureGame) capture(screen *ebiten.Image, win *eui.WindowData, id, title, route string) bool {
	if g.err != nil {
		return false
	}
	win.MarkOpen()
	_ = win.SetPos(eui.Point{X: 32, Y: 32})
	win.Refresh()
	win.Dirty = true
	screen.Fill(color.RGBA{R: 8, G: 16, B: 25, A: 255})
	eui.Draw(screen)
	pos, size := win.GetPos(), win.GetSize()
	windowBounds := image.Rect(int(pos.X), int(pos.Y), int(pos.X+size.X), int(pos.Y+size.Y))
	bounds := windowBounds.Inset(-8).Intersect(screen.Bounds())
	if !windowBounds.In(screen.Bounds()) {
		g.err = fmt.Errorf("%s does not fit screenshot: %v", id, bounds)
		return false
	}
	if err := checkRenderedControlText(win.Contents, 1); err != nil {
		g.err = fmt.Errorf("%s: %w", id, err)
		return false
	}
	shot := helpScreen{ID: id, Title: title, Route: route, Width: bounds.Dx(), Height: bounds.Dy()}
	var walk func([]*eui.ItemData)
	walk = func(items []*eui.ItemData) {
		for _, it := range items {
			if it.Invisible {
				continue
			}
			if len(it.Tabs) > 0 {
				walk(it.Tabs[it.ActiveTab].Contents)
				continue
			}
			walk(it.Contents)
			kinds := map[eui.ItemTypeData]string{eui.ITEM_BUTTON: "Button", eui.ITEM_CHECKBOX: "Checkbox", eui.ITEM_RADIO: "Radio", eui.ITEM_INPUT: "Input", eui.ITEM_SLIDER: "Slider", eui.ITEM_DROPDOWN: "Choice"}
			kind, ok := kinds[it.ItemType]
			if !ok {
				continue
			}
			label := strings.TrimSpace(it.Label)
			if it == hotkeyNameInput {
				label = "Name"
			}
			for _, input := range hotkeyCmdInputs {
				if it == input {
					label = "Command"
				}
			}
			if it == shortcutShortInp {
				label = "Shortcut"
			}
			if it == shortcutFullInp {
				label = "Expansion"
			}
			if label == "" {
				label = strings.TrimSpace(it.Text)
			}
			if label == "" {
				label = it.Tooltip
			}
			if label == "" && id == "settings-display" && it.ItemType == eui.ITEM_SLIDER {
				label = "UI Scale"
			}
			if label == "" {
				continue
			}
			r := image.Rect(int(it.DrawRect.X0), int(it.DrawRect.Y0), int(it.DrawRect.X1), int(it.DrawRect.Y1))
			if r.Empty() || !r.In(bounds) {
				continue
			}
			c := helpControl{Label: label, Help: it.Tooltip, Kind: kind, Disabled: it.Disabled, Rect: [4]int{r.Min.X - bounds.Min.X, r.Min.Y - bounds.Min.Y, r.Dx(), r.Dy()}}
			switch it.ItemType {
			case eui.ITEM_CHECKBOX, eui.ITEM_RADIO:
				if it.Checked {
					c.Value = "On"
				} else {
					c.Value = "Off"
				}
			case eui.ITEM_DROPDOWN:
				c.Options = it.Options
				if it.Selected >= 0 && it.Selected < len(it.Options) {
					c.Value = it.Options[it.Selected]
				}
			case eui.ITEM_SLIDER:
				c.Value = fmt.Sprintf("%g (range %g–%g)", it.Value, it.MinValue, it.MaxValue)
			}
			shot.Controls = append(shot.Controls, c)
		}
	}
	walk(win.Contents)
	f, err := os.Create(filepath.Join(g.dir, id+".png"))
	if err != nil {
		g.err = err
		return false
	}
	g.err = png.Encode(f, screen.SubImage(bounds))
	if err := f.Close(); g.err == nil {
		g.err = err
	}
	g.screens = append(g.screens, shot)
	return g.err == nil
}
