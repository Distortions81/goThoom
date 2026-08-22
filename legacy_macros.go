package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

const legacyMacrosDirName = "Macros"

// legacyMacroSource is one legacy macro file. The reference client starts
// with the character file; that file normally includes Default explicitly.
type legacyMacroSource struct {
	Name string
	Path string
	Text string
}

type legacyMacroLocation struct {
	Path   string
	Line   int
	Column int
}

type legacyMacroDiagnostic struct {
	Location legacyMacroLocation
	Message  string
}

func (d legacyMacroDiagnostic) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", d.Location.Path, d.Location.Line, d.Location.Column, d.Message)
}

// legacyMacroToken preserves whether a token was quoted. Text has legacy
// escapes decoded: \r, \\, \" and \' are special; all other escapes remain
// unchanged.
type legacyMacroToken struct {
	Text   string
	Quote  byte
	Column int
}

// legacyMacroLine is a physical source line after block comments are removed.
type legacyMacroLine struct {
	Source legacyMacroSource
	Number int
	Text   string
	Tokens []legacyMacroToken
}

type legacyMacroProgram struct {
	Files       []legacyMacroSource
	Lines       []legacyMacroLine
	TopLevel    []legacyMacroLine
	Macros      []legacyMacroDeclaration
	Diagnostics []legacyMacroDiagnostic
}

// legacyMacroKind identifies the trigger that starts a legacy macro.
type legacyMacroKind uint8

const (
	legacyMacroExpression legacyMacroKind = iota + 1
	legacyMacroReplacement
	legacyMacroFunction
	legacyMacroKey
	legacyMacroClick
	legacyMacroWheel
)

// legacyMacroAttributes are flags placed as the first command in a macro.
type legacyMacroAttributes uint8

const (
	legacyMacroIgnoreCase legacyMacroAttributes = 1 << iota
	legacyMacroAnyClick
	legacyMacroNoOverride
)

// legacyMacroModifiers use the names accepted by the original client. They
// intentionally do not map to platform modifier bits yet; input integration
// will do that when key and click execution is added.
type legacyMacroModifiers uint8

const (
	legacyMacroModCommand legacyMacroModifiers = 1 << iota
	legacyMacroModControl
	legacyMacroModNumpad
	legacyMacroModOption
	legacyMacroModShift
)

type legacyMacroKeyBinding struct {
	Name      string
	Button    int
	Modifiers legacyMacroModifiers
}

type legacyMacroDeclaration struct {
	Kind       legacyMacroKind
	Trigger    string
	Key        legacyMacroKeyBinding
	Attributes legacyMacroAttributes
	Header     legacyMacroLocation
	Body       []legacyMacroLine
}

func (p legacyMacroProgram) err() error {
	if len(p.Diagnostics) == 0 {
		return nil
	}
	return fmt.Errorf("legacy macro parse failed: %s", p.Diagnostics[0])
}

var (
	legacyMacrosMu        sync.RWMutex
	legacyMacrosCharacter string
	legacyMacroSources    []legacyMacroSource
	legacyMacrosProgram   legacyMacroProgram
	legacyMacrosRuntime   *legacyMacroRuntime
)

func legacyMacrosDir() string {
	return filepath.Join(dataDirPath, legacyMacrosDirName)
}

// loadLegacyMacrosForCharacter follows the reference client's entry point:
// load the current character's file, which may include Default and other files.
// A missing root file is normal. The complete parsed source set is atomically
// replaced even when diagnostics exist, so a future Reload Macros action can
// show the current errors without executing stale macros.
func loadLegacyMacrosForCharacter(character string) error {
	character = strings.TrimSpace(character)
	if character != "" && filepath.Base(character) != character {
		return fmt.Errorf("invalid legacy macro character name %q", character)
	}

	var roots []legacyMacroSource
	if character != "" {
		path := filepath.Join(legacyMacrosDir(), character)
		source, exists, err := readLegacyMacroSource(path)
		if err != nil {
			return err
		}
		if exists {
			source.Name = character
			roots = append(roots, source)
		}
	}

	libraryRoots, libraryDiagnostics := legacyMacroLibrarySelectedSources(character)
	roots = append(roots, libraryRoots...)
	program := parseLegacyMacroSources(roots)
	program.Diagnostics = append(program.Diagnostics, libraryDiagnostics...)
	runtime := newLegacyMacroRuntime(program)
	runtime.startFunctionIfDefined("@login")
	legacyMacrosMu.Lock()
	legacyMacrosCharacter = character
	legacyMacroSources = append([]legacyMacroSource(nil), program.Files...)
	legacyMacrosProgram = program
	legacyMacrosRuntime = runtime
	legacyMacrosMu.Unlock()
	return program.err()
}

