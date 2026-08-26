package main

import (
	"embed"
	"path/filepath"
	"strings"
	"testing"
)

//go:embed testdata/legacy_macros/web/*.mac
var legacyMacroWebCorpus embed.FS

// These sources are public macro files that were written for the reference C
// client. Keep this list explicit so a fixture cannot silently disappear from
// the compatibility corpus.
func TestLegacyMacroWebCorpusParses(t *testing.T) {
	fixtures := []struct {
		name           string
		diagnosticLine int
		diagnosticText string
	}{
		{name: "official-example.mac"},
		{name: "abbreviations.mac"},
		{name: "keys.mac"},
		{name: "quickchain.mac"},
		{name: "sunstone.mac"},
		{name: "dances.mac"},
		{name: "directions.mac"},
		{name: "clump-scanner.mac"},
		// The published source has an extra closing brace. The reference C
		// parser reports it and keeps loading the remaining declarations.
		{name: "clump-omega-zu.mac", diagnosticLine: 286, diagnosticText: "unexpected closing brace"},
		{name: "clump-dice-roll.mac"},
		{name: "clump-bard-instruments.mac"},
		{name: "clump-rangery.mac"},
		{name: "fastfeet-language-macros.mac"},
		{name: "magnic-pull-push.mac"},
		{name: "magnic-pet-summoning.mac"},
		{name: "gorvin-dynamicsharecads.mac"},
		{name: "gorvin-kudzu-lord.mac"},
		{name: "gorvin-coin-lord.mac"},
		{name: "gorvin-last-hit-counter.mac"},
		{name: "gorvin-ite-boost-timer.mac"},
		{name: "gorvin-right-clicker.mac"},
		{name: "gorvin-right-clicker-template.mac"},
		{name: "gorvin-right-clicker-champion.mac"},
		{name: "gorvin-right-clicker-ranger.mac"},
		{name: "gorvin-right-clicker-asklepian.mac"},
		{name: "gorvin-right-clicker-ctf.mac"},
		{name: "gorvin-macro-chess.mac"},
		{name: "gorvin-macro-tetris.mac"},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			path := filepath.Join("testdata", "legacy_macros", "web", fixture.name)
			text, err := legacyMacroWebCorpus.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			program := parseLegacyMacroSources([]legacyMacroSource{{
				Name: fixture.name,
				Path: path,
				Text: string(text),
			}})
			if fixture.diagnosticText == "" && len(program.Diagnostics) != 0 {
				t.Fatalf("diagnostics = %#v", program.Diagnostics)
			}
			if fixture.diagnosticText != "" {
				if len(program.Diagnostics) != 1 ||
					program.Diagnostics[0].Location.Line != fixture.diagnosticLine ||
					program.Diagnostics[0].Message != fixture.diagnosticText {
					t.Fatalf("diagnostics = %#v, want line %d: %s", program.Diagnostics, fixture.diagnosticLine, fixture.diagnosticText)
				}
			}
			if len(program.Macros) == 0 {
				t.Fatal("no macro declarations parsed")
			}
			t.Logf("parsed %d macro declarations", len(program.Macros))
		})
	}
}

func TestLegacyMacroWebCorpusOfficialMovement(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "official-example.mac")
	var moves []legacyMacroMove
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Move: func(move legacyMacroMove) { moves = append(moves, move) },
	})

	if started, allowDefault := runtime.triggerKey("1", legacyMacroModShift|legacyMacroModNumpad, 0); !started || allowDefault {
		t.Fatalf("shift-numpad-1 = (%t, %t), want (true, false)", started, allowDefault)
	}
	if started, allowDefault := runtime.triggerKey("5", legacyMacroModShift|legacyMacroModNumpad, 0); !started || allowDefault {
		t.Fatalf("shift-numpad-5 = (%t, %t), want (true, false)", started, allowDefault)
	}

	want := []legacyMacroMove{
		{Direction: legacyMacroMoveSouthWest},
		{Direction: legacyMacroMoveStop},
	}
	if len(moves) != len(want) {
		t.Fatalf("moves = %#v, want %#v", moves, want)
	}
	for index := range want {
		if moves[index] != want[index] {
			t.Fatalf("move %d = %#v, want %#v", index, moves[index], want[index])
		}
	}
}

