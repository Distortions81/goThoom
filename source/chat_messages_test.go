package main

import (
	"os"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"

	"golang.org/x/image/font/gofont/goregular"
)

func TestChatWindowDefersClosedUpdatesUntilOpen(t *testing.T) {
	if err := eui.EnsureFontSource(goregular.TTF); err != nil {
		t.Fatalf("load test font: %v", err)
	}
	originalWindow := chatWin
	originalList := chatList
	originalChatInput := chatInputFlow
	chatLog.mu.Lock()
	originalEntries := append([]timedMessage(nil), chatLog.entries...)
	originalMax := chatLog.max
	chatLog.mu.Unlock()
	originalMessagesToConsole := gs.MessagesToConsole
	t.Cleanup(func() {
		if chatWin != nil && chatWin != originalWindow {
			chatWin.RemoveWindow()
		}
		chatWin = originalWindow
		chatList = originalList
		chatInputFlow = originalChatInput
		chatLog = messageLog{entries: originalEntries, max: originalMax}
		gs.MessagesToConsole = originalMessagesToConsole
	})

	chatWin = nil
	chatList = nil
	chatLog = messageLog{max: maxChatMessages}
	gs.MessagesToConsole = false
	if err := makeChatWindow(); err != nil {
		t.Fatalf("make chat window: %v", err)
	}
	chatLog.Add("Bob says, hello")
	updateChatWindow()
	if len(chatList.Contents) != 0 {
		t.Fatal("closed Chat window rebuilt its message rows")
	}
	chatWin.MarkOpen()
	if len(chatList.Contents) != 1 || !strings.Contains(chatList.Contents[0].Text, "Bob says, hello") {
		t.Fatalf("Chat contents after opening = %#v", chatList.Contents)
	}
}

func TestMessageWindowsBindToSelectedSession(t *testing.T) {
	initFont()
	originalSessions := appSessions
	originalSettings := gs
	originalChat, originalChatList, originalChatInput := chatWin, chatList, chatInputFlow
	originalConsole, originalMessages, originalInput := consoleWin, messagesFlow, inputFlow
	originalChatModel, originalConsoleModel := chatWindowMessages, consoleWindowMessages
	originalChatRender, originalConsoleRender := chatRenderSession, consoleRenderSession
	originalChatCache, originalConsoleCache := chatTextWrapCache, consoleTextWrapCache
	consoleLog.mu.Lock()
	originalConsoleEntries := append([]timedMessage(nil), consoleLog.entries...)
	originalConsoleSequence := consoleLog.nextSeq
	consoleLog.mu.Unlock()
	t.Cleanup(func() {
		if chatWin != nil && chatWin != originalChat {
			chatWin.RemoveWindow()
		}
		if consoleWin != nil && consoleWin != originalConsole {
			consoleWin.RemoveWindow()
		}
		appSessions = originalSessions
		gs = originalSettings
		chatWin, chatList, chatInputFlow = originalChat, originalChatList, originalChatInput
		consoleWin, messagesFlow, inputFlow = originalConsole, originalMessages, originalInput
		chatWindowMessages, consoleWindowMessages = originalChatModel, originalConsoleModel
		chatRenderSession, consoleRenderSession = originalChatRender, originalConsoleRender
		chatTextWrapCache, consoleTextWrapCache = originalChatCache, originalConsoleCache
		consoleLog.mu.Lock()
		consoleLog.entries, consoleLog.nextSeq = originalConsoleEntries, originalConsoleSequence
		consoleLog.mu.Unlock()
	})

	gs.MessagesToConsole = false
	manager := newSessionManager(primarySession)
	slots := manager.enableMulti()
	second, third := slots[1], slots[2]
	second.setCharacterName("Second Hero")
	third.setCharacterName("Third Hero")
	second.publishChat("Second says, hello", messageTextTypeSay)
	second.publishConsole("Second server line", messageTextTypeSystem)
	third.publishChat("Third says, hello", messageTextTypeSay)
	third.publishConsole("Third server line", messageTextTypeSystem)
	appSessions = manager
	if !manager.selectSession(second.ID()) {
		t.Fatal("select second session")
	}

	chatWin, chatList, chatInputFlow = nil, nil, nil
	consoleWin, messagesFlow, inputFlow = nil, nil, nil
	chatWindowMessages, consoleWindowMessages = messageWindowState{}, messageWindowState{}
	chatRenderSession, consoleRenderSession = 0, 0
	if err := makeChatWindow(); err != nil {
		t.Fatalf("make chat window: %v", err)
	}
	makeConsoleWindow()
	chatWin.MarkOpen()
	consoleWin.MarkOpen()
	updateChatWindow()
	updateConsoleWindow()
	if chatWin.Title != "Second Hero - Chat" || len(chatList.Contents) != 1 || !strings.Contains(chatList.Contents[0].Text, "Second says, hello") {
		t.Fatalf("second Chat binding = title %q rows %#v", chatWin.Title, chatList.Contents)
	}
	if consoleWin.Title != "Second Hero - Console" || len(messagesFlow.Contents) != 1 || !strings.Contains(messagesFlow.Contents[0].Text, "Second server line") {
		t.Fatalf("second Console binding = title %q rows %#v", consoleWin.Title, messagesFlow.Contents)
	}

	if !manager.selectSession(third.ID()) {
		t.Fatal("select third session")
	}
	updateChatWindow()
	updateConsoleWindow()
	if chatWin.Title != "Third Hero - Chat" || len(chatList.Contents) != 1 || !strings.Contains(chatList.Contents[0].Text, "Third says, hello") {
		t.Fatalf("third Chat binding = title %q rows %#v", chatWin.Title, chatList.Contents)
	}
	if consoleWin.Title != "Third Hero - Console" || len(messagesFlow.Contents) != 1 || !strings.Contains(messagesFlow.Contents[0].Text, "Third server line") {
		t.Fatalf("third Console binding = title %q rows %#v", consoleWin.Title, messagesFlow.Contents)
	}
}