func readLegacyMacroSource(path string) (legacyMacroSource, bool, error) {
	text, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return legacyMacroSource{}, false, nil
	}
	if err != nil {
		return legacyMacroSource{}, false, fmt.Errorf("read legacy macro file %q: %w", path, err)
	}
	return legacyMacroSource{Path: path, Text: string(text)}, true, nil
}

func legacyMacroSourcesSnapshot() []legacyMacroSource {
	legacyMacrosMu.RLock()
	defer legacyMacrosMu.RUnlock()
	return append([]legacyMacroSource(nil), legacyMacroSources...)
}

func legacyMacroProgramSnapshot() legacyMacroProgram {
	legacyMacrosMu.RLock()
	defer legacyMacrosMu.RUnlock()

	p := legacyMacrosProgram
	p.Files = append([]legacyMacroSource(nil), p.Files...)
	p.Diagnostics = append([]legacyMacroDiagnostic(nil), p.Diagnostics...)
	p.Lines = append([]legacyMacroLine(nil), p.Lines...)
	p.TopLevel = append([]legacyMacroLine(nil), p.TopLevel...)
	p.Macros = append([]legacyMacroDeclaration(nil), p.Macros...)
	for i := range p.Lines {
		p.Lines[i].Tokens = append([]legacyMacroToken(nil), p.Lines[i].Tokens...)
	}
	for i := range p.TopLevel {
		p.TopLevel[i].Tokens = append([]legacyMacroToken(nil), p.TopLevel[i].Tokens...)
	}
	for i := range p.Macros {
		p.Macros[i].Body = append([]legacyMacroLine(nil), p.Macros[i].Body...)
		for j := range p.Macros[i].Body {
			p.Macros[i].Body[j].Tokens = append([]legacyMacroToken(nil), p.Macros[i].Body[j].Tokens...)
		}
	}
	return p
}

// parseLegacyMacroDeclarations mirrors the reference parser's simple
// line-oriented brace handling. A macro may have an inline command, a braced
// body, or both. Top-level set declarations initialize global variables and
// are kept apart for the evaluator to apply later.
func parseLegacyMacroDeclarations(program *legacyMacroProgram) {
	program.TopLevel = nil
	program.Macros = nil

	depth := 0
	lastMacro := -1
	var openingBrace legacyMacroLocation

	for _, line := range program.Lines {
		if len(line.Tokens) == 0 {
			continue
		}
		first := line.Tokens[0]

		if first.Quote == 0 && first.Text == "{" {
			if depth == 0 {
				openingBrace = tokenLocation(line, first)
			}
			depth++
			continue
		}
		if first.Quote == 0 && first.Text == "}" {
			if depth == 0 {
				program.Diagnostics = append(program.Diagnostics, legacyMacroDiagnostic{
					Location: tokenLocation(line, first),
					Message:  "unexpected closing brace",
				})
				continue
			}
			depth--
			continue
		}

		if depth == 0 {
			// The reference parser starts every top-level non-control line with
			// no current macro. A new declaration below becomes the only macro
			// a following braced block can extend.
			lastMacro = -1
			if first.Quote == 0 && legacyMacroKeywordPrefix(first.Text, "set") {
				program.TopLevel = append(program.TopLevel, line)
				continue
			}

			declaration := newLegacyMacroDeclaration(line)
			program.Macros = append(program.Macros, declaration)
			lastMacro = len(program.Macros) - 1
			if len(line.Tokens) > 1 {
				addLegacyMacroBodyLine(&program.Macros[lastMacro], legacyMacroLineTail(line, 1))
			}
			continue
		}

		if lastMacro >= 0 {
			addLegacyMacroBodyLine(&program.Macros[lastMacro], line)
		}
	}

	if depth != 0 {
		program.Diagnostics = append(program.Diagnostics, legacyMacroDiagnostic{
			Location: openingBrace,
			Message:  "macro file is missing a closing brace",
		})
	}
}