func TestLegacyMacroWebCorpusOfficialDynamicTextAndEquipment(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "official-example.mac")
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
		ResolveState: func(name string) (string, bool) {
			if name == "@my.right_item" {
				return "Sun Sword", true
			}
			return "", false
		},
	})

	if !runtime.triggerExpression("accent cats", 0) {
		t.Fatal("accent expression macro did not start")
	}
	for frame := int64(0); frame <= 4; frame++ {
		runtime.advance(frame)
	}
	if got, want := sent, []string{"khatz "}; !equalStrings(got, want) {
		t.Fatalf("accent sends = %#v, want %#v", got, want)
	}

	if !runtime.triggerExpression("time", 5) {
		t.Fatal("time expression macro did not start")
	}
	for frame := int64(5); frame <= 10; frame++ {
		runtime.advance(frame)
	}
	want := []string{"khatz ", "/action looks at the sky.", "/equip Green Token", "/use", "/equip Sun Sword"}
	if !equalStrings(sent, want) {
		t.Fatalf("official sends = %#v, want %#v", sent, want)
	}
}

func TestLegacyMacroWebCorpusSmallUtilityMacros(t *testing.T) {
	tests := []struct {
		fixture string
		input   string
		want    string
	}{
		{fixture: "abbreviations.mac", input: "aa waves", want: "/action waves"},
		{fixture: "sunstone.mac", input: "/tbeer hello", want: "/thinkto Beer hello"},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			program := legacyMacroWebFixtureProgram(t, test.fixture)
			var sent []string
			runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
				SendText: func(text string) { sent = append(sent, text) },
			})
			if !runtime.triggerExpression(test.input, 0) {
				t.Fatalf("%q did not start", test.input)
			}
			runtime.advance(1)
			if got, want := sent, []string{test.want}; !equalStrings(got, want) {
				t.Fatalf("sent = %#v, want %#v", got, want)
			}
			if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 0 {
				t.Fatalf("runtime diagnostics = %#v", diagnostics)
			}
		})
	}
}

func TestLegacyMacroWebCorpusKeysAndQuickchainHelp(t *testing.T) {
	keysProgram := legacyMacroWebFixtureProgram(t, "keys.mac")
	var keyMessages []string
	keysRuntime := newLegacyMacroRuntimeWithHooks(keysProgram, legacyMacroRuntimeHooks{
		Message: func(message string) { keyMessages = append(keyMessages, message) },
		ResolveState: func(name string) (string, bool) {
			if name == "@my.left_item" {
				return "Nothing", true
			}
			return "", false
		},
	})
	if !keysRuntime.triggerExpression("/keys", 0) {
		t.Fatal("/keys did not start")
	}
	if len(keyMessages) != 11 || keyMessages[0] != "Usage: /keys action." {
		t.Fatalf("key help messages = %#v", keyMessages)
	}
	if diagnostics := keysRuntime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("keys diagnostics = %#v", diagnostics)
	}

	quickchainProgram := legacyMacroWebFixtureProgram(t, "quickchain.mac")
	var quickchainMessages []string
	quickchainRuntime := newLegacyMacroRuntimeWithHooks(quickchainProgram, legacyMacroRuntimeHooks{
		Message: func(message string) { quickchainMessages = append(quickchainMessages, message) },
	})
	if !quickchainRuntime.triggerExpression("testqchain", 0) {
		t.Fatal("testqchain did not start")
	}
	if got, want := quickchainMessages, []string{"chainlock 10"}; !equalStrings(got, want) {
		t.Fatalf("quickchain messages = %#v, want %#v", got, want)
	}
	if diagnostics := quickchainRuntime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("quickchain diagnostics = %#v", diagnostics)
	}
}

