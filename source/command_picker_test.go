package main

import (
	"reflect"
	"strings"
	"testing"

	"gothoom/eui"
)

func TestAvailableTextCommandGroupsSortByTypeAndSource(t *testing.T) {
	session, err := newSession(7)
	if err != nil {
		t.Fatal(err)
	}
	session.automation.scriptMu.Lock()
	session.automation.localCommands["wave"] = sessionScriptCommand{owner: "alpha"}
	session.automation.localCommands["block"] = sessionScriptCommand{owner: "client"}
	session.automation.scriptMu.Unlock()
	session.automation.legacyMu.Lock()
	session.automation.legacyProgram.Macros = []legacyMacroDeclaration{
		{Kind: legacyMacroExpression, Trigger: "zebra", Header: legacyMacroLocation{Path: "/Macros/z.mac"}},
		{Kind: legacyMacroExpression, Trigger: "apple", Header: legacyMacroLocation{Path: "/Macros/a.mac"}},
		{Kind: legacyMacroExpression, Trigger: "again", Header: legacyMacroLocation{Path: "/Macros/a.mac"}},
		{Kind: legacyMacroFunction, Trigger: "not-a-text-command", Header: legacyMacroLocation{Path: "/Macros/a.mac"}},
	}
	session.automation.legacyMu.Unlock()

	groups := availableTextCommandGroups(session)
	want := []textCommandGroup{
		{typeName: "Client", source: "goThoom", commands: []string{"/block", "/palette", "/play", "/setting", "/tab"}},
		{typeName: "Script", source: "alpha", commands: []string{"/wave"}},
		{typeName: "Macro", source: "a.mac", commands: []string{"again", "apple"}},
		{typeName: "Macro", source: "z.mac", commands: []string{"zebra"}},
		{typeName: "Server", source: "Clan Lord", commands: slashCommands(serverCommandNames)},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %#v, want %#v", groups, want)
	}
}

func TestStartMessageCommandPrefillsDraftWithoutSending(t *testing.T) {
	oldText, oldPos, oldActive, oldSelected := inputText, inputPos, inputActive, selectedMessageInput
	oldConsole, oldChat := consoleWin, chatWin
	t.Cleanup(func() {
		inputText, inputPos, inputActive, selectedMessageInput = oldText, oldPos, oldActive, oldSelected
		consoleWin, chatWin = oldConsole, oldChat
	})
	consoleWin, chatWin = nil, nil
	resetCommandStateForTest(t, 1)
	flow := eui.NewColumn()
	startMessageCommand(flow, "/status")
	if got := string(inputText); got != "/status " || inputPos != len([]rune(got)) || !inputActive || selectedMessageInput != flow {
		t.Fatalf("draft %q, cursor %d, active %v", got, inputPos, inputActive)
	}
	if primarySession.commands.pending != "" || len(primarySession.commands.queue) != 0 {
		t.Fatal("command picker sent the draft")
	}
}

func TestTextCommandDescriptionsCoverBuiltInCommands(t *testing.T) {
	for _, name := range goThoomTextCommands {
		if got := goThoomCommandDescriptions[name]; got == "" {
			t.Errorf("goThoom command /%s has no explanation", name)
		}
	}
	for _, name := range serverCommandNames {
		if got := serverCommandDescriptions[name]; got == "" {
			t.Errorf("server command /%s has no explanation", name)
		}
	}
	if got := goThoomCommandDescriptions["play"]; got != "Play a bard tune with an instrument and Clan Lord note syntax." {
		t.Errorf("/play explanation = %q", got)
	}
	if got := textCommandDescription(textCommandGroup{typeName: "Script", source: "Wave Helper"}, "/wave"); got != "Run the command registered by Wave Helper." {
		t.Errorf("script explanation = %q", got)
	}
	if got := textCommandDescription(textCommandGroup{typeName: "Macro", source: "travel.mac"}, "town"); got != "Run this text macro from travel.mac." {
		t.Errorf("macro explanation = %q", got)
	}
}

func TestTextCommandSyntaxUsesDimmedArgumentSignatures(t *testing.T) {
	tests := []struct {
		group   textCommandGroup
		command string
		want    string
	}{
		{textCommandGroup{typeName: "Server"}, "/give", " <person> <amount>"},
		{textCommandGroup{typeName: "Client"}, "/play", " <instrument> <notes>"},
		{textCommandGroup{typeName: "Server"}, "/money", ""},
		{textCommandGroup{typeName: "Script"}, "/custom", " [arguments]"},
		{textCommandGroup{typeName: "Macro"}, "travel", " [arguments]"},
	}
	for _, test := range tests {
		if got := textCommandSyntax(test.group, test.command); got != test.want {
			t.Errorf("syntax for %s %s = %q, want %q", test.group.typeName, test.command, got, test.want)
		}
	}
}

func TestInputBarHelpCoversCompletionHistoryAndWordEditing(t *testing.T) {
	want := map[string]bool{"Tab": false, "Up / Down": false, "Ctrl+Backspace": false, "Right-click": false}
	for _, entry := range inputBarHelpEntries {
		if _, ok := want[entry.shortcut]; ok && entry.description != "" {
			want[entry.shortcut] = true
		}
	}
	for shortcut, found := range want {
		if !found {
			t.Errorf("missing input-bar help for %s", shortcut)
		}
	}
}

func TestCommandPickerUsesAvailableSpaceAndWrapsInputHelp(t *testing.T) {
	initFont()
	oldWidth, oldHeight := eui.ScreenSize()
	oldScale := eui.UIScale()
	eui.SetScreenSize(800, 900)
	eui.SetUIScale(2)
	picker := newCommandPicker(nil, nil)
	t.Cleanup(func() {
		picker.win.RemoveWindow()
		eui.SetScreenSize(oldWidth, oldHeight)
		eui.SetUIScale(oldScale)
	})
	if picker.list.Size != (eui.Point{X: 360, Y: 330}) {
		t.Fatalf("command picker list size = %v, want {360 330}", picker.list.Size)
	}
	var copyHelp *eui.ItemData
	var giveCommand *eui.ItemData
	for _, item := range picker.list.Contents {
		if strings.HasPrefix(item.Text, "Ctrl/Cmd+C") {
			copyHelp = item
		}
		if strings.HasPrefix(item.Text, "/give\n") {
			giveCommand = item
		}
	}
	if copyHelp == nil {
		t.Fatal("command picker is missing copy/paste input help")
	}
	if giveCommand == nil || giveCommand.Prediction != " <person> <amount>" || strings.Contains(giveCommand.Text, "<person>") {
		t.Fatalf("give command did not keep its syntax in dimmed prediction text: %#v", giveCommand)
	}
	copyHelp.GetSize()
	if !strings.Contains(copyHelp.Text, "\n") {
		t.Fatalf("copy/paste input help did not wrap: %q", copyHelp.Text)
	}
	if got, want := copyHelp.GetSize().X, (picker.list.Size.X-eui.ScrollbarWidth()/eui.UIScale())*eui.UIScale(); got > want {
		t.Fatalf("wrapped input help width = %v, exceeds %v", got, want)
	}
}
