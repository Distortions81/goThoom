package main

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/climg"
	"gothoom/eui"
	scriptapi "gt2"
)

// Run alone: this starts Ebitengine's game loop and exports visual QA images.
func TestRenderScriptWindow(t *testing.T) {
	beastSettings := os.Getenv("GOTHOOM_RENDER_BEAST_SETTINGS") != ""
	lasties := os.Getenv("GOTHOOM_RENDER_LASTIES") != "" || beastSettings
	controls := os.Getenv("GOTHOOM_RENDER_SCRIPT_CONTROLS") != ""
	settings := os.Getenv("GOTHOOM_RENDER_SCRIPT_SETTINGS")
	manager := os.Getenv("GOTHOOM_RENDER_SCRIPTS_LIST") != ""
	permissions := os.Getenv("GOTHOOM_RENDER_SCRIPT_PERMISSIONS") != ""
	info := os.Getenv("GOTHOOM_RENDER_SCRIPT_INFO") != ""
	if os.Getenv("GOTHOOM_RENDER_SCRIPT_WINDOW") == "" && !permissions && !info && !manager && settings == "" && !controls && !lasties {
		t.Skip("set GOTHOOM_RENDER_SCRIPT_WINDOW=1")
	}
	initFont()
	eui.SetScreenSize(1400, 1000)
	eui.SetUIScale(1)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	isolateScriptWorld(t)
	const owner = "window_render"
	var panel Window
	var native *eui.WindowData
	prefix := "follow"
	if lasties {
		prefix = "lasties"
		original := clImages
		if path := os.Getenv("GOTHOOM_TEST_CL_IMAGES"); path != "" {
			var err error
			clImages, err = climg.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { clearCaches(); clImages = original })
		}
		sim := activateBundledProofScript(t, owner, "mark_beasts.go")
		for _, id := range []uint16{22, 71, 82} {
			click := makeScriptInputEvent("Alt-LeftClick")
			click.OnMobile, click.Mobile.PictID = true, id
			sim.input(t, click)
		}
		sim.barrier(t)
		value, err := currentScriptEventQueue(owner).interpreter.Eval("lastiesWindow")
		if err != nil {
			t.Fatal(err)
		}
		panel = value.Interface().(Window)
		panel.state.controls["entry-2-name"].item.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: "Sam"})
		for index, note := range []string{"Leave last hit for Sam", "Watch for a group nearby", "Check this sprite on the next hunt"} {
			panel.state.controls[fmt.Sprintf("entry-%d-note", index+1)].item.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: note})
		}
		sim.barrier(t)
		panel.state.buttons["add"].Handler.Emit(eui.UIEvent{Type: eui.EventClick})
		sim.barrier(t)
		if beastSettings {
			panel.state.buttons["settings"].Handler.Emit(eui.UIEvent{Type: eui.EventClick})
			sim.barrier(t)
			if scriptConfigWin == nil {
				t.Fatal("Mark Beasts settings gear did not open native settings")
			}
			native = scriptConfigWin
			prefix = "beast-settings"
		}
		if native == nil {
			native = panel.state.ui
		}
	} else if controls {
		prefix = "controls"
		resetScriptCallbackTestState(t, owner)
		create := exportsForscript(owner)["gt2/gt2"]["CreateWindow"].Interface().(func(scriptapi.WindowOptions) Window)
		panel = create(scriptapi.WindowOptions{Title: "Delayed Command", Width: 340, Text: "Choose a command and delay, then start.",
			Controls: []scriptapi.WindowControl{
				{ID: "command", Label: "Command", Kind: scriptapi.ControlText, Text: "/pose sit"},
				{ID: "notify", Label: "Notify when sent", Kind: scriptapi.ControlCheckbox, Checked: true},
				{ID: "delay", Label: "Delay", Kind: scriptapi.ControlDropdown, Options: []string{"1 second", "3 seconds", "10 seconds"}},
				{ID: "recent", Label: "Recent commands", Kind: scriptapi.ControlList, Options: []string{"/pose sit", "/pose kneel", "/pose akimbo"}},
			}, Buttons: []scriptapi.WindowButton{{ID: "start", Label: "Start", OnClick: func() {}}, {ID: "cancel", Label: "Cancel", Disabled: true, OnClick: func() {}}},
		})
		drainScriptDispatcher()
		native = panel.state.ui
		t.Cleanup(func() { disablescript(owner, "test cleanup"); drainScriptDispatcher() })
	} else if settings != "" {
		prefix = "settings-" + settings
		activateBundledProofScript(t, owner, "follow_player.go")
		scriptDisplayNames[owner] = "Follow Player"
		scriptAddHotkeyFn(owner, "Ctrl-F3", func(InputEvent) {})
		openscriptConfigWindow(owner)
		native = scriptConfigWin
		for index, tab := range native.Contents[0].Tabs {
			if tab.Name == settings {
				native.Contents[0].ActiveTab = index
			}
		}
		t.Cleanup(func() { native.Close() })
	} else if manager {
		prefix = "list"
		isolateScriptScopeSelection(t)
		name = "Alpha"
		scriptDisplayNames = map[string]string{"follow": "Follow Player", "reminder": "Daily Reminder", "chat": "Chat Tools", "inventory": "Inventory Helper", "broken": "Example With An Error"}
		scriptCategories = map[string]string{"follow": "Automation", "reminder": "Automation", "chat": "Communication", "inventory": "Inventory", "broken": "Inventory"}
		scriptDisabled = map[string]bool{"follow": true, "reminder": true, "chat": true, "inventory": true, "broken": true}
		scriptEnabledFor = map[string]scriptScope{"follow": {All: true}, "reminder": {Chars: map[string]bool{"Alpha": true}}}
		scriptErrors = map[string]string{"broken": "Example error"}
		makescriptsWindow()
		native = scriptsWin
		native.MarkOpen()
		t.Cleanup(func() { native.RemoveWindow() })
	} else if info {
		prefix = "info"
		scriptDisplayNames[owner] = "Follow Player"
		scriptAuthors[owner] = "Example Scripts"
		scriptPaths[owner] = "/Scripts/follow_player.go"
		scriptDescriptions[owner] = "Follow a visible player with clearance, local routing and wiggle recovery."
		scriptAPIVersions[owner] = 2
		shortcutMaps[owner] = map[string]string{}
		for index := 0; index < 35; index++ {
			shortcutMaps[owner][fmt.Sprintf("example-%02d", index)] = "Example action"
		}
		selectscript(owner)
		native = scriptInfoWin
		t.Cleanup(func() { native.Close(); delete(shortcutMaps, owner) })
	} else if permissions {
		prefix = "permissions"
		source, err := scriptScripts.ReadFile(bundledScriptDir + "/follow_player.go")
		if err != nil {
			t.Fatal(err)
		}
		scriptPackages[owner] = scriptInfo{id: owner, name: "Follow Player", src: source}
		scriptDisplayNames[owner] = "Follow Player"
		grantScriptPermissionsForTest(t, owner)
		openScriptPermissionsWindow(owner)
		native = scriptPermissionsWin
		t.Cleanup(func() { native.Close() })
	} else {
		activateBundledProofScript(t, owner, "follow_player.go")
		value, err := currentScriptEventQueue(owner).interpreter.Eval("followWindow")
		if err != nil {
			t.Fatal(err)
		}
		panel = value.Interface().(Window)
		native = panel.state.ui
	}
	dir := os.Getenv("GOTHOOM_SCRIPT_WINDOW_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	game := &scriptWindowRenderGame{panel: panel, window: native, prefix: prefix, dir: dir}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type scriptWindowRenderGame struct {
	panel  Window
	window *eui.WindowData
	prefix string
	dir    string
	done   bool
	err    error
}

