package main

import (
	"fmt"
	"sort"

	"gothoom/eui"
)

var (
	hdPicturePreviewWin      *eui.WindowData
	hdPicturePreviewPicker   *eui.ItemData
	hdPicturePreviewOriginal *eui.ItemData
	hdPicturePreviewNew      *eui.ItemData
	hdPicturePreviewStatus   *eui.ItemData
	hdPicturePreviewIDs      []uint16
	hdPicturePreviewID       uint16
)

func makeHDPicturePreviewWindow() {
	if hdPicturePreviewWin != nil {
		return
	}
	hdPicturePreviewWin = eui.NewWindow()
	hdPicturePreviewWin.Title = "HD Sprite Preview"
	hdPicturePreviewWin.Closable = true
	hdPicturePreviewWin.Resizable = false
	hdPicturePreviewWin.AutoSize = true
	hdPicturePreviewWin.Movable = true
	hdPicturePreviewWin.SetZone(eui.HZoneRight, eui.VZoneTop)

	flow := eui.NewColumn()
	var pickerEvents *eui.EventHandler
	hdPicturePreviewPicker, pickerEvents = eui.NewDropdown()
	hdPicturePreviewPicker.Label = "Picture"
	hdPicturePreviewPicker.Size = eui.Point{X: 280, Y: settingsControlHeight}
	pickerEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(hdPicturePreviewIDs) {
			hdPicturePreviewID = hdPicturePreviewIDs[ev.Index]
			updateHDPicturePreviewImages()
		}
	}
	flow.AddItem(hdPicturePreviewPicker)

	images := eui.NewRow()
	original := eui.NewColumn()
	originalLabel, _ := eui.NewText()
	originalLabel.Text = "Original"
	originalLabel.Size = eui.Point{X: 140, Y: 22}
	original.AddItem(originalLabel)
	hdPicturePreviewOriginal = eui.NewImageReferenceItem(140, 140)
	original.AddItem(hdPicturePreviewOriginal)
	images.AddItem(original)

	replacement := eui.NewColumn()
	replacementLabel, _ := eui.NewText()
	replacementLabel.Text = "HD replacement"
	replacementLabel.Size = eui.Point{X: 140, Y: 22}
	replacement.AddItem(replacementLabel)
	hdPicturePreviewNew = eui.NewImageReferenceItem(140, 140)
	replacement.AddItem(hdPicturePreviewNew)
	images.AddItem(replacement)
	flow.AddItem(images)

	hdPicturePreviewStatus, _ = eui.NewText()
	hdPicturePreviewStatus.Size = eui.Point{X: 280, Y: 32}
	flow.AddItem(hdPicturePreviewStatus)

	reloadButton, reloadEvents := eui.NewButton()
	reloadButton.Text = "Reload HD Sprites"
	setMaterialButtonIcon(reloadButton, "restart_alt")
	reloadButton.Size = eui.Point{X: 280, Y: settingsControlHeight}
	reloadButton.SetTooltip("Read PNGs and ZIPs in the game or user-data hdimg folder again.")
	reloadEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			reloadHDPictureDebug()
		}
	}
	flow.AddItem(reloadButton)

	hdPicturePreviewWin.AddItem(flow)
	hdPicturePreviewWin.AddWindow(false)
}

func openHDPicturePreview() {
	makeHDPicturePreviewWindow()
	refreshHDPicturePreview()
	hdPicturePreviewWin.MarkOpen()
}

func reloadHDPictureDebug() {
	// EUI must not retain a texture that the reload is about to deallocate.
	if hdPicturePreviewOriginal != nil {
		hdPicturePreviewOriginal.Image = nil
		hdPicturePreviewOriginal.Dirty = true
		hdPicturePreviewNew.Image = nil
		hdPicturePreviewNew.Dirty = true
	}
	location, count := reloadHDPictures()
	updateToolbarHands()
	refreshHDPicturePreview()
	consoleMessage(fmt.Sprintf("Reloaded %d HD sprites from %s.", count, location))
}

func refreshHDPicturePreview() {
	if hdPicturePreviewWin == nil {
		return
	}
	imageCacheLifecycleMu.RLock()
	ids := make([]uint16, 0, len(hdPictureSources))
	for id := range hdPictureSources {
		ids = append(ids, id)
	}
	imageCacheLifecycleMu.RUnlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	hdPicturePreviewIDs = ids
	hdPicturePreviewPicker.Options = hdPicturePreviewPicker.Options[:0]
	hdPicturePreviewPicker.Selected = 0
	for index, id := range ids {
		hdPicturePreviewPicker.Options = append(hdPicturePreviewPicker.Options, fmt.Sprintf("%d", id))
		if id == hdPicturePreviewID {
			hdPicturePreviewPicker.Selected = index
		}
	}
	if len(ids) == 0 {
		hdPicturePreviewPicker.Options = []string{"No HD sprites"}
		hdPicturePreviewID = 0
	} else {
		hdPicturePreviewID = ids[hdPicturePreviewPicker.Selected]
	}
	hdPicturePreviewPicker.Dirty = true
	updateHDPicturePreviewImages()
}

func updateHDPicturePreviewImages() {
	if hdPicturePreviewOriginal == nil {
		return
	}
	hdPicturePreviewOriginal.Image = nil
	hdPicturePreviewNew.Image = nil
	if hdPicturePreviewID != 0 {
		hdPicturePreviewOriginal.Image = loadImageFrameOriginal(hdPicturePreviewID, 0)
		hdPicturePreviewNew.Image = loadHDPicture(hdPicturePreviewID)
	}
	hdPicturePreviewOriginal.Dirty = true
	hdPicturePreviewNew.Dirty = true
	switch {
	case len(hdPicturePreviewIDs) == 0:
		hdPicturePreviewStatus.Text = "No sprite pack files found in game or user-data hdimg."
	case !gs.UseSpritePackFiles:
		hdPicturePreviewStatus.Text = "Enable Use sprite pack files to preview replacements."
	case clImages == nil:
		hdPicturePreviewStatus.Text = "Original appears after CL_Images loads."
	case hdPicturePreviewNew.Image == nil:
		hdPicturePreviewStatus.Text = "HD image unavailable or not single-frame."
	default:
		hdPicturePreviewStatus.Text = fmt.Sprintf("Picture %d: original and HD at equal UI size.", hdPicturePreviewID)
	}
	hdPicturePreviewStatus.Dirty = true
}
