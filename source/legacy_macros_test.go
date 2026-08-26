package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestLoadLegacyMacrosForCharacter(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	dir := legacyMacrosDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Default"), []byte("\"hi\" \"hello\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Gaia"), []byte("include \"Default\"\nf1 \"/wave\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := loadLegacyMacrosForCharacter("Gaia"); err != nil {
		t.Fatal(err)
	}
	sources := legacyMacroSourcesSnapshot()
	if len(sources) != 2 {
		t.Fatalf("loaded %d sources, want 2", len(sources))
	}
	if sources[0].Name != "Gaia" || sources[1].Name != "Default" {
		t.Fatalf("source order = %#v, want Gaia then included Default", sources)
	}
}

func TestLegacyMacroSourceSupportsMacRomanAndUTF8(t *testing.T) {
	dir := t.TempDir()
	sourceText := "\"café\" \"/say café \\\\path\\r\"\n"
	macRoman := bytes.ReplaceAll([]byte(sourceText), []byte("é"), []byte{0x8e})
	tests := []struct {
		name string
		text []byte
	}{
		{name: "MacRoman", text: macRoman},
		{name: "UTF-8", text: []byte(sourceText)},
		{name: "UTF-8 BOM", text: append([]byte{0xef, 0xbb, 0xbf}, []byte(sourceText)...)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, test.name)
			if err := os.WriteFile(path, test.text, 0o644); err != nil {
				t.Fatal(err)
			}
			source, exists, err := readLegacyMacroSource(path)
			if err != nil {
				t.Fatal(err)
			}
			if !exists {
				t.Fatal("macro source was not found")
			}
			if source.Text != sourceText {
				t.Fatalf("source text = %q, want %q", source.Text, sourceText)
			}

			program := parseLegacyMacroSources([]legacyMacroSource{source})
			var sent []string
			runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
				SendText: func(text string) { sent = append(sent, text) },
			})
			if !runtime.triggerExpression("café", 0) {
				t.Fatal("accented expression macro did not start")
			}
			if got, want := sent, []string{"/say café \\path"}; !equalStrings(got, want) {
				t.Fatalf("sent = %#v, want %#v", got, want)
			}
		})
	}
}

func TestLoadLegacyMacrosReportsTopLevelRuntimeError(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })
	if err := os.MkdirAll(legacyMacrosDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyMacrosDir(), "Gaia"), []byte("set missing + 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := loadLegacyMacrosForCharacter("Gaia")
	if err == nil || !strings.Contains(err.Error(), "is not defined") {
		t.Fatalf("load error = %v, want top-level setup error", err)
	}
}

func TestLegacyMacroParserCommentsEscapesAndIncludes(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	dir := legacyMacrosDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := strings.Join([]string{
		"/* top comment */",
		"include \"nested/extra\" // trailing",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "Gaia"), []byte(root), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "extra"), []byte("// ignored\ninclude \"base\"\nf1 \"/wave\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "base"), []byte("f2 \"/bow\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := loadLegacyMacrosForCharacter("Gaia"); err != nil {
		t.Fatal(err)
	}
	program := legacyMacroProgramSnapshot()
	if len(program.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", program.Diagnostics)
	}
	if len(program.Files) != 3 || program.Files[1].Name != "nested/extra" || program.Files[2].Name != "base" {
		t.Fatalf("files = %#v", program.Files)
	}
	if len(program.Lines) != 2 || program.Lines[0].Tokens[1].Text != "/bow\r" || program.Lines[1].Tokens[1].Text != "/wave\r" {
		t.Fatalf("lines = %#v", program.Lines)
	}
}

func TestLegacyMacroNumericPrefixesMatchClassic(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
		ok    bool
	}{
		{value: "12:34:56p", want: 12, ok: true},
		{value: " -7 trailing", want: -7, ok: true},
		{value: "+3", want: 3, ok: true},
		{value: "timestamp", ok: false},
	} {
		got, ok := legacyMacroStringToInt(test.value)
		if got != test.want || ok != test.ok {
			t.Errorf("legacyMacroStringToInt(%q) = (%d, %t), want (%d, %t)", test.value, got, ok, test.want, test.ok)
		}
	}
	if matched, err := legacyMacroCompare("12:34:56p message", ">", "9"); err != nil || !matched {
		t.Fatalf("numeric-prefix comparison = (%t, %v), want (true, nil)", matched, err)
	}
}

func TestLegacyMacroTextLogClassicTimestamp(t *testing.T) {
	now := time.Date(2026, time.August, 26, 0, 5, 9, 0, time.Local)
	if got, want := legacyMacroTimestampedTextLog("healing message", now), "8/26/26 12:05:09a healing message"; got != want {
		t.Fatalf("timestamped text log = %q, want %q", got, want)
	}
}

func TestLegacyMacroSelectedItemDefaultsToNothing(t *testing.T) {
	originalID, originalIndex := selectedInvID, selectedInvIdx
	selectedInvID = 0
	selectedInvIdx = 0
	t.Cleanup(func() {
		selectedInvID = originalID
		selectedInvIdx = originalIndex
	})
	if got := legacyMacroSelectedItemName(); got != "Nothing" {
		t.Fatalf("selected item = %q, want Nothing", got)
	}
}

func TestLegacyMacroDebugAndUnfriendlyRuntimeBehavior(t *testing.T) {
	debugProgram := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "debug",
		Path: filepath.Join(t.TempDir(), "debug"),
		Text: strings.Join([]string{
			"run",
			"{",
			"set @env.debug true",
			"set value Moonstone",
			"message value",
			"}",
		}, "\n"),
	}})
	var messages []string
	debugRuntime := newLegacyMacroRuntimeWithHooks(debugProgram, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})
	execution, err := debugRuntime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	debugRuntime.advance(0)
	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("debug execution = %#v", execution)
	}
	if got, want := messages, []string{"• MACRO set @env.debug true", "• MACRO set value Moonstone", "Moonstone"}; !equalStrings(got, want) {
		t.Fatalf("debug messages = %#v, want %#v", got, want)
	}

	makeSliceProgram := func(unfriendly bool) legacyMacroProgram {
		lines := []string{"run", "{"}
		if unfriendly {
			lines = append(lines, "set @env.unfriendly true")
		}
		for range 130 {
			lines = append(lines, `"x"`)
		}
		lines = append(lines, "}")
		return parseLegacyMacroSources([]legacyMacroSource{{
			Name: "slice",
			Path: filepath.Join(t.TempDir(), "slice"),
			Text: strings.Join(lines, "\n"),
		}})
	}
	clock := func() func() time.Time {
		calls := 0
		return func() time.Time {
			result := time.Unix(0, 0).Add(time.Duration(calls) * 6 * time.Millisecond)
			calls++
			return result
		}
	}
	friendlyRuntime := newLegacyMacroRuntimeWithHooks(makeSliceProgram(false), legacyMacroRuntimeHooks{Now: clock()})
	friendly, err := friendlyRuntime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	friendlyRuntime.advance(0)
	if friendly.complete {
		t.Fatal("friendly macro ignored its time slice")
	}

	unfriendlyRuntime := newLegacyMacroRuntimeWithHooks(makeSliceProgram(true), legacyMacroRuntimeHooks{Now: clock()})
	unfriendly, err := unfriendlyRuntime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	unfriendlyRuntime.advance(0)
	if !unfriendly.complete || unfriendly.diagnostic != nil {
		t.Fatalf("unfriendly execution = %#v, want completion without a friendly time slice", unfriendly)
	}
}