func legacyMacroKeywordPrefix(text, keyword string) bool {
	return len(text) >= len(keyword) && strings.EqualFold(text[:len(keyword)], keyword)
}

func newLegacyMacroDeclaration(line legacyMacroLine) legacyMacroDeclaration {
	first := line.Tokens[0]
	declaration := legacyMacroDeclaration{
		Trigger: first.Text,
		Header:  tokenLocation(line, first),
	}
	switch first.Quote {
	case '"':
		declaration.Kind = legacyMacroExpression
	case '\'':
		declaration.Kind = legacyMacroReplacement
	default:
		if kind, binding, ok := parseLegacyMacroKeyBinding(first.Text); ok {
			declaration.Kind = kind
			declaration.Key = binding
		} else {
			declaration.Kind = legacyMacroFunction
		}
	}
	return declaration
}

func legacyMacroLineTail(line legacyMacroLine, first int) legacyMacroLine {
	line.Tokens = append([]legacyMacroToken(nil), line.Tokens[first:]...)
	return line
}

func addLegacyMacroBodyLine(declaration *legacyMacroDeclaration, line legacyMacroLine) {
	if len(line.Tokens) == 0 {
		return
	}
	if attribute := legacyMacroAttribute(line.Tokens[0]); attribute != 0 {
		declaration.Attributes |= attribute
		return
	}
	declaration.Body = append(declaration.Body, line)
}

func legacyMacroAttribute(token legacyMacroToken) legacyMacroAttributes {
	if token.Quote != 0 {
		return 0
	}
	switch {
	case strings.EqualFold(token.Text, "$ignore_case"):
		return legacyMacroIgnoreCase
	case strings.EqualFold(token.Text, "$any_click"):
		return legacyMacroAnyClick
	case strings.EqualFold(token.Text, "$no_override"):
		return legacyMacroNoOverride
	default:
		return 0
	}
}

func parseLegacyMacroKeyBinding(trigger string) (legacyMacroKind, legacyMacroKeyBinding, bool) {
	parts := strings.Split(trigger, "-")
	keyName := parts[len(parts)-1]
	modifierParts := parts[:len(parts)-1]
	if len(parts) >= 2 &&
		strings.EqualFold(parts[len(parts)-2], "right") &&
		strings.EqualFold(parts[len(parts)-1], "click") {
		keyName = "right-click"
		modifierParts = parts[:len(parts)-2]
	}

	var modifiers legacyMacroModifiers
	for _, part := range modifierParts {
		if modifier, ok := legacyMacroModifier(part); ok {
			modifiers |= modifier
		}
	}

	name := strings.ToLower(keyName)
	binding := legacyMacroKeyBinding{Modifiers: modifiers}
	switch name {
	case "click":
		binding.Name = "click"
		binding.Button = 1
		return legacyMacroClick, binding, true
	case "click2", "right-click":
		binding.Name = "click2"
		binding.Button = 2
		return legacyMacroClick, binding, true
	case "click3":
		binding.Name = "click3"
		binding.Button = 3
		return legacyMacroClick, binding, true
	case "click4":
		binding.Name = "click4"
		binding.Button = 4
		return legacyMacroClick, binding, true
	case "click5":
		binding.Name = "click5"
		binding.Button = 5
		return legacyMacroClick, binding, true
	case "click6":
		binding.Name = "click6"
		binding.Button = 6
		return legacyMacroClick, binding, true
	case "click7":
		binding.Name = "click7"
		binding.Button = 7
		return legacyMacroClick, binding, true
	case "click8":
		binding.Name = "click8"
		binding.Button = 8
		return legacyMacroClick, binding, true
	case "wheelup", "wheeldown", "wheelleft", "wheelright":
		binding.Name = name
		return legacyMacroWheel, binding, true
	}

	if legacyMacroNamedKey(name) {
		binding.Name = name
		return legacyMacroKey, binding, true
	}
	if utf8.RuneCountInString(keyName) == 1 {
		// The original client preserves the character's case for ordinary
		// single-character key triggers.
		binding.Name = keyName
		return legacyMacroKey, binding, true
	}
	return 0, legacyMacroKeyBinding{}, false
}

