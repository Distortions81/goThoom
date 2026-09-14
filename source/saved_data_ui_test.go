package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestSavedDataRowWrapsNameAndKeepsViewButtonInside(t *testing.T) {
	initFont()
	originalScale := eui.UIScale()
	t.Cleanup(func() { eui.SetUIScale(originalScale) })
	loadMaterialIcons()

	for _, scale := range []float32{1, 1.5, 2} {
		eui.SetUIScale(scale)
		const listWidth = float32(430)
		row := newSavedDataRow(strings.Repeat("Long saved-data script name ", 5), 18, 4096, listWidth, "test")
		if row.FlowType != eui.FLOW_OVERLAY || !row.Fixed || !row.ConstrainToSize {
			t.Fatalf("%gx row is not a constrained overlay", scale)
		}
		if len(row.Contents) != 3 {
			t.Fatalf("%gx row controls = %d, want name, details, and View", scale, len(row.Contents))
		}
		name, details, view := row.Contents[0], row.Contents[1], row.Contents[2]
		if view.Text != "View" || !view.ConstrainToSize {
			t.Fatalf("%gx View action is not explicitly sized", scale)
		}
		if right := view.Position.X + view.Size.X; right > row.Size.X {
			t.Fatalf("%gx View right edge %.1f exceeds row width %.1f", scale, right, row.Size.X)
		}
		nameBottom := name.Position.Y + name.GetSize().Y/scale
		if details.Position.Y < nameBottom || row.Size.Y < details.Position.Y+details.Size.Y {
			t.Fatalf("%gx text overlaps or is clipped: name bottom %.1f, details top %.1f, row bottom %.1f", scale, nameBottom, details.Position.Y, row.Size.Y)
		}
		view.ConstrainToSize = false
		if natural := view.GetSize().X / scale; natural > view.Size.X {
			t.Fatalf("%gx View caption and icon need %.1f logical pixels, have %.1f", scale, natural, view.Size.X)
		}
	}
}

func TestSavedDataEntriesAreWrappedAndSelectable(t *testing.T) {
	initFont()
	const owner = "saved-data-layout-test"
	originalWin, originalList := dataEntriesWin, dataEntriesList
	originalOwner, originalWrap := dataEntriesOwner, dataEntriesWrap
	scriptStoreMu.Lock()
	originalStores := scriptStores
	scriptStores = map[string]*scriptStore{
		owner: {data: map[string]any{
			"preferences": map[string]any{"player": "Mossy Boots", "message": strings.Repeat("wrapped value ", 20)},
		}},
	}
	scriptStoreMu.Unlock()

	dataEntriesWin, dataEntriesList, _ = eui.NewTextWindow("Saved Data", eui.HZoneCenter, eui.VZoneMiddleTop, false)
	dataEntriesWin.Size = eui.Point{X: 340, Y: 240}
	dataEntriesWrap = eui.TextWindowWrapCache{}
	t.Cleanup(func() {
		dataEntriesWin.RemoveWindow()
		dataEntriesWin, dataEntriesList = originalWin, originalList
		dataEntriesOwner, dataEntriesWrap = originalOwner, originalWrap
		scriptStoreMu.Lock()
		scriptStores = originalStores
		scriptStoreMu.Unlock()
	})

	refreshSavedDataEntries(owner)
	if len(dataEntriesList.Contents) != 2 {
		t.Fatalf("detail rows = %d, want script heading and one value", len(dataEntriesList.Contents))
	}
	entry := dataEntriesList.Contents[1]
	if !entry.SelectableText || !strings.Contains(entry.Text, "preferences\n{") || strings.Count(entry.Text, "\n") < 3 {
		t.Fatalf("saved value is not readable wrapped selectable text: selectable=%v text=%q", entry.SelectableText, entry.Text)
	}
}