func TestLegacyMacroOutputUsesLocalCommandDispatcher(t *testing.T) {
	originalCommands := scriptCommands
	originalOwners := scriptCommandOwners
	originalDisabled := scriptDisabled
	scriptCommands = map[string]scriptCommandHandler{}
	scriptCommandOwners = map[string]string{}
	scriptDisabled = map[string]bool{}
	t.Cleanup(func() {
		scriptCommands = originalCommands
		scriptCommandOwners = originalOwners
		scriptDisabled = originalDisabled
	})

	var got string
	scriptCommands["compat"] = func(args string) { got = args }
	scriptCommandOwners["compat"] = "test"
	legacyMacroQueueText("/compat hello world")
	if got != "hello world" {
		t.Fatalf("local macro command args = %q, want hello world", got)
	}
}

func TestServerMacroInterruptCancelsActiveLegacyMacros(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "interrupt",
		Path: filepath.Join(t.TempDir(), "interrupt"),
		Text: "run\n{\npause 100\n}\n",
	}})
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{})
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)

	legacyMacrosMu.Lock()
	originalRuntime := legacyMacrosRuntime
	legacyMacrosRuntime = runtime
	legacyMacrosMu.Unlock()
	originalBlockMusic := blockMusic
	blockMusic = false
	t.Cleanup(func() {
		legacyMacrosMu.Lock()
		legacyMacrosRuntime = originalRuntime
		legacyMacrosMu.Unlock()
		blockMusic = originalBlockMusic
	})

	if !parseInterruptCommand("/m_interrupt") {
		t.Fatal("server macro interrupt was not handled")
	}
	if !execution.complete || len(runtime.active) != 0 {
		t.Fatalf("execution after interrupt = %#v, active = %d", execution, len(runtime.active))
	}
}

func TestTokenizeLegacyMacroLineQuotedEscapes(t *testing.T) {
	line := legacyMacroLine{
		Source: legacyMacroSource{Path: "test.mac"},
		Number: 3,
		Text:   "\"hello // still text\" \"say\\r\\\\\\\"\\'\"",
	}
	tokens, diagnostic := tokenizeLegacyMacroLine(line)
	if diagnostic != nil {
		t.Fatal(diagnostic)
	}
	if len(tokens) != 2 {
		t.Fatalf("tokens = %#v", tokens)
	}
	if tokens[0].Text != "hello // still text" || tokens[0].Quote != '"' {
		t.Fatalf("first token = %#v", tokens[0])
	}
	if got, want := tokens[1].Text, "say\r\\\"'"; got != want {
		t.Fatalf("escape decoding = %q, want %q", got, want)
	}
}

func TestLegacyMacroParserReportsSourceLocations(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	dir := legacyMacrosDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"include \"missing\"",
		"\"unterminated",
		"/* never closes",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "Gaia"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := loadLegacyMacrosForCharacter("Gaia")
	if err == nil {
		t.Fatal("loadLegacyMacrosForCharacter succeeded despite malformed source")
	}
	program := legacyMacroProgramSnapshot()
	if len(program.Diagnostics) != 3 {
		t.Fatalf("diagnostics = %#v, want 3", program.Diagnostics)
	}
	foundMissing := false
	for _, diagnostic := range program.Diagnostics {
		if diagnostic.Location.Path == "" || diagnostic.Location.Line <= 0 || diagnostic.Location.Column <= 0 {
			t.Fatalf("diagnostic lacks source location: %#v", diagnostic)
		}
		if strings.Contains(diagnostic.Message, "does not exist") {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Fatalf("diagnostics lack missing include error: %#v", program.Diagnostics)
	}
}

func TestLoadLegacyMacrosRejectsPathCharacterName(t *testing.T) {
	if err := loadLegacyMacrosForCharacter("../outside"); err == nil {
		t.Fatal("loadLegacyMacrosForCharacter accepted a path character name")
	}
}

func TestLegacyMacroDeclarationParser(t *testing.T) {
	originalDataDir := dataDirPath
	dataDirPath = t.TempDir()
	t.Cleanup(func() { dataDirPath = originalDataDir })

	dir := legacyMacrosDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"set greeting \"hello\"",
		"\"say\" \"/say \" @text \"\\r\"",
		"'brb' \"be right back\"",
		"f1",
		"{",
		"$ignore_case",
		"\"/wave\\r\"",
		"}",
		"control-click",
		"{",
		"$any_click",
		"$no_override",
		"}",
		"wheelup \"/up\\r\"",
		"helper",
		"{",
		"message \"hello\"",
		"}",
		"control-right-click",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "Gaia"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := loadLegacyMacrosForCharacter("Gaia"); err != nil {
		t.Fatal(err)
	}
	program := legacyMacroProgramSnapshot()
	if len(program.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", program.Diagnostics)
	}
	if len(program.TopLevel) != 1 || program.TopLevel[0].Tokens[0].Text != "set" {
		t.Fatalf("top-level declarations = %#v", program.TopLevel)
	}
	if len(program.Macros) != 7 {
		t.Fatalf("macro count = %d, want 7", len(program.Macros))
	}

	expression := program.Macros[0]
	if expression.Kind != legacyMacroExpression || expression.Trigger != "say" || len(expression.Body) != 1 {
		t.Fatalf("expression = %#v", expression)
	}
	if got, want := expression.Body[0].Tokens[0].Text, "/say "; got != want {
		t.Fatalf("expression command = %q, want %q", got, want)
	}

	replacement := program.Macros[1]
	if replacement.Kind != legacyMacroReplacement || replacement.Trigger != "brb" {
		t.Fatalf("replacement = %#v", replacement)
	}

	key := program.Macros[2]
	if key.Kind != legacyMacroKey || key.Key.Name != "f1" ||
		key.Attributes&legacyMacroIgnoreCase == 0 || len(key.Body) != 1 {
		t.Fatalf("key macro = %#v", key)
	}

	click := program.Macros[3]
	if click.Kind != legacyMacroClick || click.Key.Name != "click" || click.Key.Button != 1 ||
		click.Key.Modifiers&legacyMacroModControl == 0 ||
		click.Attributes&(legacyMacroAnyClick|legacyMacroNoOverride) !=
			legacyMacroAnyClick|legacyMacroNoOverride {
		t.Fatalf("click macro = %#v", click)
	}

	wheel := program.Macros[4]
	if wheel.Kind != legacyMacroWheel || wheel.Key.Name != "wheelup" || len(wheel.Body) != 1 {
		t.Fatalf("wheel macro = %#v", wheel)
	}

	function := program.Macros[5]
	if function.Kind != legacyMacroFunction || function.Trigger != "helper" || len(function.Body) != 1 {
		t.Fatalf("function macro = %#v", function)
	}

	rightClick := program.Macros[6]
	if rightClick.Kind != legacyMacroClick || rightClick.Key.Name != "click2" ||
		rightClick.Key.Button != 2 || rightClick.Key.Modifiers&legacyMacroModControl == 0 {
		t.Fatalf("right-click macro = %#v", rightClick)
	}
}

func TestLegacyMacroDeclarationParserReportsBraceErrors(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: "f1\n{\n}\n}\n",
	}})
	if len(program.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want one unexpected closing brace", program.Diagnostics)
	}
	if got, want := program.Diagnostics[0].Message, "unexpected closing brace"; got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}

