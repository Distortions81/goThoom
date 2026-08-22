package main

import (
	"embed"
	"path/filepath"
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
		{name: "gorvin-dynamicsharecads.mac"},
		{name: "gorvin-right-clicker.mac"},
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

func TestLegacyMacroMovePlayerQueuesReferenceInput(t *testing.T) {
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
	})

	legacyMacroMovePlayer(legacyMacroMove{Direction: legacyMacroMoveSouthWest})
	legacyMacroMovePlayer(legacyMacroMove{Direction: legacyMacroMoveStop})

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