func TestChatWindowAfterCombinedMode(t *testing.T) {
	initFont()
	originalSettings, originalDirty := gs, settingsDirty
	originalChat, originalList, originalChatInput := chatWin, chatList, chatInputFlow
	originalSelected := selectedMessageInput
	originalText, originalPos, originalActive := inputText, inputPos, inputActive
	chatLog.mu.Lock()
	originalEntries := append([]timedMessage(nil), chatLog.entries...)
	originalSequence := chatLog.nextSeq
	chatLog.mu.Unlock()
	originalConsole, originalMessages, originalInput := consoleWin, messagesFlow, inputFlow
	originalModel, originalCache := chatWindowMessages, chatTextWrapCache
	originalGame, originalInventory, originalPlayers, originalHUD, originalMovie := gameWin, inventoryWin, playersWin, hudWin, movieWin
	w, h := eui.ScreenSize()
	eui.SetScreenSize(1400, 1000)
	gameWin, inventoryWin, playersWin, hudWin, movieWin = nil, nil, nil, nil, nil
	consoleWin = nil
	chatWin, chatList = nil, nil
	chatWindowMessages = messageWindowState{}
	chatTextWrapCache = textWindowWrapCache{}
	gs = gsdef
	t.Cleanup(func() {
		if chatWin != nil {
			chatWin.RemoveWindow()
		}
		if consoleWin != nil {
			consoleWin.RemoveWindow()
		}
		chatWin, chatList, chatInputFlow = originalChat, originalList, originalChatInput
		selectedMessageInput = originalSelected
		inputText, inputPos, inputActive = originalText, originalPos, originalActive
		chatLog.mu.Lock()
		chatLog.entries, chatLog.nextSeq = originalEntries, originalSequence
		chatLog.mu.Unlock()
		consoleWin, messagesFlow, inputFlow = originalConsole, originalMessages, originalInput
		chatWindowMessages, chatTextWrapCache = originalModel, originalCache
		gameWin, inventoryWin, playersWin, hudWin, movieWin = originalGame, originalInventory, originalPlayers, originalHUD, originalMovie
		gs, settingsDirty = originalSettings, originalDirty
		eui.SetScreenSize(w, h)
	})
	makeConsoleWindow()
	combine := newCombineMessagesCheckbox(310)
	for layout := TiledLayout(-1); layout <= TiledLayoutFullMessagesAbove; layout++ {
		tiled := layout >= TiledLayoutCenter
		gs.TiledWindows = tiled
		gs.TiledLayout = max(layout, TiledLayoutCenter)
		combine.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true, Item: combine})
		combine.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: false, Item: combine})
		if chatWin == nil || !chatWin.IsOpen() || !gs.ChatWindow.Open {
			t.Fatalf("Chat did not reopen (tiled=%v, layout=%v)", tiled, layout)
		}
		chatLog.Add("A player says, after switching modes")
		updateChatWindow()
		if len(chatList.Contents) == 0 || chatList.Size.X <= 1 || chatList.Size.Y <= 1 {
			t.Fatalf("Chat has no visible message area (tiled=%v, layout=%v): list=%+v", tiled, layout, chatList.Size)
		}
		if item := messageInputItem(chatInputFlow); item == nil || chatInputFlow.Size.Y <= 0 {
			t.Fatalf("Chat is missing its input bar (tiled=%v, layout=%v)", tiled, layout)
		}
		inputText, inputPos, inputActive = []rune("hello from Chat"), 5, true
		selectedMessageInput = chatInputFlow
		updateMessageInputWindows()
		chatInput := currentMessageInputItem()
		if chatInput != messageInputItem(chatInputFlow) || strings.ReplaceAll(chatInput.Text, "\n", "") != string(inputText) {
			t.Fatal("Chat did not display the shared draft")
		}
		if strings.ReplaceAll(messageInputItem(inputFlow).Text, "\n", "") != string(inputText) {
			t.Fatal("Console draft diverged from Chat")
		}
		if typingInUI() {
			t.Fatal("Chat input was mistaken for another UI text field")
		}
		chatInput.SelectStart, chatInput.SelectEnd = 0, 5
		if legacyMacroInputSelection() != "hello" {
			t.Fatal("selection came from the wrong input bar")
		}
		consoleWin.Close()
		if currentMessageInputItem() != chatInput {
			t.Fatal("Chat input requires Console to be open")
		}
		consoleWin.MarkOpen()
		combine.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true, Item: combine})
		if currentMessageInputItem() != messageInputItem(inputFlow) {
			t.Fatal("combined mode did not return input to Console")
		}
		combine.Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: false, Item: combine})
	}
	messageInputItem(inputFlow).Focused = false
	messageInputItem(chatInputFlow).Focused = true
	selectedMessageInput = inputFlow
	if !captureMessageInputFocus() || currentMessageInputItem() != messageInputItem(chatInputFlow) {
		t.Fatal("clicking Chat did not move input ownership")
	}
	inputActive = false
	inputText, inputPos = nil, 0
	messageInputItem(chatInputFlow).Text = "[Press Enter To Type]"
	if !captureMessageInputFocus() || !inputActive || messageInputItem(chatInputFlow).Text != "" {
		t.Fatal("activating Chat copied its placeholder into the outgoing draft")
	}
	inputText, inputPos = []rune("hello from Chat"), 5
	updateMessageInputWindows()
	if dir := os.Getenv("GOTHOOM_RENDER_CHAT"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		consoleWin.Close()
		game := &scriptWindowRenderGame{window: chatWin, prefix: "chat", dir: dir}
		if err := ebiten.RunGame(game); err != nil {
			t.Fatal(err)
		}
		if game.err != nil {
			t.Fatal(game.err)
		}
	}
}