func TestLegacyMacroDeclarationNamesMayStartWithSetOrInclude(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"setBagOne message \"set function\"",
			"includeHelper message \"include function\"",
		}, "\n"),
	}})
	if len(program.TopLevel) != 0 || len(program.Macros) != 2 {
		t.Fatalf("program = %#v, want two function declarations", program)
	}
	if program.Macros[0].Trigger != "setBagOne" || program.Macros[1].Trigger != "includeHelper" {
		t.Fatalf("function names = %q, %q", program.Macros[0].Trigger, program.Macros[1].Trigger)
	}
}

func TestLegacyMacroKeyParserHandlesCompatibilityNames(t *testing.T) {
	kind, binding, ok := parseLegacyMacroKeyBinding("undo")
	if !ok || kind != legacyMacroKey || binding.Name != "undo" {
		t.Fatalf("undo binding = (%v, %#v, %t), want legacy key", kind, binding, ok)
	}
	kind, binding, ok = parseLegacyMacroKeyBinding("wnumpad-1")
	if !ok || kind != legacyMacroKey || binding.Name != "1" || binding.Modifiers != legacyMacroModNumpad {
		t.Fatalf("wnumpad binding = (%v, %#v, %t), want numpad-1", kind, binding, ok)
	}
	if _, _, ok := parseLegacyMacroKeyBinding("banana-f1"); ok {
		t.Fatal("unknown modifier was silently ignored")
	}
}

func TestLegacyMacroRuntimeCoreCommands(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"set greeting \"world\"",
			"helper",
			"{",
			"set local \"friend\"",
			"message \"hello \" greeting",
			"\"/say \" local \"\\r\"",
			"pause 2",
			"setglobal greeting \"everyone\"",
			"}",
			"run",
			"{",
			"call helper",
			"\"/wave \" greeting \"\\r\"",
			"}",
		}, "\n"),
	}})
	if len(program.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %#v", program.Diagnostics)
	}

	var sent, messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
		Message:  func(message string) { messages = append(messages, message) },
	})
	if got := runtime.globalsSnapshot()["greeting"]; got != "world" {
		t.Fatalf("top-level set greeting = %q, want world", got)
	}
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}

	runtime.advance(10)
	if got, want := sent, []string{"/say friend"}; !equalStrings(got, want) {
		t.Fatalf("first send = %#v, want %#v", got, want)
	}
	if got, want := messages, []string{"hello  world"}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
	if execution.complete {
		t.Fatal("execution completed before its pause")
	}

	runtime.advance(10)
	if len(sent) != 1 {
		t.Fatalf("pause sent commands early: %#v", sent)
	}

	runtime.advance(11)
	runtime.advance(12)
	if len(sent) != 1 {
		t.Fatalf("pause sent commands early: %#v", sent)
	}

	runtime.advance(13)
	if got, want := sent, []string{"/say friend", "/wave everyone"}; !equalStrings(got, want) {
		t.Fatalf("second send = %#v, want %#v", got, want)
	}
	runtime.advance(14)
	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want completed without error", execution)
	}
	if got := runtime.globalsSnapshot()["greeting"]; got != "everyone" {
		t.Fatalf("setglobal greeting = %q, want everyone", got)
	}
}

func TestLegacyMacroRuntimeInterrupts(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"keyWait",
			"{",
			"set @env.key_interrupts true",
			"pause 10",
			"message \"key\"",
			"}",
			"clickWait",
			"{",
			"set @env.click_interrupts true",
			"pause 10",
			"message \"click\"",
			"}",
			"plainWait",
			"{",
			"pause 10",
			"message \"plain\"",
			"}",
		}, "\n"),
	}})
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{})
	for _, name := range []string{"keyWait", "clickWait", "plainWait"} {
		if _, err := runtime.startFunction(name); err != nil {
			t.Fatal(err)
		}
	}
	runtime.advance(0)

	if got := runtime.interruptIfEnabled("@env.key_interrupts"); got != 1 {
		t.Fatalf("key interrupts = %d, want 1", got)
	}
	if got := len(runtime.active); got != 2 {
		t.Fatalf("active macros after key interrupt = %d, want 2", got)
	}
	if started, _ := runtime.triggerClick(legacyMacroClickEvent{HasButton: true}, 0); started {
		t.Fatal("unexpected click macro")
	}
	if got := len(runtime.active); got != 1 {
		t.Fatalf("active macros after world click = %d, want 1", got)
	}

	if _, err := runtime.startFunction("clickWait"); err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)
	if started, _ := runtime.triggerClick(legacyMacroClickEvent{}, 0); started {
		t.Fatal("unexpected player-list click macro")
	}
	if got := len(runtime.active); got != 2 {
		t.Fatalf("player-list click interrupted macros: %d active, want 2", got)
	}
	if got := runtime.cancelAll(); got != 2 {
		t.Fatalf("Ctrl-Escape cancellation count = %d, want 2", got)
	}
	if len(runtime.active) != 0 {
		t.Fatalf("active macros after Ctrl-Escape = %#v", runtime.active)
	}
}

