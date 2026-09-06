package main

import (
	"testing"

	"gothoom/eui"
)

func TestTTSSettingsReflectEnablementAndInstalledFiles(t *testing.T) {
	initFont()
	originalSettings, originalStatus := gs, status
	t.Cleanup(func() { gs, status = originalSettings, originalStatus })
	gs.ChatTTS = true
	status = dataFilesStatus{NeedPiperFem: true}
	section := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}
	addTTSEnablementControls(section, settingsPanelWidth)
	enabled, download := section.Contents[0], section.Contents[1]
	if !enabled.Checked || download.Disabled || download.Text != "Download TTS files" {
		t.Fatal("TTS controls did not reflect saved enablement and missing voice files")
	}
	// Changes made by the mixer and a completed download must be visible
	// without rebuilding Settings.
	gs.ChatTTS = false
	status = dataFilesStatus{}
	enabled.Action()
	download.Action()
	if enabled.Checked || !download.Disabled || download.Text != "TTS files installed" {
		t.Fatal("TTS controls did not refresh after disablement and installation")
	}
}