func TestLegacyMacroWebCorpusDancesEndLabelIsNoOp(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "dances.mac")
	var messages, inserted []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message:    func(message string) { messages = append(messages, message) },
		InsertText: func(text string) { inserted = append(inserted, text) },
	})

	if !runtime.triggerExpression("/dc", 0) {
		t.Fatal("/dc expression macro did not start")
	}
	for frame := int64(0); frame <= 64; frame++ {
		runtime.advance(frame)
	}
	if len(messages) != 8 {
		t.Fatalf("messages = %#v, want eight dance-help messages", messages)
	}
	if len(inserted) != 0 {
		t.Fatalf("end label inserted text %#v", inserted)
	}
}

func TestLegacyMacroWebCorpusExpressionAndCall(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "directions.mac")
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})

	if !runtime.triggerExpression("/dir DI pit", 0) {
		t.Fatal("/dir expression macro did not start")
	}
	want := []string{
		"Directions to The Pit:",
		"Land at N beach",
		"Go E, S, S",
		"Go E to The Pit snell",
		"To enter The Pit, go N in NW corner along the E rock wall, then go S where the ground turns dark grey",
		"",
	}
	if len(messages) != len(want) {
		t.Fatalf("messages = %#v, want %#v", messages, want)
	}
	for index := range want {
		if messages[index] != want[index] {
			t.Fatalf("message %d = %q, want %q", index, messages[index], want[index])
		}
	}
}

func TestLegacyMacroWebCorpusScannerDynamicReaction(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "clump-scanner.mac")
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
	})
	if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("setup diagnostics = %#v", diagnostics)
	}
	runtime.globals["pointer"] = "6"
	execution, err := runtime.startFunction("reaction")
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)
	runtime.advance(1)
	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want successful completion", execution)
	}
	if got, want := sent, []string{"/equip bag of kudzu seedlings 1"}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
}

func TestLegacyMacroWebCorpusOmegaZuHelp(t *testing.T) {
	path := filepath.Join("testdata", "legacy_macros", "web", "clump-omega-zu.mac")
	text, err := legacyMacroWebCorpus.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	program := parseLegacyMacroSources([]legacyMacroSource{{Name: "clump-omega-zu.mac", Path: path, Text: string(text)}})
	if len(program.Diagnostics) != 1 || program.Diagnostics[0].Message != "unexpected closing brace" {
		t.Fatalf("parse diagnostics = %#v", program.Diagnostics)
	}
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})
	if !runtime.triggerExpression("/setzu ?", 0) {
		t.Fatal("/setzu did not start")
	}
	for frame := int64(0); frame <= 6; frame++ {
		runtime.advance(frame)
	}
	if got, want := messages, []string{
		"*The following variables are available to set using this command:",
		"bag [?|(number)], maxbag ?|(number)",
	}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
	if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("runtime diagnostics = %#v", diagnostics)
	}
}

