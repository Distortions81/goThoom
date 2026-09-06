package main

import (
	"errors"
	"fmt"

	"gothoom/eui"
)

var (
	filePathsWin      *eui.WindowData
	filePathsCopyCB   *eui.ItemData
	filePathsStatus   *eui.ItemData
	filePathsDisplays map[storagePathKind]*eui.ItemData
)

func makeFilePathsWindow() {
	if filePathsWin != nil {
		return
	}
	filePathsWin = eui.NewWindow()
	filePathsWin.ShowTooltipIndicators = true
	filePathsWin.Title = "File Paths"
	filePathsWin.Closable = true
	filePathsWin.Resizable = false
	filePathsWin.AutoSize = true
	filePathsWin.Movable = true
	filePathsWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	flow := eui.NewColumn()
	info, _ := eui.NewText()
	info.Text = "Choose alternate folders for assets and audio, logs, legacy macros, and Go scripts.\nSaved changes take effect after restarting goThoom."
	info.Size = eui.Point{X: 720, Y: 36}
	info.FontSize = 11
	flow.AddItem(info)

	filePathsCopyCB, _ = eui.NewCheckbox()
	filePathsCopyCB.Text = "Copy existing files to the selected folder"
	filePathsCopyCB.Checked = true
	filePathsCopyCB.Size = eui.Point{X: 720, Y: 24}
	filePathsCopyCB.SetTooltip("Copy files used by this category. Existing files with different contents cause the change to fail and are never overwritten.")
	flow.AddItem(filePathsCopyCB)

	filePathsDisplays = make(map[storagePathKind]*eui.ItemData)
	for _, kind := range storagePathKinds {
		flow.AddItem(makeFilePathRow(kind))
	}

	note, _ := eui.NewText()
	note.Text = "Every destination is created if needed, then tested by writing, reading, and removing\na temporary file before the setting is saved."
	note.Size = eui.Point{X: 720, Y: 36}
	note.FontSize = 10
	flow.AddItem(note)

	filePathsStatus, _ = eui.NewText()
	filePathsStatus.Size = eui.Point{X: 720, Y: 30}
	filePathsStatus.FontSize = 11
	flow.AddItem(filePathsStatus)

	filePathsWin.AddItem(flow)
	filePathsWin.AddWindow(false)
}

func makeFilePathRow(kind storagePathKind) *eui.ItemData {
	row := eui.NewRow()
	row.Size = eui.Point{X: 720, Y: 28}

	label, _ := eui.NewText()
	label.Text = storagePathName(kind)
	label.Size = eui.Point{X: 115, Y: 24}
	label.FontSize = 12
	eui.ApplyBoldFace(label)
	row.AddItem(label)

	display, _ := eui.NewText()
	display.Text = configuredStoragePath(gs, kind)
	display.Size = eui.Point{X: 395, Y: 24}
	display.FontSize = 10
	display.SelectableText = true
	display.SetTooltip(display.Text)
	filePathsDisplays[kind] = display
	row.AddItem(display)

	choose, chooseEvents := eui.NewButton()
	choose.Text = "Choose..."
	choose.Size = eui.Point{X: 100, Y: 24}
	setMaterialButtonIcon(choose, "folder_open")
	chooseEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		selected, err := pickStorageDirectory("Choose "+storagePathName(kind)+" Folder", configuredStoragePath(gs, kind))
		if errors.Is(err, errStorageDirectoryDialogCancelled) {
			return
		}
		if err != nil {
			makeErrorWindow(fmt.Sprintf("Choose %s folder: %v", storagePathName(kind), err))
			return
		}
		applyStoragePathSelection(kind, selected)
	}
	row.AddItem(choose)

	useDefault, defaultEvents := eui.NewButton()
	useDefault.Text = "Default"
	useDefault.Size = eui.Point{X: 90, Y: 24}
	useDefault.SetTooltip("Restore the standard location inside the goThoom user data folder.")
	defaultEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			applyStoragePathSelection(kind, "")
		}
	}
	row.AddItem(useDefault)
	return row
}

func applyStoragePathSelection(kind storagePathKind, configuredPath string) {
	copyFiles := filePathsCopyCB != nil && filePathsCopyCB.Checked
	SettingsLock.Lock()
	destination, err := changeStoragePath(kind, configuredPath, copyFiles)
	SettingsLock.Unlock()
	if err != nil {
		makeErrorWindow(fmt.Sprintf("File path change failed: %v", err))
		return
	}
	if display := filePathsDisplays[kind]; display != nil {
		display.Text = destination
		display.SetTooltip(destination)
		display.Dirty = true
	}
	if filePathsStatus != nil {
		filePathsStatus.Text = fmt.Sprintf("%s path saved. Restart goThoom to use %s.", storagePathName(kind), destination)
		filePathsStatus.Dirty = true
	}
	filePathsWin.Refresh()
}