func TestIsSelfChatMessage(t *testing.T) {
	playerName = "Hero"
	cases := []struct {
		msg  string
		want bool
	}{
		{"Hero says, hello there", true},
		{"(Hero waves)", true},
		{"Hero yells, hey!", true},
		{"Bob says, hi", false},
		{"You are sharing experiences with Bob.", false},
		{"Hero has fallen", false},
	}
	for _, c := range cases {
		if got := isSelfChatMessage(c.msg); got != c.want {
			t.Errorf("isSelfChatMessage(%q) = %v; want %v", c.msg, got, c.want)
		}
	}
}

func TestChatMessageBlocked(t *testing.T) {
	players = make(map[string]*Player)
	chatLog = messageLog{max: maxChatMessages}
	p := getPlayer("Bob")
	playersMu.Lock()
	p.Blocked = true
	playersMu.Unlock()
	chatMessage("Bob says, hi")
	if len(getChatMessages()) != 0 {
		t.Fatalf("expected no messages")
	}
}

func TestChatMessageIgnored(t *testing.T) {
	players = make(map[string]*Player)
	chatLog = messageLog{max: maxChatMessages}
	p := getPlayer("Bob")
	playersMu.Lock()
	p.Ignored = true
	playersMu.Unlock()
	chatMessage("Bob says, hi")
	if len(getChatMessages()) != 0 {
		t.Fatalf("expected no messages")
	}
}

func TestChatSpeakerNPCWithDescriptor(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"(Town Crier) says, hello", "Town Crier"},
		{"(Boat Seller) yells, boats", "Boat Seller"},
		{"Goblin says, hi", "Goblin"},
		{"Captain Barnac says Ah, Malcom.", "Captain Barnac"},
		{"High Priestess Aria whispers, hush.", "High Priestess Aria"},
	}
	for _, c := range cases {
		if got := chatSpeaker(c.msg); got != c.want {
			t.Fatalf("chatSpeaker(%q) = %q; want %q", c.msg, got, c.want)
		}
	}
}