func legacyMacroModifier(name string) (legacyMacroModifiers, bool) {
	switch {
	case strings.EqualFold(name, "command"):
		return legacyMacroModCommand, true
	case strings.EqualFold(name, "control"):
		return legacyMacroModControl, true
	case strings.EqualFold(name, "numpad"):
		return legacyMacroModNumpad, true
	case strings.EqualFold(name, "option"):
		return legacyMacroModOption, true
	case strings.EqualFold(name, "shift"):
		return legacyMacroModShift, true
	default:
		return 0, false
	}
}

func legacyMacroNamedKey(name string) bool {
	switch name {
	case "escape",
		"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8",
		"f9", "f10", "f11", "f12", "f13", "f14", "f15", "f16",
		"minus", "delete", "tab", "return", "space", "help", "home",
		"pageup", "del", "end", "pagedown", "up", "down", "left",
		"right", "clear", "enter":
		return true
	default:
		return false
	}
}

func parseLegacyMacroSources(roots []legacyMacroSource) legacyMacroProgram {
	program := legacyMacroProgram{}
	loaded := make(map[string]bool)

	var load func(legacyMacroSource)
	load = func(source legacyMacroSource) {
		path, err := filepath.Abs(source.Path)
		if err != nil {
			program.Diagnostics = append(program.Diagnostics, legacyMacroDiagnostic{
				Location: legacyMacroLocation{Path: source.Path, Line: 1, Column: 1},
				Message:  "resolve macro file path: " + err.Error(),
			})
			return
		}
		if loaded[path] {
			return
		}
		loaded[path] = true
		source.Path = path
		if source.Name == "" {
			source.Name = filepath.Base(path)
		}
		program.Files = append(program.Files, source)

		lines, diagnostics := legacyMacroSourceLines(source)
		program.Diagnostics = append(program.Diagnostics, diagnostics...)
		for _, line := range lines {
			tokens, diagnostic := tokenizeLegacyMacroLine(line)
			if diagnostic != nil {
				program.Diagnostics = append(program.Diagnostics, *diagnostic)
				continue
			}
			line.Tokens = tokens
			if len(tokens) == 0 {
				continue
			}

			if tokens[0].Quote == 0 && legacyMacroKeywordPrefix(tokens[0].Text, "include") {
				if len(tokens) < 2 || tokens[1].Text == "" {
					program.Diagnostics = append(program.Diagnostics, legacyMacroDiagnostic{
						Location: tokenLocation(line, tokens[0]),
						Message:  "include requires a file name",
					})
					continue
				}
				includePath, err := legacyMacroIncludePath(line.Source.Path, tokens[1].Text)
				if err != nil {
					program.Diagnostics = append(program.Diagnostics, legacyMacroDiagnostic{
						Location: tokenLocation(line, tokens[1]),
						Message:  err.Error(),
					})
					continue
				}
				included, exists, err := readLegacyMacroSource(includePath)
				if err != nil {
					program.Diagnostics = append(program.Diagnostics, legacyMacroDiagnostic{
						Location: tokenLocation(line, tokens[1]),
						Message:  err.Error(),
					})
					continue
				}
				if !exists {
					program.Diagnostics = append(program.Diagnostics, legacyMacroDiagnostic{
						Location: tokenLocation(line, tokens[1]),
						Message:  fmt.Sprintf("included macro file %q does not exist", tokens[1].Text),
					})
					continue
				}
				macroDir, _ := filepath.Abs(legacyMacrosDir())
				rel, _ := filepath.Rel(macroDir, includePath)
				included.Name = filepath.ToSlash(rel)
				load(included)
				continue
			}

			program.Lines = append(program.Lines, line)
		}
	}

	for _, root := range roots {
		load(root)
	}
	parseLegacyMacroDeclarations(&program)
	return program
}

func legacyMacroIncludePath(includingPath, includeName string) (string, error) {
	if filepath.IsAbs(includeName) {
		return "", fmt.Errorf("included macro file %q must be relative", includeName)
	}
	base, err := filepath.Abs(legacyMacrosDir())
	if err != nil {
		return "", fmt.Errorf("resolve macro directory: %w", err)
	}
	candidate, err := filepath.Abs(filepath.Join(filepath.Dir(includingPath), includeName))
	if err != nil {
		return "", fmt.Errorf("resolve included macro file %q: %w", includeName, err)
	}
	rel, err := filepath.Rel(base, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("included macro file %q escapes the Macros directory", includeName)
	}
	return candidate, nil
}