func TestLegacyMacroMouseChord(t *testing.T) {
	if got := legacyMacroMouseChordFromPressed(true, false, false); got != 1 {
		t.Fatalf("left-click chord = %d, want 1", got)
	}
	if got := legacyMacroMouseChordFromPressed(true, true, true); got != 7 {
		t.Fatalf("three-button chord = %d, want 7", got)
	}
	if got := legacyMacroMouseChordFromPressed(true, false, false, true, true); got != 25 {
		t.Fatalf("five-button chord = %d, want 25", got)
	}
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: "click2\n{\nmessage @click.chord\n}\n",
	}})
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})
	event := legacyMacroClickEvent{Button: 2, Chord: 3, HasButton: true, HasChord: true, OnPlayer: true}
	if started, _ := runtime.triggerClick(event, 0); !started {
		t.Fatal("first right click did not trigger")
	}
	if started, _ := runtime.triggerClick(event, 1); !started {
		t.Fatal("second right click did not trigger")
	}
	if got, want := messages, []string{"3", "3"}; !equalStrings(got, want) {
		t.Fatalf("click chord messages = %#v, want %#v", got, want)
	}
}

func TestLegacyMacroPlayersWindowPlainClickIsReserved(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"click message \"plain\"",
			"command-click message \"command\"",
		}, "\n"),
	}})
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})
	playerEvent := legacyMacroClickEvent{Name: "Gaia", HasName: true, OnPlayer: true, Button: 1}
	if started, allowDefault := runtime.triggerClick(playerEvent, 0); started || !allowDefault {
		t.Fatalf("plain Players click = (%t, %t), want (false, true)", started, allowDefault)
	}
	playerEvent.Modifiers = legacyMacroModCommand
	if started, allowDefault := runtime.triggerClick(playerEvent, 0); !started || allowDefault {
		t.Fatalf("Command-click = (%t, %t), want (true, false)", started, allowDefault)
	}
	if got, want := messages, []string{"command"}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
}

func TestLegacyMacroPrintedKeyName(t *testing.T) {
	if got, want := legacyMacroPrintedKeyName(ebiten.KeyDigit3, "3", legacyMacroModShift, []ebiten.Key{ebiten.KeyDigit3}, nil), "#"; got != want {
		t.Fatalf("Shift-3 name = %q, want %q", got, want)
	}
	if got, want := legacyMacroPrintedKeyName(ebiten.KeyM, "m", legacyMacroModOption, []ebiten.Key{ebiten.KeyM}, []rune{'µ'}), "µ"; got != want {
		t.Fatalf("Option-M name = %q, want %q", got, want)
	}
	if got, want := legacyMacroPrintedKeyName(ebiten.KeyA, "a", 0, []ebiten.Key{ebiten.KeyA}, []rune{'a'}), "a"; got != want {
		t.Fatalf("A name = %q, want %q", got, want)
	}
	if name, numpad, ok := legacyMacroKeyName(ebiten.KeyInsert); !ok || numpad || name != "help" {
		t.Fatalf("Insert = (%q, %t, %t), want (help, false, true)", name, numpad, ok)
	}
}

func TestLegacyMacroPrintedKeySuppressesPhysicalHotkey(t *testing.T) {
	legacyMacroInputState.Lock()
	originalKeys := legacyMacroInputState.consumedKeys
	originalConsumed := legacyMacroInputState.consumed
	legacyMacroInputState.consumedKeys = make(map[ebiten.Key]bool)
	legacyMacroInputState.consumed = make(map[string]bool)
	legacyMacroInputState.Unlock()
	t.Cleanup(func() {
		legacyMacroInputState.Lock()
		legacyMacroInputState.consumedKeys = originalKeys
		legacyMacroInputState.consumed = originalConsumed
		legacyMacroInputState.Unlock()
	})

	legacyMacroMarkKeyConsumed(ebiten.KeyM, "µ")
	if !legacyMacroHotkeySuppressed("option-µ") || !legacyMacroHotkeySuppressed("option-m") {
		t.Fatal("printed and physical key names were not both suppressed")
	}
}

func TestLegacyMacroShiftWheelActsHorizontally(t *testing.T) {
	name, modifiers := legacyMacroWheelInput(0, 1, legacyMacroModShift|legacyMacroModControl)
	if name != "wheelleft" || modifiers != legacyMacroModControl {
		t.Fatalf("Shift-wheel up = (%q, %v), want (wheelleft, control)", name, modifiers)
	}
	name, modifiers = legacyMacroWheelInput(0, -1, legacyMacroModShift)
	if name != "wheelright" || modifiers != 0 {
		t.Fatalf("Shift-wheel down = (%q, %v), want (wheelright, none)", name, modifiers)
	}
	name, modifiers = legacyMacroWheelInput(1, 0, legacyMacroModShift)
	if name != "wheelright" || modifiers != legacyMacroModShift {
		t.Fatalf("horizontal wheel = (%q, %v), want (wheelright, shift)", name, modifiers)
	}
}