// Run alone because it starts Ebitengine's game loop and exports a visual QA
// image of both the script list and the View window.
func TestRenderSavedDataWindows(t *testing.T) {
	dir := os.Getenv("GOTHOOM_SAVED_DATA_RENDER_DIR")
	if dir == "" {
		t.Skip("set GOTHOOM_SAVED_DATA_RENDER_DIR to export the Saved Data review image")
	}
	initFont()
	eui.SetScreenSize(1280, 720)
	eui.SetUIScale(1)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	if err := eui.LoadStyle("Default"); err != nil {
		t.Fatal(err)
	}
	loadMaterialIcons()

	originalDataDir := dataDirPath
	originalNames := scriptDisplayNames
	scriptStoreMu.Lock()
	originalStores := scriptStores
	scriptStoreMu.Unlock()
	originalSavedWin, originalSavedRoot, originalSavedList := savedDataWin, savedDataRoot, savedDataList
	originalEntriesWin, originalEntriesList := dataEntriesWin, dataEntriesList
	originalEntriesOwner, originalEntriesWrap := dataEntriesOwner, dataEntriesWrap
	dataDirPath = t.TempDir()
	scriptDisplayNames = map[string]string{
		"follow":    "Follow Player and Keep the Party Together Across Every Hunting Ground",
		"reminder":  "Daily Reminder",
		"inventory": "Inventory Helper",
		"notes":     "Hunting Notes for Mossy Boots",
		"alerts":    "Rare Item Alerts",
		"combat":    "Combat Summary",
		"healing":   "Healer Targets",
		"routes":    "Favorite Routes",
	}
	scriptStoreMu.Lock()
	scriptStores = map[string]*scriptStore{}
	scriptStoreMu.Unlock()
	savedDataWin, savedDataRoot, savedDataList = nil, nil, nil
	dataEntriesWin, dataEntriesList = nil, nil
	dataEntriesOwner, dataEntriesWrap = "", eui.TextWindowWrapCache{}
	t.Cleanup(func() {
		if savedDataWin != nil {
			savedDataWin.RemoveWindow()
		}
		if dataEntriesWin != nil {
			dataEntriesWin.RemoveWindow()
		}
		dataDirPath = originalDataDir
		scriptDisplayNames = originalNames
		scriptStoreMu.Lock()
		scriptStores = originalStores
		scriptStoreMu.Unlock()
		savedDataWin, savedDataRoot, savedDataList = originalSavedWin, originalSavedRoot, originalSavedList
		dataEntriesWin, dataEntriesList = originalEntriesWin, originalEntriesList
		dataEntriesOwner, dataEntriesWrap = originalEntriesOwner, originalEntriesWrap
	})

	for owner, name := range scriptDisplayNames {
		values := map[string]any{
			"character":   "Mossy Boots",
			"preferences": map[string]any{"enabled": true, "message": "Keep this long value readable when the detail window is resized instead of cutting it off."},
			"visits":      []any{"Town", "Forest", "Mountain Pass"},
		}
		path := scriptStoragePath(owner)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		encoded := []byte(fmt.Sprintf("{\n  \"script\": %q,\n  \"enabled\": true\n}\n", name))
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		scriptStoreMu.Lock()
		scriptStores[owner] = &scriptStore{path: path, data: values}
		scriptStoreMu.Unlock()
	}

	makeSavedDataWindow()
	savedDataWin.ClearZone()
	savedDataWin.SetPos(eui.Point{X: 30, Y: 40})
	savedDataWin.MarkOpen()
	showSavedDataEntries("follow")
	dataEntriesWin.ClearZone()
	dataEntriesWin.SetPos(eui.Point{X: 660, Y: 40})

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	game := &savedDataRenderGame{path: filepath.Join(dir, "saved-data.png")}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type savedDataRenderGame struct {
	done bool
	err  error
	path string
}

func (g *savedDataRenderGame) Layout(_, _ int) (int, int) { return 1280, 720 }

func (g *savedDataRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *savedDataRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	screen.Fill(eui.NewColor(28, 28, 28, 255).ToRGBA())
	eui.Draw(screen)
	for index, row := range savedDataList.Contents {
		if len(row.Contents) != 3 {
			continue
		}
		view := row.Contents[2]
		// Rows below the viewport are intentionally reduced to an empty clipped
		// rectangle; only validate actions in rows that are actually visible.
		if row.DrawRect.Y1-row.DrawRect.Y0 < view.Size.Y {
			continue
		}
		if view.DrawRect.X0 < row.DrawRect.X0 || view.DrawRect.X1 > row.DrawRect.X1 ||
			view.DrawRect.Y0 < row.DrawRect.Y0 || view.DrawRect.Y1 > row.DrawRect.Y1 {
			g.err = fmt.Errorf("row %d View action escaped row: view=%+v row=%+v", index, view.DrawRect, row.DrawRect)
			return
		}
	}
	file, err := os.Create(g.path)
	if err != nil {
		g.err = err
		return
	}
	g.err = png.Encode(file, screen)
	_ = file.Close()
}