func TestLegacyMacroWebCorpusLongRunningGamesAndSharecads(t *testing.T) {
	shareProgram := legacyMacroWebFixtureProgram(t, "gorvin-dynamicsharecads.mac")
	var shareMessages []string
	shareRuntime := newLegacyMacroRuntimeWithHooks(shareProgram, legacyMacroRuntimeHooks{
		Message: func(message string) { shareMessages = append(shareMessages, message) },
		ResolveState: func(name string) (string, bool) {
			if name == "@env.textlog" {
				return "", true
			}
			return "", false
		},
	})
	if !shareRuntime.triggerExpression("/shcads", 0) {
		t.Fatal("/shcads did not start")
	}
	for frame := int64(0); frame <= 3; frame++ {
		shareRuntime.advance(frame)
	}
	if !shareRuntime.triggerExpression("/shcads", 4) {
		t.Fatal("second /shcads did not start")
	}
	for frame := int64(4); frame <= 8; frame++ {
		shareRuntime.advance(frame)
	}
	if got, want := shareMessages, []string{"* Sharecads is now on.", "* Sharecads is now off."}; !equalStrings(got, want) {
		t.Fatalf("share messages = %#v, want %#v", got, want)
	}
	if diagnostics := shareRuntime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("share diagnostics = %#v", diagnostics)
	}

	tetrisProgram := legacyMacroWebFixtureProgram(t, "gorvin-macro-tetris.mac")
	tetrisRuntime := newLegacyMacroRuntimeWithHooks(tetrisProgram, legacyMacroRuntimeHooks{
		RandomInt: func(int) int { return 0 },
	})
	if !tetrisRuntime.triggerExpression("/tetris", 0) {
		t.Fatal("/tetris did not start")
	}
	for frame := int64(0); frame <= 5; frame++ {
		tetrisRuntime.advance(frame)
	}
	if got, want := tetrisRuntime.globalsSnapshot()["gtetris[0]"], "|~~~~~~~~~~|"; got != want {
		t.Fatalf("gTetris[0] = %q, want %q", got, want)
	}
	if !tetrisRuntime.triggerExpression("/tetris", 6) {
		t.Fatal("second /tetris did not start")
	}
	for frame := int64(6); frame <= 12; frame++ {
		tetrisRuntime.advance(frame)
	}
	if diagnostics := tetrisRuntime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("tetris diagnostics = %#v", diagnostics)
	}
	tetrisRuntime.cancelAll()
}

func TestLegacyMacroWebCorpusSharecadsSharesDetectedHealer(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "gorvin-dynamicsharecads.mac")
	textLog := "Welcome."
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
		ResolveState: func(name string) (string, bool) {
			if name == "@env.textlog" {
				return textLog, true
			}
			return "", false
		},
	})
	if !runtime.triggerExpression("/shcads", 0) {
		t.Fatal("/shcads did not start")
	}

	// The classic client retains the leading MacRoman bullet on healing
	// messages in @env.textLog. Dynamic Sharecads deliberately uses >= so the
	// first word can contain that marker while still matching "You".
	textLog = "•You sense healing energy from Bob."
	for frame := int64(1); frame <= 40 && len(sent) == 0; frame++ {
		runtime.advance(frame)
	}
	if got, want := sent, []string{"/share Bob"}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
	if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("runtime diagnostics = %#v", diagnostics)
	}
	runtime.cancelAll()
}

func TestLegacyMacroWebCorpusChessHelp(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "gorvin-macro-chess.mac")
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})

	if !runtime.triggerExpression("/chess ?", 0) {
		t.Fatal("/chess expression macro did not start")
	}
	if len(messages) != 14 {
		t.Fatalf("help message count = %d, want 14: %#v", len(messages), messages)
	}
	if messages[0] != "* - - - - - - - - - - - *" || messages[len(messages)-1] != "* - - - - - - - - - - - *" {
		t.Fatalf("help divider messages = %q, %q", messages[0], messages[len(messages)-1])
	}
	if messages[1] != "/chess - displays current chess board." || messages[9] != "/chessmove <directions> - selects where to move the piece." {
		t.Fatalf("help messages do not match source: %#v", messages)
	}
}

func TestLegacyMacroWebCorpusRightClickerUsesClassicSingleStageExpansion(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "gorvin-right-clicker.mac")
	var sent, messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
		Message:  func(message string) { messages = append(messages, message) },
		ResolveState: func(name string) (string, bool) {
			switch name {
			case "@my.name":
				return "Gaia", true
			case "@my.simple_name":
				return "Gaia", true
			case "@my.right_item":
				return "Sun Sword", true
			case "@my.left_item":
				return "Nothing", true
			}
			return "", false
		},
	})

	if !runtime.triggerExpression("/fightmode", 0) {
		t.Fatal("/fightmode did not start")
	}
	event := legacyMacroClickEvent{
		Name:      "Bob Jones",
		HasName:   true,
		OnPlayer:  true,
		Button:    2,
		Chord:     2,
		HasButton: true,
		HasChord:  true,
	}
	if started, _ := runtime.triggerClick(event, 1); !started {
		t.Fatal("right click did not start")
	}
	for frame := int64(1); frame <= 8; frame++ {
		runtime.advance(frame)
	}
	if len(sent) != 0 {
		t.Fatalf("sent = %#v, want no recursively dereferenced action", sent)
	}
	if len(messages) != 1 || messages[0] != "* Using Fighter right-click settings." {
		t.Fatalf("messages = %#v", messages)
	}
	if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 1 ||
		!strings.Contains(diagnostics[0].Message, `function "gRC_right_click_player" is not defined`) {
		t.Fatalf("runtime diagnostics = %#v", diagnostics)
	}
	runtime.cancelAll()
}