func TestLegacyMacroPlayerModifierClick(t *testing.T) {
	playersMu.Lock()
	originalPlayers := players
	players = make(map[string]*Player)
	playersMu.Unlock()
	originalSelected := selectedPlayerName
	originalDirty := playersDirty
	originalPersistDirty := playersPersistDirty
	inputMu.Lock()
	originalInput := append([]rune(nil), inputText...)
	originalInputPos := inputPos
	originalInputActive := inputActive
	inputMu.Unlock()
	t.Cleanup(func() {
		playersMu.Lock()
		players = originalPlayers
		playersMu.Unlock()
		selectedPlayerName = originalSelected
		playersDirty = originalDirty
		playersPersistDirty = originalPersistDirty
		inputMu.Lock()
		inputText = originalInput
		inputPos = originalInputPos
		inputActive = originalInputActive
		inputMu.Unlock()
	})

	const name = "Anne-Marie"
	for want := 1; want <= 5; want++ {
		if !legacyMacroHandlePlayerModifierClick(name, legacyMacroModControl) {
			t.Fatal("Control-click was not handled")
		}
		if got := getPlayer(name).GlobalLabel; got != want {
			t.Fatalf("Control-click label = %d, want %d", got, want)
		}
	}
	legacyMacroHandlePlayerModifierClick(name, legacyMacroModControl)
	if got := getPlayer(name).GlobalLabel; got != 0 {
		t.Fatalf("label cycle wrapped to %d, want 0", got)
	}

	legacyMacroHandlePlayerModifierClick(name, legacyMacroModControl|legacyMacroModShift)
	if got := getPlayer(name).GlobalLabel; got != 6 {
		t.Fatalf("block cycle = %d, want 6", got)
	}
	legacyMacroHandlePlayerModifierClick(name, legacyMacroModControl|legacyMacroModShift)
	if got := getPlayer(name).GlobalLabel; got != 7 {
		t.Fatalf("ignore cycle = %d, want 7", got)
	}
	legacyMacroHandlePlayerModifierClick(name, legacyMacroModControl|legacyMacroModShift)
	if got := getPlayer(name).GlobalLabel; got != 0 {
		t.Fatalf("ignore cycle wrapped to %d, want 0", got)
	}

	legacyMacroHandlePlayerModifierClick(name, legacyMacroModOption)
	inputMu.Lock()
	gotInput := string(inputText)
	inputMu.Unlock()
	if gotInput != " AnneMarie " {
		t.Fatalf("Option-click input = %q, want sanitized player name", gotInput)
	}
	legacyMacroHandlePlayerModifierClick(name, legacyMacroModCommand)
	if selectedPlayerName != name {
		t.Fatalf("Command-click selected %q, want %q", selectedPlayerName, name)
	}
}

func TestLegacyMacroRuntimeSetOperationsAndErrors(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"run",
			"{",
			"set count \"2\"",
			"set count + 3",
			"set word \"hi\"",
			"set word + \"!\"",
			"setglobal total count",
			"call missing",
			"}",
		}, "\n"),
	}})
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{})
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)

	if !execution.complete || execution.diagnostic == nil {
		t.Fatalf("execution = %#v, want missing-function error", execution)
	}
	if got := runtime.globalsSnapshot()["total"]; got != "5" {
		t.Fatalf("setglobal total = %q, want 5", got)
	}
	if diagnostics := runtime.diagnosticsSnapshot(); len(diagnostics) != 1 ||
		!strings.Contains(diagnostics[0].Message, "not defined") {
		t.Fatalf("runtime diagnostics = %#v", diagnostics)
	}
}

func TestLegacyMacroVariableNamesAreCaseInsensitive(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"set MixedCase \"ready\"",
			"run",
			"{",
			"setglobal RESULT mixedcase",
			"message result",
			"}",
		}, "\n"),
	}})
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)
	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want successful completion", execution)
	}
	if got, want := messages, []string{"ready"}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
	if got := runtime.globalsSnapshot()["result"]; got != "ready" {
		t.Fatalf("result = %q, want ready", got)
	}
}

func TestLegacyMacroRuntimeIfElseAndGoto(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"run",
			"{",
			"set value \"2\"",
			"if value > 3",
			"message \"too high\"",
			"else if value == 2",
			"message \"matched\"",
			"if value == 3",
			"message \"wrong nested branch\"",
			"else",
			"message \"nested else\"",
			"end if",
			"else",
			"message \"wrong outer branch\"",
			"end if",
			"label again",
			"set value + 1",
			"if value < 4",
			"goto again",
			"end if",
			"message value",
			"}",
		}, "\n"),
	}})
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)

	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want successful completion", execution)
	}
	if got, want := messages, []string{"matched", "nested else", "4"}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
}

func TestLegacyMacroStringComparisonsUseSubstringDirection(t *testing.T) {
	tests := []struct {
		left, comparison, right string
		want                    bool
	}{
		{left: "please kill the rat", comparison: "<", right: "kill", want: true},
		{left: "kill", comparison: ">", right: "please kill the rat", want: true},
		{left: "please kill the rat", comparison: ">=", right: "kill", want: true},
		{left: "kill", comparison: "<=", right: "please kill the rat", want: true},
		{left: "kill", comparison: "<", right: "kill", want: false},
		{left: "kill", comparison: "<=", right: "kill", want: true},
		{left: "kill", comparison: ">", right: "kill", want: false},
		{left: "kill", comparison: ">=", right: "kill", want: true},
		{left: "kill", comparison: ">=", right: "please kill the rat", want: false},
		{left: "please kill the rat", comparison: "<=", right: "kill", want: false},
		{left: "Kill", comparison: "==", right: "kill", want: true},
		{left: "Kill the rat", comparison: ">=", right: "kill", want: false},
	}
	for _, test := range tests {
		got, err := legacyMacroCompare(test.left, test.comparison, test.right)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Errorf("%q %s %q = %t, want %t", test.left, test.comparison, test.right, got, test.want)
		}
	}
}

func TestLegacyMacroObsoleteMessageAndClickAliases(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"click2",
			"{",
			"msg \"clicked \" @clickplayer",
			"}",
		}, "\n"),
	}})
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
	})
	event := legacyMacroClickEvent{Name: "Anne-Marie", HasName: true, OnPlayer: true, Button: 2, HasButton: true}
	if started, _ := runtime.triggerClick(event, 0); !started {
		t.Fatal("right click did not trigger")
	}
	if got, want := messages, []string{"clicked  AnneMarie"}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
}

func TestLegacyMacroRuntimeRandomNoRepeat(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"run",
			"{",
			"random no-repeat",
			"message \"one\"",
			"or",
			"message \"two\"",
			"or",
			"message \"three\"",
			"end random",
			"}",
		}, "\n"),
	}})
	rolls := []int{1, 1, 2}
	var messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		Message: func(message string) { messages = append(messages, message) },
		RandomInt: func(limit int) int {
			value := rolls[0]
			rolls = rolls[1:]
			return value
		},
	})

	for i := 0; i < 2; i++ {
		execution, err := runtime.startFunction("run")
		if err != nil {
			t.Fatal(err)
		}
		runtime.advance(int64(i))
		if !execution.complete || execution.diagnostic != nil {
			t.Fatalf("execution %d = %#v, want successful completion", i, execution)
		}
	}
	if got, want := messages, []string{"two", "three"}; !equalStrings(got, want) {
		t.Fatalf("random messages = %#v, want %#v", got, want)
	}
}

