package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err := os.WriteFile(filepath.Join(dir, "nested", "extra"), []byte("// ignored\nf1 \"/wave\\r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := loadLegacyMacrosForCharacter("Gaia"); err != nil {
		t.Fatal(err)
	}
	program := legacyMacroProgramSnapshot()
	if len(program.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", program.Diagnostics)
	}
	if len(program.Files) != 2 || program.Files[1].Name != "nested/extra" {
		t.Fatalf("files = %#v", program.Files)
	}
	if len(program.Lines) != 1 || program.Lines[0].Tokens[1].Text != "/wave\r" {
		t.Fatalf("lines = %#v", program.Lines)
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

func TestLegacyMacroRuntimeCoreCommands(t *testing.T) {
	program := parseLegacyMacroSources([]legacyMacroSource{{
		Name: "test",
		Path: filepath.Join(t.TempDir(), "test"),
		Text: strings.Join([]string{
			"set greeting \"world\"",
			"helper",
			"{",
			"set local \"friend\"",
			"message \"hello\" greeting",
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
	if got, want := messages, []string{"hello world"}; !equalStrings(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
	if execution.complete {
		t.Fatal("execution completed before its pause")
	}

	runtime.advance(10)
	runtime.advance(11)
	if len(sent) != 1 {
		t.Fatalf("pause sent commands early: %#v", sent)
	}

	runtime.advance(12)
	if got, want := sent, []string{"/say friend", "/wave everyone"}; !equalStrings(got, want) {
		t.Fatalf("second send = %#v, want %#v", got, want)
	}
	runtime.advance(12)
	if !execution.complete || execution.diagnostic != nil {
		t.Fatalf("execution = %#v, want completed without error", execution)
	}
	if got := runtime.globalsSnapshot()["greeting"]; got != "everyone" {
		t.Fatalf("setglobal greeting = %q, want everyone", got)
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

func TestLegacyMacroRuntimeStopsRunawayGoto(t *testing.T) {
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

	if !execution.complete || execution.diagnostic == nil ||
		!strings.Contains(execution.diagnostic.Message, "execution limit") {
		t.Fatalf("execution = %#v, want execution-limit error", execution)
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
	runtime.advance(0)
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