func TestLegacyMacroWebCorpusAdditionalGorvinCommands(t *testing.T) {
	tests := []struct {
		fixture string
		input   string
		want    []string
	}{
		{
			fixture: "gorvin-kudzu-lord.mac",
			input:   "/zutrans",
			want: []string{
				"/zutrans <amount> from <bag/pack/sack> <#> to <bag/pack/sack> <#> - Transfers seeds from one container to another.",
				"Example: \"/zutrans 5 from bag 3 to pack 1\" to transfer 5 seeds from bag #3 to pack #1.",
			},
		},
		{
			fixture: "gorvin-ite-boost-timer.mac",
			input:   "/boost",
			want:    []string{"* You must have an earth mineral equipped to boost."},
		},
		{
			fixture: "gorvin-right-clicker-ctf.mac",
			input:   "/ctf ?",
			want:    []string{"* Right-click after selecting a Sub-Subclass to initialize controls."},
		},
	}

	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			program := legacyMacroWebFixtureProgram(t, test.fixture)
			var messages []string
			runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
				Message: func(message string) { messages = append(messages, message) },
			})
			if !runtime.triggerExpression(test.input, 0) {
				t.Fatalf("%q did not start", test.input)
			}
			for frame := int64(0); frame <= 8; frame++ {
				runtime.advance(frame)
			}
			if !equalStrings(messages, test.want) {
				t.Fatalf("messages = %#v, want %#v", messages, test.want)
			}
			if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 0 {
				t.Fatalf("runtime diagnostics = %#v", diagnostics)
			}
		})
	}

	coinProgram := legacyMacroWebFixtureProgram(t, "gorvin-coin-lord.mac")
	var coinMessages []string
	coinRuntime := newLegacyMacroRuntimeWithHooks(coinProgram, legacyMacroRuntimeHooks{
		Message: func(message string) { coinMessages = append(coinMessages, message) },
	})
	if !coinRuntime.triggerExpression("/cw ?", 0) {
		t.Fatal("/cw ? did not start")
	}
	coinRuntime.advance(0)
	if len(coinMessages) != 9 || coinMessages[0] != "* ----------------- *" || coinMessages[len(coinMessages)-1] != "* ----------------- *" {
		t.Fatalf("coin help messages = %#v", coinMessages)
	}
	if diagnostics := coinRuntime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("coin runtime diagnostics = %#v", diagnostics)
	}

	lastProgram := legacyMacroWebFixtureProgram(t, "gorvin-last-hit-counter.mac")
	var lastMessages []string
	lastRuntime := newLegacyMacroRuntimeWithHooks(lastProgram, legacyMacroRuntimeHooks{
		Message: func(message string) { lastMessages = append(lastMessages, message) },
	})
	if !lastRuntime.triggerExpression("/lastcount", 0) {
		t.Fatal("/lastcount did not start")
	}
	lastRuntime.advance(0)
	if got, want := lastMessages, []string{
		"* Now counting kills on: Haremau Kitten",
		"Use /lasts+ and /lasts- to manually adjust kill counts if needed.",
	}; !equalStrings(got, want) {
		t.Fatalf("last-counter messages = %#v, want %#v", got, want)
	}
	if diagnostics := lastRuntime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("last-counter runtime diagnostics = %#v", diagnostics)
	}
	lastRuntime.cancelAll()
}