func TestLegacyMacroRuntimeContinuousModeTimeSlicesBusyGoto(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"run",
			"{",
			"label again",
			"goto again",
			"}",
		}, "\n"),
	}})
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{})
	runtime.allowContinuous = true
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	for frame := 0; frame < 200; frame++ {
		runtime.advance(int64(frame))
	}
	if execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want time-sliced busy loop to remain active", execution)
	}
	runtime.cancelAll()
}

func TestLegacyMacroRuntimeStopsBusyGotoAtInstructionLimit(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"run",
			"{",
			"label again",
			"goto again",
			"}",
		}, "\n"),
	}})
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{})
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	for frame := 0; frame < 200 && !execution.complete; frame++ {
		runtime.advance(int64(frame))
	}
	if !execution.complete || execution.diagnostic == nil {
		t.Fatalf("execution = %#v, want instruction-limit error", execution)
	}
	if !strings.Contains(execution.diagnostic.Message, "Allow continuous macros") {
		t.Fatalf("diagnostic = %q, want option guidance", execution.diagnostic.Message)
	}
}

func TestNewLegacyMacroRuntimeUsesContinuousMacroSetting(t *testing.T) {
	original := gs.LegacyMacroContinuous
	t.Cleanup(func() { gs.LegacyMacroContinuous = original })

	for _, enabled := range []bool{false, true} {
		gs.LegacyMacroContinuous = enabled
		runtime := newLegacyMacroRuntime(legacyMacroProgram{})
		if runtime.allowContinuous != enabled {
			t.Fatalf("allowContinuous = %t, want %t", runtime.allowContinuous, enabled)
		}
	}
}

func TestLegacyMacroRuntimeAllowsLongRunningYieldingLoop(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"run",
			"{",
			"label again",
			"pause 1",
			"goto again",
			"}",
		}, "\n"),
	}})
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{})
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	for frame := int64(0); frame < 200; frame++ {
		runtime.advance(frame)
	}
	if execution.complete || execution.diagnostic != nil {
		t.Fatalf("yielding execution = %#v, want it to remain active", execution)
	}
	runtime.cancelAll()
}

func TestLegacyMacroRuntimeBusyScannerObservesLaterEquipment(t *testing.T) {
	lines := []string{
		"set htest 0",
		"run",
		"{",
		"label top",
	}
	for range 80 {
		lines = append(lines,
			"if @env.textlog >= \"message that is not present\"",
			"message \"wrong branch\"",
			"end if",
		)
	}
	lines = append(lines,
		"if htest == 0",
		"if @my.right_item == \"Moonstone\"",
		"\"/useitem moonstone\\r\"",
		"end if",
		"end if",
		"goto top",
		"}",
	)
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join(lines, "\n"),
	}})
	rightItem := "Nothing"
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
		ResolveState: func(name string) (string, bool) {
			switch name {
			case "@env.textlog":
				return "unrelated text", true
			case "@my.right_item":
				return rightItem, true
			default:
				return "", false
			}
		},
	})
	runtime.allowContinuous = true
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	for frame := int64(0); frame < 200; frame++ {
		runtime.advance(frame)
	}
	if execution.complete || execution.diagnostic != nil {
		t.Fatalf("scanner execution = %#v, want it to remain active", execution)
	}

	rightItem = "Moonstone"
	for frame := int64(200); frame < 220 && len(sent) == 0; frame++ {
		runtime.advance(frame)
	}
	if got, want := sent, []string{"/useitem moonstone"}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
	runtime.cancelAll()
}

func TestLegacyMacroRuntimeInclusiveSubstringTradeMatch(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"run",
			"{",
			"set logck @env.textlog",
			"if logck >= \"To accept, \\BUY 0\"",
			"\"/buy 0 Trader\\r\"",
			"end if",
			"}",
		}, "\n"),
	}})
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
		ResolveState: func(name string) (string, bool) {
			if name == "@env.textlog" {
				return "To accept, \\BUY 0 from Trader", true
			}
			return "", false
		},
	})
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)
	if got, want := sent, []string{"/buy 0 Trader"}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
	if execution.diagnostic != nil {
		t.Fatalf("execution diagnostic = %#v", execution.diagnostic)
	}
}

func TestLegacyMacroRuntimeDynamicIndexesAndTextExpansion(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"set table[0] \"zero\"",
			"set table[1] \"one\"",
			"set answer \"ready\"",
			"set pointer \"answer\"",
			"set phrase \"status: \" answer",
			"helper[1]",
			"{",
			"message \"helper \" table[index]",
			"}",
			"run",
			"{",
			"set index 1",
			"message \"indirect: \" pointer",
			"message phrase",
			"message \"picked: \" table[index] missing",
			"message @text.word_count \"/\" @text.word[index].num_letters",
			"\"/say \" @text.word[index].letter[index] missing \"\\r\"",
			"setglobal table[index] \"changed\"",
			"call helper[index]",
			"}",
		}, "\n"),
	}})
	if len(program.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics = %#v", program.Diagnostics)
	}
	var sent, messages []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
		Message:  func(message string) { messages = append(messages, message) },
	})
	execution, err := runtime.startFunctionWithContext("run", legacyMacroExecutionContext{Text: "red blue"})
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)
	if got, want := sent, []string{"/say lmissing"}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
	if got, want := messages, []string{"indirect:  ready", "status: ready", "picked:  one missing", "2 / 4"}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
	runtime.advance(1)
	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want successful completion", execution)
	}
	if got, want := messages, []string{"indirect:  ready", "status: ready", "picked:  one missing", "2 / 4", "helper  changed"}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
	if got := runtime.globalsSnapshot()["table[1]"]; got != "changed" {
		t.Fatalf("table[1] = %q, want changed", got)
	}
}