func (g *scriptWindowRenderGame) Layout(_, _ int) (int, int) { return 1400, 1000 }
func (g *scriptWindowRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *scriptWindowRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	loadMaterialIcons()
	for index, scale := range []float32{1, 1.5, 2} {
		eui.SetUIScale(scale)
		if index > 0 && g.panel.state != nil && g.prefix == "follow" {
			g.panel.SetText("Following: A Player With A Long Character Name\nStatus: Routing")
			g.panel.SetButtonEnabled("stop", true)
		}
		if g.prefix == "chat" {
			updateChatWindow()
		}
		if g.prefix == "list" {
			refreshscriptsWindow()
		}
		if g.prefix == "info" && index > 0 {
			scriptDetails.Scroll.Y = 300
		}
		drainScriptDispatcher()
		g.window.Refresh()
		screen.Fill(color.RGBA{R: 24, G: 28, B: 34, A: 255})
		eui.Draw(screen)
		path := filepath.Join(g.dir, fmt.Sprintf("%s-%.1fx.png", g.prefix, scale))
		file, err := os.Create(path)
		if err != nil {
			g.err = err
			return
		}
		g.err = png.Encode(file, screen)
		_ = file.Close()
		if g.err != nil {
			return
		}
		if g.err = checkRenderedControlText(g.window.Contents, scale); g.err != nil {
			return
		}
		if horizontal, vertical := g.window.RequiresScroll(); horizontal || vertical && g.prefix != "controls" {
			g.err = fmt.Errorf("window needs scrollbars at %.1fx", scale)
			return
		}

	}
}