func TestLegacyMacroWebCorpusRightClickerAddOnSetups(t *testing.T) {
	tests := []struct {
		fixture  string
		function string
		message  string
		global   string
		want     string
	}{
		{
			fixture:  "gorvin-right-clicker-template.mac",
			function: "RC_template",
			message:  "* Using <INSERT SET NAME HERE> right-click settings.",
			global:   "grc_init",
			want:     "1",
		},
		{
			fixture:  "gorvin-right-clicker-champion.mac",
			function: "RC_champion_setup",
			message:  "* Using Champion right-click settings.",
			global:   "grc_double_right_click_ground",
			want:     "RC_swap_ite",
		},
		{
			fixture:  "gorvin-right-clicker-ranger.mac",
			function: "RC_ranger_setup",
			message:  "* Using Ranger right-click settings.",
			global:   "grc_double_right_click_self",
			want:     "RC_morph",
		},
		{
			fixture:  "gorvin-right-clicker-asklepian.mac",
			function: "RC_asklepian_setup",
			message:  "* Using Asklepian right-click settings.",
			global:   "grc_right_click_player",
			want:     "RC_cad",
		},
	}

	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			program := legacyMacroWebFixtureProgram(t, test.fixture)
			var messages []string
			runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
				Message: func(message string) { messages = append(messages, message) },
			})
			execution, err := runtime.startFunction(test.function)
			if err != nil {
				t.Fatal(err)
			}
			runtime.advance(0)
			if !execution.complete || execution.diagnostic != nil {
				t.Fatalf("execution = %#v, want successful completion", execution)
			}
			if got, want := messages, []string{test.message}; !equalStrings(got, want) {
				t.Fatalf("messages = %#v, want %#v", got, want)
			}
			if got := runtime.globalsSnapshot()[test.global]; got != test.want {
				t.Fatalf("%s = %q, want %q", test.global, got, test.want)
			}
			if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 0 {
				t.Fatalf("runtime diagnostics = %#v", diagnostics)
			}
		})
	}
}

func TestLegacyMacroWebCorpusCommunityCommands(t *testing.T) {
	tests := []struct {
		fixture    string
		input      string
		state      map[string]string
		frames     int64
		want       []string
		globalName string
		globalWant string
	}{
		{
			fixture: "clump-dice-roll.mac",
			input:   "/roll 2d10",
			state:   map[string]string{"@my.left_item": "Short Sword"},
			frames:  8,
			want:    []string{"/equip pouchofdice", "/useitem left 2d10", "/equip Short Sword"},
		},
		{
			fixture: "clump-bard-instruments.mac",
			input:   "/add Pine Flute",
			state:   map[string]string{"@my.left_item": "Short Sword"},
			frames:  15,
			want:    []string{"/equip instrument case", "/useitem instrument case /add Pine Flute", "/equip Short Sword"},
		},
		{
			fixture: "clump-rangery.mac",
			input:   "/judge Bear",
			want:    []string{"/useitem belt /judge Bear"},
		},
		{
			fixture:    "fastfeet-language-macros.mac",
			input:      "/sylvan Greetings",
			frames:     4,
			want:       []string{"/speak sylvan", "Greetings"},
			globalName: "current_lang",
			globalWant: "sylvan",
		},
		{
			fixture: "magnic-pet-summoning.mac",
			input:   "/cp Nimbus",
			state:   map[string]string{"@my.right_item": "Short Sword"},
			frames:  20,
			want: []string{
				"/bag /remove /exact shadow bell",
				"/equip shadow bell",
				"/use Nimbus",
				"/equip Short Sword",
				"/selectitem Shadow bell",
				"/bag /add",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			program := legacyMacroWebFixtureProgram(t, test.fixture)
			var sent []string
			runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
				SendText: func(text string) { sent = append(sent, text) },
				ResolveState: func(name string) (string, bool) {
					value, ok := test.state[name]
					return value, ok
				},
			})
			if !runtime.triggerExpression(test.input, 0) {
				t.Fatalf("%q did not start", test.input)
			}
			for frame := int64(0); frame <= test.frames; frame++ {
				runtime.advance(frame)
			}
			if !equalStrings(sent, test.want) {
				t.Fatalf("sent = %#v, want %#v", sent, test.want)
			}
			if test.globalName != "" {
				if got := runtime.globalsSnapshot()[test.globalName]; got != test.globalWant {
					t.Fatalf("%s = %q, want %q", test.globalName, got, test.globalWant)
				}
			}
			if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 0 {
				t.Fatalf("runtime diagnostics = %#v", diagnostics)
			}
		})
	}
}

