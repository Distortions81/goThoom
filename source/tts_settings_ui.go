package main

import "gothoom/eui"

func ttsFilesMissing() bool {
	return status.NeedPiper || status.NeedPiperFem || status.NeedPiperMale
}

func openTTSDownloads() {
	makeDownloadsWindow(true)
	if downloadWin != nil {
		downloadWin.MarkOpen()
	}
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

func addTTSEnablementControls(section *eui.ItemData, width float32) {
	enabled, events := eui.NewCheckbox()
	enabled.Text = "Enable Text to Speech"
	enabled.Size = eui.Point{X: width, Y: 24}
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

	download, downloadEvents := eui.NewButton()
	download.Size = eui.Point{X: width, Y: 24}
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
	section.AddItem(download)
}
