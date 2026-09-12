package main

import (
	"os"
	"path/filepath"

	"gothoom/eui"

	"github.com/pkg/browser"
	open "github.com/skratchdot/open-golang/open"
)

const piperVoicesBrowseURL = "https://rhasspy.github.io/piper-samples/"

func ttsFilesMissing() bool {
	return status.NeedPiper || status.NeedPiperFem || status.NeedPiperMale
}

func openTTSDownloads() {
	makeDownloadsWindow(true)
	if downloadWin != nil {
		downloadWin.MarkOpen()
	}
}

func ttsVoicesDirPath() string {
	return filepath.Join(piperDirPath(), "voices")
}

func ensureTTSVoicesFolder() (string, error) {
	path := ttsVoicesDirPath()
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func openTTSVoicesFolder() error {
	path, err := ensureTTSVoicesFolder()
	if err != nil {
		return err
	}
	return open.Run(path)
}

// Share enablement between Settings, the Mixer, and the test-phrase button.
func setTTSEnabled(enabled bool) bool {
	if !enabled || isWASM {
		disableTTS()
		return false
	}
	if ttsFilesMissing() {
		disableTTS()
		openTTSDownloads()
		return false
	}
	gs.ChatTTS = true
	settingsDirty = true
	if ttsMixCB != nil {
		ttsMixCB.Checked = true
		ttsMixCB.Dirty = true
	}
	if ttsMixSlider != nil {
		ttsMixSlider.Disabled = false
		ttsMixSlider.Dirty = true
	}
	updateSoundVolume()
	return true
}

func addTTSEnablementControls(section *eui.ItemData, panelWidth float32) {
	enabled, events := eui.NewCheckbox()
	enabled.Text = "Enable Text to Speech"
	enabled.Size = eui.Point{X: 240, Y: 24}
	enabled.Checked = gs.ChatTTS
	enabled.Disabled = isWASM
	enabled.Action = func() {
		if enabled.Checked != gs.ChatTTS {
			enabled.Checked = gs.ChatTTS
			enabled.Dirty = true
		}
	}
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			setTTSEnabled(ev.Checked)
			enabled.Checked = gs.ChatTTS
		}
	}
	section.AddItem(enabled)

	actions := eui.NewRow()
	const actionColumns = 3
	actionWidth := (panelWidth - 8*(actionColumns-1)) / actionColumns

	download, downloadEvents := eui.NewButton()
	download.Size = eui.Point{X: actionWidth, Y: 24}
	setMaterialButtonIcon(download, "download")
	refresh := func() {
		label := "TTS files installed"
		if isWASM {
			label = "TTS unavailable in browser"
		} else if ttsFilesMissing() {
			label = "Download TTS files"
		}
		disabled := isWASM || !ttsFilesMissing()
		if download.Text != label || download.Disabled != disabled {
			download.Text = label
			download.Disabled = disabled
			download.Dirty = true
		}
	}
	refresh()
	download.Action = refresh
	downloadEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			openTTSDownloads()
		}
	}
	actions.AddItem(download)

	folder, folderEvents := eui.NewButton()
	folder.Text = "Open Voices Folder"
	folder.Size = eui.Point{X: actionWidth, Y: 24}
	folder.Disabled = isWASM
	setMaterialButtonIcon(folder, "folder_open")
	folder.SetTooltip("Open the piper/voices folder used for additional voice models.")
	folderEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if err := openTTSVoicesFolder(); err != nil {
			consoleMessage("open TTS voices folder: " + err.Error())
		}
	}
	actions.AddItem(folder)

	browse, browseEvents := eui.NewButton()
	browse.Text = "Browse More Voices"
	browse.Size = eui.Point{X: actionWidth, Y: 24}
	setMaterialButtonIcon(browse, "language")
	browse.SetTooltip("Preview and download Piper voices. Install both the .onnx model and its matching .onnx.json configuration file.")
	browseEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if err := browser.OpenURL(piperVoicesBrowseURL); err != nil {
			consoleMessage("open Piper voices page: " + err.Error())
		}
	}
	actions.AddItem(browse)

	section.AddItem(actions)
}