func TestLegacyMacroRuntimeReturnPacingAndEquipmentCommands(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"run",
			"{",
			"\"/one\\r/two\\r\"",
			"equip \"Green Token\"",
			"unequip left",
			"}",
		}, "\n"),
	}})
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
	})
	execution, err := runtime.startFunction("run")
	if err != nil {
		t.Fatal(err)
	}

	runtime.advance(0)
	runtime.advance(0)
	if got, want := sent, []string{"/one"}; !equalStrings(got, want) {
		t.Fatalf("same-frame sends = %#v, want %#v", got, want)
	}
	runtime.advance(1)
	runtime.advance(2)
	runtime.advance(3)
	if got, want := sent, []string{"/one", "/two", "/equip Green Token", "/unequip left"}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
	runtime.advance(4)
	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want successful completion", execution)
	}
}

func TestLegacyMacroRuntimeHistoryIsBounded(t *testing.T) {
	runtime := newLegacyMacroRuntimeWithHooks(legacyMacroProgram{}, legacyMacroRuntimeHooks{})
	for index := 0; index < legacyMacroMaxHistory+20; index++ {
		runtime.messageLocked(strconv.Itoa(index))
		runtime.sendTextLocked(nil, strconv.Itoa(index))
		runtime.recordDiagnosticLocked(legacyMacroLine{}, strconv.Itoa(index))
	}
	if got := len(runtime.messagesSnapshot()); got != legacyMacroMaxHistory {
		t.Fatalf("message history = %d, want %d", got, legacyMacroMaxHistory)
	}
	if got := len(runtime.sentSnapshot()); got != legacyMacroMaxHistory {
		t.Fatalf("sent history = %d, want %d", got, legacyMacroMaxHistory)
	}
	if got := len(runtime.diagnosticsSnapshot()); got != legacyMacroMaxHistory {
		t.Fatalf("diagnostic history = %d, want %d", got, legacyMacroMaxHistory)
	}
}

func TestLegacyMacroTextTrailersUseRunes(t *testing.T) {
	if got, want := legacyMacroApplyTextTrailers("café", ".letter[3]"), "é"; got != want {
		t.Fatalf("letter = %q, want %q", got, want)
	}
	if got, want := legacyMacroApplyTextTrailers("café", ".num_letters"), "4"; got != want {
		t.Fatalf("num_letters = %q, want %q", got, want)
	}
}

func TestLegacyMacroRuntimeStateVariablesAndAliases(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"set @echo \"echo\"",
			"set @debug \"debug\"",
			"set @interruptclick \"click\"",
			"set @interruptkey \"key\"",
			"run",
			"{",
			"@text \"|\" @textsel \"|\" @text.word[1] \"|\" @text.num_words \"|\" @text.letter[1] \"|\" @text.num_letters \"|\" @word[0] \"|\" @my.name \"|\" @my.simple_name \"|\" @selplayer.name \"|\" @selplayer.simple_name \"|\" @my.right_item \"|\" @my.left_item \"|\" @env.textlog \"|\" @click.name \"|\" @click.simple_name \"|\" @click.button \"|\" @click.chord \"|\" @name \"|\" @splayer \"|\" @rplayer \"|\" @rhanditem \"|\" @lhanditem \"|\" @echo \"|\" @debug \"|\" @interruptclick \"|\" @interruptkey \"|\" @clicksplayer \"|\" @clickrplayer \"|\" @wordcount \"\\r\"",
			"}",
		}, "\n"),
	}})
	state := map[string]string{
		"@my.name":               "Gaia O'Neill",
		"@my.simple_name":        "GaiaONeill",
		"@my.right_item":         "sun sword",
		"@my.left_item":          "moon shield",
		"@selplayer.name":        "Anne-Marie",
		"@selplayer.simple_name": "AnneMarie",
		"@env.textlog":           "latest log line",
	}
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
		ResolveState: func(name string) (string, bool) {
			value, ok := state[strings.ToLower(name)]
			return value, ok
		},
	})
	execution, err := runtime.startFunctionWithContext("run", legacyMacroExecutionContext{
		Text:           "hello there",
		TextSelection:  "there",
		ClickName:      "Boo-Boo",
		ClickButton:    2,
		ClickChord:     3,
		HasClickName:   true,
		HasClickButton: true,
		HasClickChord:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime.advance(0)
	runtime.advance(1)
	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want successful completion", execution)
	}

	want := "hello there|there|there|2|e|11|hello|Gaia O'Neill|GaiaONeill|Anne-Marie|AnneMarie|sun sword|moon shield|latest log line|Boo-Boo|BooBoo|2|3|Gaia O'Neill|AnneMarie|Anne-Marie|sun sword|moon shield|echo|debug|click|key|BooBoo|Boo-Boo|2"
	if got := sent; !equalStrings(got, []string{want}) {
		t.Fatalf("sent = %#v, want %#v", got, []string{want})
	}
	globals := runtime.globalsSnapshot()
	for name, want := range map[string]string{
		"@env.echo":             "echo",
		"@env.debug":            "debug",
		"@env.click_interrupts": "click",
		"@env.key_interrupts":   "key",
	} {
		if got := globals[name]; got != want {
			t.Fatalf("global %s = %q, want %q", name, got, want)
		}
	}
}

func TestLegacyMacroCurrentGameVariables(t *testing.T) {
	originalPlayerName := playerName
	originalSelectedPlayerName := selectedPlayerName
	playersMu.Lock()
	originalPlayers := players
	players = map[string]*Player{
		"Gaia O'Neill": {Name: "Gaia O'Neill"},
		"Anne-Marie":   {Name: "Anne-Marie", Sharing: true},
		"Bob Jones":    {Name: "Bob Jones", Sharee: true},
		"Zed":          {Name: "Zed", Sharee: true},
	}
	playersMu.Unlock()
	originalTextLog := legacyMacroTextLogValue()
	playerName = "Gaia O'Neill"
	selectedPlayerName = "Anne-Marie"
	legacyMacroSetTextLog("last text line")
	t.Cleanup(func() {
		playerName = originalPlayerName
		selectedPlayerName = originalSelectedPlayerName
		playersMu.Lock()
		players = originalPlayers
		playersMu.Unlock()
		legacyMacroSetTextLog(originalTextLog)
	})

	for name, want := range map[string]string{
		"@my.name":               "Gaia O'Neill",
		"@my.simple_name":        "GaiaONeill",
		"@selplayer.name":        "Anne-Marie",
		"@selplayer.simple_name": "AnneMarie",
		"@my.shares_in":          "AnneMarie",
		"@my.shares_out":         "BobJones Zed",
		"@env.textlog":           "last text line",
	} {
		got, ok := legacyMacroCurrentGameVariable(name)
		if !ok || got != want {
			t.Errorf("%s = %q, %t; want %q, true", name, got, ok, want)
		}
	}
}