func TestLegacyMacroWebCorpusMagnicShiftClick(t *testing.T) {
	program := legacyMacroWebFixtureProgram(t, "magnic-pull-push.mac")
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
	})
	event := legacyMacroClickEvent{
		Name:      "Bob Jones",
		HasName:   true,
		OnPlayer:  true,
		Button:    1,
		Chord:     1,
		HasButton: true,
		HasChord:  true,
		Modifiers: legacyMacroModShift,
	}
	if started, _ := runtime.triggerClick(event, 0); !started {
		t.Fatal("Shift-click pull did not start")
	}
	for frame := int64(0); frame <= 2; frame++ {
		runtime.advance(frame)
	}
	event.Button = 2
	if started, _ := runtime.triggerClick(event, 3); !started {
		t.Fatal("Shift-right-click push did not start")
	}
	for frame := int64(3); frame <= 5; frame++ {
		runtime.advance(frame)
	}
	if got, want := sent, []string{
		"/pull BobJones ",
		"/whisper excuse me Bob Jones ",
		"/push BobJones ",
		"/whisper excuse me Bob Jones ",
	}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
	if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 0 {
		t.Fatalf("runtime diagnostics = %#v", diagnostics)
	}
}

func TestLegacyMacroMovePlayerQueuesReferenceInput(t *testing.T) {
	legacyMacroInputState.Lock()
	originalMoved := legacyMacroInputState.moved
	legacyMacroInputState.moved = false
	legacyMacroInputState.Unlock()
	inputMu.Lock()
	originalLatest := latestInput
	originalQueue := append([]inputState(nil), inputQueue...)
	latestInput = inputState{mouseX: 12, mouseY: -7, mouseDown: true}
	inputQueue = nil
	inputMu.Unlock()
	originalWalkSpeed := gs.KBWalkSpeed
	gs.KBWalkSpeed = 0.25
	walkSpeed := gs.KBWalkSpeed
	t.Cleanup(func() {
		inputMu.Lock()
		latestInput = originalLatest
		inputQueue = originalQueue
		inputMu.Unlock()
		gs.KBWalkSpeed = originalWalkSpeed
		legacyMacroInputState.Lock()
		legacyMacroInputState.moved = originalMoved
		legacyMacroInputState.Unlock()
	})

	legacyMacroMovePlayer(legacyMacroMove{Direction: legacyMacroMoveSouthWest})
	legacyMacroMovePlayer(legacyMacroMove{Direction: legacyMacroMoveStop})
	if !legacyMacroMovedThisFrame() {
		t.Fatal("macro movement was not marked for the current input frame")
	}

	inputMu.Lock()
	got := append([]inputState(nil), inputQueue...)
	inputMu.Unlock()
	want := []inputState{
		{mouseX: int16(-float64(fieldCenterX) * walkSpeed), mouseY: int16(float64(fieldCenterY) * walkSpeed), mouseDown: true},
		{mouseX: 12, mouseY: -7},
	}
	if len(got) != len(want) {
		t.Fatalf("queued input = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("queued input %d = %#v, want %#v", index, got[index], want[index])
		}
	}
}

func legacyMacroWebFixtureProgram(t *testing.T, fixture string) legacyMacroProgram {
	t.Helper()
	path := filepath.Join("testdata", "legacy_macros", "web", fixture)
	text, err := legacyMacroWebCorpus.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: fixture,
		Path: path,
		Text: string(text),
	}})
	if len(program.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", program.Diagnostics)
	}
	return program
}
