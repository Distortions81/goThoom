package main

import (
	"os"

	"gothoom/eui"

	open "github.com/skratchdot/open-golang/open"
)

var snapshotWin *eui.WindowData
var snapshotName, snapshotStatus *eui.ItemData

func showSnapshotWindow() {
	if isWASM {
		consoleMessage("Snapshots are available in the desktop client.")
		return
	}
	if snapshotWin != nil && snapshotWin.Open {
		snapshotWin.BringForward()
		return
	}
	if snapshotWin == nil {
		makeSnapshotWindow()
	}
	snapshotName.Text = defaultSnapshotName()
	snapshotStatus.Text = ""
	snapshotWin.DefaultButton.Disabled = false
	snapshotWin.MarkOpen()
	snapshotWin.Refresh()
}

func makeSnapshotWindow() {
	win := eui.NewWindow()
	snapshotWin = win
	win.Title = "Snapshot"
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.Resizable = false
	win.Padding = 12
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 400, Y: 1}}
	name, nameEvents := eui.NewInput()
	snapshotName = name
	name.Label = "Filename"
	name.Size = eui.Point{X: 400, Y: 28}
	root.AddItem(name)
	row := eui.NewRow()
	area, _ := eui.NewDropdown()
	area.Label = "Capture"
	area.Options = []string{"Game view only", "Entire client window"}
	area.Size = eui.Point{X: 260, Y: 28}
	format, _ := eui.NewDropdown()
	format.Label = "Format"
	format.Options = []string{"PNG", "JPEG"}
	format.Size = eui.Point{X: 128, Y: 28}
	format.Position.X = 8
	row.AddItem(area)
	row.AddItem(format)
	root.AddItem(row)
	names, _ := eui.NewCheckbox()
	names.Text = "Include name tags"
	names.Checked = true
	names.Size = eui.Point{X: 400, Y: 28}
	names.SetTooltip("Turn off to hide overhead name tags for this snapshot.")
	root.AddItem(names)
	hint, _ := eui.NewText()
	hint.Text = "Saved in Screenshots. Duplicate names get a number."
	hint.FontSize = 11
	hint.Size = eui.Point{X: 400, Y: 24}
	root.AddItem(hint)
	status, _ := eui.NewText()
	snapshotStatus = status
	status.FontSize = 11
	status.Size = eui.Point{X: 400, Y: 30}
	root.AddItem(status)
	footer := eui.NewRow()
	folder, folderEvents := eui.NewButton()
	folder.Text = "Open Folder"
	folder.Size = eui.Point{X: 128, Y: 32}
	folderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		dir := snapshotDirectory()
		err := os.MkdirAll(dir, 0o755)
		if err == nil {
			err = open.Run(dir)
		}
		if err != nil {
			status.Text = "Could not open the Screenshots folder."
			logError("snapshot folder: %v", err)
			win.Refresh()
		}
	}
	cancel, cancelEvents := eui.NewButton()
	cancel.Text = "Cancel"
	cancel.Size = eui.Point{X: 112, Y: 32}
	cancelEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			win.Close()
		}
	}
	capture, captureEvents := eui.NewButton()
	capture.Text = "Capture"
	capture.Size = eui.Point{X: 144, Y: 32}
	nameEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventInputChanged {
			_, err := snapshotFilename(ev.Text, format.Selected == 1)
			capture.Disabled = err != nil
			status.Text = ""
			if err != nil {
				status.Text = err.Error()
			}
			win.Refresh()
		}
	}
	captureEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick || pendingSnapshot != nil {
			return
		}
		if _, err := snapshotFilename(name.Text, format.Selected == 1); err != nil {
			status.Text = err.Error()
			win.Refresh()
			return
		}
		pendingSnapshot = &snapshotRequest{name: name.Text, fullWindow: area.Selected == 1, hideNameTags: !names.Checked, jpeg: format.Selected == 1, done: func(err error) {
			if err != nil {
				status.Text = "Could not save snapshot; see Console for details."
				win.MarkOpen()
				win.Refresh()
			}
		}}
		win.Close()
	}
	footer.AddItem(folder)
	footer.AddItem(cancel)
	footer.AddItem(capture)
	root.AddItem(footer)
	win.AddItem(root)
	win.DefaultButton = capture
	win.AddWindow(false)
	_ = win.SetPos(eui.Point{X: 80, Y: 80})
}