// legacyMacroSourceLines removes block comments using the reference client's
// deliberately simple behavior: /* ... */ is a comment even inside quotes.
func legacyMacroSourceLines(source legacyMacroSource) ([]legacyMacroLine, []legacyMacroDiagnostic) {
	var (
		lines        []legacyMacroLine
		diagnostics  []legacyMacroDiagnostic
		text         strings.Builder
		line, column = 1, 1
		blockComment bool
		commentStart legacyMacroLocation
	)

	flush := func() {
		lines = append(lines, legacyMacroLine{Source: source, Number: line, Text: text.String()})
		text.Reset()
	}

	for i := 0; i < len(source.Text); {
		ch := source.Text[i]
		if blockComment {
			if ch == '*' && i+1 < len(source.Text) && source.Text[i+1] == '/' {
				blockComment = false
				i += 2
				column += 2
				continue
			}
			if ch == '\r' || ch == '\n' {
				flush()
				if ch == '\r' && i+1 < len(source.Text) && source.Text[i+1] == '\n' {
					i++
				}
				i++
				line++
				column = 1
				continue
			}
			i++
			column++
			continue
		}

		if ch == '/' && i+1 < len(source.Text) && source.Text[i+1] == '*' {
			blockComment = true
			commentStart = legacyMacroLocation{Path: source.Path, Line: line, Column: column}
			i += 2
			column += 2
			continue
		}
		if ch == 0 {
			diagnostics = append(diagnostics, legacyMacroDiagnostic{
				Location: legacyMacroLocation{Path: source.Path, Line: line, Column: column},
				Message:  "macro file contains a NUL character",
			})
			i++
			column++
			continue
		}
		if ch == '\r' || ch == '\n' {
			flush()
			if ch == '\r' && i+1 < len(source.Text) && source.Text[i+1] == '\n' {
				i++
			}
			i++
			line++
			column = 1
			continue
		}
		text.WriteByte(ch)
		i++
		column++
	}
	if blockComment {
		diagnostics = append(diagnostics, legacyMacroDiagnostic{
			Location: commentStart,
			Message:  "unterminated block comment",
		})
	}
	if text.Len() > 0 {
		flush()
	}
	return lines, diagnostics
}

func tokenizeLegacyMacroLine(line legacyMacroLine) ([]legacyMacroToken, *legacyMacroDiagnostic) {
	var tokens []legacyMacroToken
	for i := 0; i < len(line.Text); {
		for i < len(line.Text) && (line.Text[i] == ' ' || line.Text[i] == '\t') {
			i++
		}
		if i >= len(line.Text) || strings.HasPrefix(line.Text[i:], "//") {
			break
		}

		start := i
		quote := byte(0)
		if line.Text[i] == '"' || line.Text[i] == '\'' {
			quote = line.Text[i]
			i++
		}
		var value strings.Builder
		closed := quote == 0
		for i < len(line.Text) {
			ch := line.Text[i]
			if quote != 0 && ch == quote {
				i++
				closed = true
				break
			}
			if quote == 0 && (ch == ' ' || ch == '\t' || strings.HasPrefix(line.Text[i:], "//")) {
				break
			}
			if ch == '\\' {
				if i+1 >= len(line.Text) {
					value.WriteByte(ch)
					i++
					continue
				}
				next := line.Text[i+1]
				switch next {
				case 'r':
					value.WriteByte('\r')
					i += 2
				case '\\', '"', '\'':
					value.WriteByte(next)
					i += 2
				default:
					// Match the reference parser: preserve an unknown slash and
					// process the following byte normally on the next iteration.
					value.WriteByte('\\')
					i++
				}
				continue
			}
			value.WriteByte(ch)
			i++
		}
		if !closed {
			return nil, &legacyMacroDiagnostic{
				Location: legacyMacroLocation{Path: line.Source.Path, Line: line.Number, Column: start + 1},
				Message:  fmt.Sprintf("matching %q not found", quote),
			}
		}
		tokens = append(tokens, legacyMacroToken{Text: value.String(), Quote: quote, Column: start + 1})
	}
	return tokens, nil
}

func tokenLocation(line legacyMacroLine, token legacyMacroToken) legacyMacroLocation {
	return legacyMacroLocation{Path: line.Source.Path, Line: line.Number, Column: token.Column}
}