func TestLegacyMacroTriggerDispatch(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"\"say\" \"/say \" @text \"\\r\"",
			"f1 \"/wave\\r\"",
			"click",
			"{",
			"\"/tell \" @click.simple_name \"\\r\"",
			"}",
			"click2",
			"{",
			"$any_click",
			"$no_override",
			"\"/look \" @click.name \"\\r\"",
			"}",
			"wheelup \"/up\\r\"",
		}, "\n"),
	}})
	var sent []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText: func(text string) { sent = append(sent, text) },
	})

	if !runtime.triggerExpression("say hello", 0) {
		t.Fatal("expression macro did not start")
	}
	if started, allowDefault := runtime.triggerKey("f1", 0, 0); !started || allowDefault {
		t.Fatalf("f1 trigger = (%t, %t), want (true, false)", started, allowDefault)
	}
	if started, allowDefault := runtime.triggerClick(legacyMacroClickEvent{
		Name:      "Bob O'Reilly",
		HasName:   true,
		OnPlayer:  true,
		Button:    1,
		Chord:     1,
		HasButton: true,
		HasChord:  true,
	}, 0); !started || allowDefault {
		t.Fatalf("player click trigger = (%t, %t), want (true, false)", started, allowDefault)
	}
	if started, allowDefault := runtime.triggerClick(legacyMacroClickEvent{
		HasName:   true,
		Button:    2,
		Chord:     1,
		HasButton: true,
		HasChord:  true,
	}, 0); !started || !allowDefault {
		t.Fatalf("any-click trigger = (%t, %t), want (true, true)", started, allowDefault)
	}
	if started, allowDefault := runtime.triggerWheel("wheelup", 0, 0); !started || allowDefault {
		t.Fatalf("wheel trigger = (%t, %t), want (true, false)", started, allowDefault)
	}

	if got, want := sent, []string{"/say hello", "/wave", "/tell BobOReilly", "/look ", "/up"}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
}

func TestLegacyMacroReplacementExpansion(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"'brb' \"be right back\"",
			"'can\\'t' \"cannot\"",
			"'drop'",
			"{",
			"}",
			"'show' \"[\" @text \"|\" @textsel \"]\"",
			"'wait'",
			"{",
			"\"soon\"",
			"pause 1",
			"}",
			"'bad' \"prefix\" \"\\r\"",
		}, "\n"),
	}})
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{})

	updated, cursor, handled := runtime.triggerReplacement("say brb", len([]rune("say brb")))
	if !handled || updated != "say be right back" || cursor != len([]rune(updated)) {
		t.Fatalf("brb replacement = (%q, %d, %t)", updated, cursor, handled)
	}
	updated, cursor, handled = runtime.triggerReplacement("say (brb", len([]rune("say (brb")))
	if handled || updated != "say (brb" || cursor != len([]rune(updated)) {
		t.Fatalf("punctuation boundary replacement = (%q, %d, %t)", updated, cursor, handled)
	}
	updated, cursor, handled = runtime.triggerReplacement("say can't", len([]rune("say can't")))
	if !handled || updated != "say cannot" || cursor != len([]rune(updated)) {
		t.Fatalf("punctuation trigger replacement = (%q, %d, %t)", updated, cursor, handled)
	}
	updated, cursor, handled = runtime.triggerReplacement("say drop", len([]rune("say drop")))
	if !handled || updated != "say" || cursor != len([]rune("say")) {
		t.Fatalf("empty replacement = (%q, %d, %t)", updated, cursor, handled)
	}
	updated, cursor, handled = runtime.triggerReplacement("say show later", len([]rune("say show")))
	if !handled || updated != "say [say show later|] later" || cursor != len([]rune("say [say show later|]")) {
		t.Fatalf("context replacement = (%q, %d, %t)", updated, cursor, handled)
	}
	updated, cursor, handled = runtime.triggerReplacementWithSelection("say show later", len([]rune("say show")), "show")
	if !handled || updated != "say [say show later|show] later" || cursor != len([]rune("say [say show later|show]")) {
		t.Fatalf("selected context replacement = (%q, %d, %t)", updated, cursor, handled)
	}
	updated, _, handled = runtime.triggerReplacement("wait", len([]rune("wait")))
	if !handled || updated != "soon" {
		t.Fatalf("paused replacement = (%q, %t)", updated, handled)
	}
	updated, _, handled = runtime.triggerReplacement("bad", len([]rune("bad")))
	if !handled || updated != "prefix" {
		t.Fatalf("return replacement = (%q, %t)", updated, handled)
	}

	diagnostics := runtime.diagnosticsSnapshot()
	if len(diagnostics) != 2 || !strings.Contains(diagnostics[0].Message, "may not pause") ||
		!strings.Contains(diagnostics[1].Message, "may not contain a return") {
		t.Fatalf("replacement diagnostics = %#v", diagnostics)
	}
}

func TestLegacyMacroRuntimeTriggerOutputTargets(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"set @env.echo \"true\"",
			"\"edit\" \"edited\"",
			"f1 \"typed\"",
			"f2 \"/wave\\r\"",
		}, "\n"),
	}})
	var sent, inserted, setText []string
	runtime := newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText:   func(text string) { sent = append(sent, text) },
		InsertText: func(text string) { inserted = append(inserted, text) },
		SetText:    func(text string) { setText = append(setText, text) },
	})

	if started, allowDefault := runtime.triggerKey("f1", 0, 0); !started || allowDefault {
		t.Fatalf("f1 trigger = (%t, %t), want (true, false)", started, allowDefault)
	}
	if !runtime.triggerExpression("edit", 0) {
		t.Fatal("expression output macro did not start")
	}
	if started, allowDefault := runtime.triggerKey("f2", 0, 0); !started || allowDefault {
		t.Fatalf("f2 trigger = (%t, %t), want (true, false)", started, allowDefault)
	}

	if got, want := inserted, []string{"typed"}; !equalStrings(got, want) {
		t.Fatalf("inserted = %#v, want %#v", got, want)
	}
	if got, want := setText, []string{"edited", "/wave"}; !equalStrings(got, want) {
		t.Fatalf("set text = %#v, want %#v", got, want)
	}
	if got, want := sent, []string{"/wave"}; !equalStrings(got, want) {
		t.Fatalf("sent = %#v, want %#v", got, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
