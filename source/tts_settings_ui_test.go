package main

import (
	"os"
	"path/filepath"
	"testing"

	"gothoom/eui"
)

func TestTTSSettingsReflectEnablementAndInstalledFiles(t *testing.T) {
	initFont()
	originalSettings, originalStatus := gs, status
	t.Cleanup(func() { gs, status = originalSettings, originalStatus })
	gs.ChatTTS = true
	status = dataFilesStatus{NeedPiperFem: true}
	section := eui.NewColumn()
	addTTSEnablementControls(section, settingsPanelWidth)
	enabled := section.Contents[0]
	actions := section.Contents[1]
	if len(actions.Contents) != 3 {
		t.Fatalf("TTS setup actions = %d, want download, folder, and browse buttons", len(actions.Contents))
	}
	download, folder, browse := actions.Contents[0], actions.Contents[1], actions.Contents[2]
	if !enabled.Checked || download.Disabled || download.Text != "Download TTS files" {
		t.Fatal("TTS controls did not reflect saved enablement and missing voice files")
	}
	if folder.Text != "Open Voices Folder" || browse.Text != "Browse More Voices" {
		t.Fatalf("TTS setup actions = %q and %q", folder.Text, browse.Text)
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

func TestTTSVoicesDirectoryUsesActiveAssetsPath(t *testing.T) {
	originalSettings := gs
	originalStorageActive := storagePathsActivated
	t.Cleanup(func() {
		gs = originalSettings
		storagePathsActivated = originalStorageActive
	})

	assets := t.TempDir()
	gs.AssetsPath = assets
	storagePathsActivated = false
	want := filepath.Join(assets, "piper", "voices")
	if got := ttsVoicesDirPath(); got != want {
		t.Fatalf("TTS voices directory = %q, want %q", got, want)
	}
	created, err := ensureTTSVoicesFolder()
	if err != nil {
		t.Fatalf("create TTS voices directory: %v", err)
	}
	if created != want {
		t.Fatalf("created TTS voices directory = %q, want %q", created, want)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("TTS voices directory was not created: %v", err)
	}
}

func TestTTSMessageOptionsUpdateSettings(t *testing.T) {
	initFont()
	originalSettings, originalStatus := gs, status
	originalDirty := settingsDirty
	t.Cleanup(func() {
		gs, status = originalSettings, originalStatus
		settingsDirty = originalDirty
	})
	gs = gsdef
	status = dataFilesStatus{}
	settingsDirty = false
	mainSection := eui.NewColumn()
	messagesSection := eui.NewColumn()
	testSection := eui.NewColumn()
	addTTSSettings(mainSection, messagesSection, testSection)

	items := make(map[string]*eui.ItemData)
	for _, row := range messagesSection.Contents {
		for _, item := range row.Contents {
			items[item.Text] = item
		}
	}
	if len(items) != 9 {
		t.Fatalf("TTS message options = %d, want 9", len(items))
	}
	if len(messagesSection.Contents) != 3 {
		t.Fatalf("TTS message option rows = %d, want 3 so test controls remain visible", len(messagesSection.Contents))
	}
	speech, own, notifications := items["Speech"], items["Your messages"], items["Notifications"]
	if speech == nil || own == nil || notifications == nil {
		t.Fatal("TTS message options are missing Speech, Your messages, or Notifications")
	}
	speech.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: false, Item: speech})
	own.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true, Item: own})
	notifications.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true, Item: notifications})
	if gs.ChatTTSSay || !gs.ChatTTSSelf || !gs.ChatTTSNotifications || !settingsDirty {
		t.Fatalf("TTS option changes not saved: say=%t self=%t notifications=%t dirty=%t", gs.ChatTTSSay, gs.ChatTTSSelf, gs.ChatTTSNotifications, settingsDirty)
	}
}
