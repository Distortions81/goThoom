package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

type legacyTriggerField struct {
	declaration legacyMacroDeclaration
	quote       byte
	start, end  int
}

type legacyTriggerDocument struct {
	entry     legacyMacroLibraryEntry
	program   legacyMacroProgram
	originals map[string][]byte
	fields    []legacyTriggerField
}

func loadLegacyTriggerDocument(entry legacyMacroLibraryEntry, keys bool) (*legacyTriggerDocument, error) {
	source, exists, err := readLegacyMacroSource(entry.Path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("macro file no longer exists")
	}
	doc := &legacyTriggerDocument{entry: entry, program: parseLegacyMacroSources([]legacyMacroSource{source}), originals: map[string][]byte{}}
	for _, file := range doc.program.Files {
		raw, err := os.ReadFile(file.Path)
		if err != nil {
			return nil, err
		}
		if decodeLegacyMacroSourceText(raw) != file.Text {
			return nil, fmt.Errorf("%s changed while loading; reopen the editor", file.Path)
		}
		doc.originals[file.Path] = raw
	}
	for _, decl := range doc.program.Macros {
		key := decl.Kind == legacyMacroKey || decl.Kind == legacyMacroClick || decl.Kind == legacyMacroWheel
		command := decl.Kind == legacyMacroExpression || decl.Kind == legacyMacroReplacement
		if (keys && !key) || (!keys && !command) {
			continue
		}
		for _, line := range doc.program.Lines {
			if line.Source.Path != decl.Header.Path || line.Number != decl.Header.Line || len(line.Tokens) == 0 {
				continue
			}
			token := line.Tokens[0]
			if token.Column != decl.Header.Column || token.EndColumn <= token.Column {
				continue
			}
			start, end := line.Offsets[token.Column-1], line.Offsets[token.EndColumn-2]+1
			// A comment inside the trigger cannot be replaced without removing it.
			if end-start != token.EndColumn-token.Column {
				return nil, fmt.Errorf("%s:%d: edit the comment inside this trigger in the source editor", decl.Header.Path, decl.Header.Line)
			}
			doc.fields = append(doc.fields, legacyTriggerField{decl, token.Quote, start, end})
			break
		}
	}
	return doc, nil
}

func formatLegacyTrigger(field legacyTriggerField, value string) (string, error) {
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n\x00") {
		return "", fmt.Errorf("enter a non-empty trigger on one line")
	}
	if strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return "", fmt.Errorf("a trigger cannot contain block-comment markers")
	}
	token := value
	if field.quote != 0 {
		token = strings.ReplaceAll(token, "\\", "\\\\")
		token = strings.ReplaceAll(token, string(field.quote), "\\"+string(field.quote))
		token = string(field.quote) + token + string(field.quote)
	} else {
		if strings.ContainsAny(value, " \t") {
			return "", fmt.Errorf("a key binding cannot contain spaces; use names such as Control-F6 or Shift-Click2")
		}
		// Legacy escaping keeps punctuation keys distinct from quote delimiters.
		token = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "'", "\\'").Replace(token)
	}
	line := legacyMacroLine{Text: token}
	tokens, diagnostic := tokenizeLegacyMacroLine(line)
	if diagnostic != nil || len(tokens) != 1 {
		return "", fmt.Errorf("invalid trigger")
	}
	line.Tokens = tokens
	parsed := newLegacyMacroDeclaration(line)
	if parsed.Trigger != value {
		return "", fmt.Errorf("this trigger cannot be represented in legacy macro syntax")
	}
	if field.quote == 0 && parsed.Kind != legacyMacroKey && parsed.Kind != legacyMacroClick && parsed.Kind != legacyMacroWheel {
		return "", fmt.Errorf("unrecognized key or mouse binding")
	}
	return token, nil
}

func legacyTriggersConflict(a, b legacyMacroDeclaration) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case legacyMacroExpression, legacyMacroReplacement:
		if (a.Attributes|b.Attributes)&legacyMacroIgnoreCase != 0 {
			return strings.EqualFold(a.Trigger, b.Trigger)
		}
		return a.Trigger == b.Trigger
	case legacyMacroKey:
		return legacyMacroKeyMatches(a, b.Key.Name, b.Key.Modifiers) || legacyMacroKeyMatches(b, a.Key.Name, a.Key.Modifiers)
	case legacyMacroClick, legacyMacroWheel:
		return a.Key == b.Key
	}
	return false
}

func (doc *legacyTriggerDocument) save(values []string) error {
	if isWASM {
		return fmt.Errorf("embedded macros are read-only")
	}
	if len(values) != len(doc.fields) {
		return fmt.Errorf("trigger list changed; reopen the editor")
	}
	changed := map[legacyMacroLocation]bool{}
	type edit struct {
		start, end int
		text       string
	}
	edits := map[string][]edit{}
	for i, field := range doc.fields {
		if values[i] == field.declaration.Trigger {
			continue
		}
		token, err := formatLegacyTrigger(field, values[i])
		if err != nil {
			return fmt.Errorf("%s:%d: %w", filepath.Base(field.declaration.Header.Path), field.declaration.Header.Line, err)
		}
		path := field.declaration.Header.Path
		edits[path] = append(edits[path], edit{field.start, field.end, token})
		changed[field.declaration.Header] = true
	}
	if len(edits) == 0 {
		return nil
	}
	for path, original := range doc.originals {
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, original) {
			return fmt.Errorf("%s changed outside this editor; reopen it before saving", filepath.Base(path))
		}
	}
	texts := map[string]string{}
	output := map[string][]byte{}
	modes := map[string]os.FileMode{}
	for path, changes := range edits {
		original := doc.originals[path]
		text := decodeLegacyMacroSourceText(original)
		sort.Slice(changes, func(i, j int) bool { return changes[i].start > changes[j].start })
		for _, change := range changes {
			text = text[:change.start] + change.text + text[change.end:]
		}
		texts[path] = text
		data := []byte(text)
		if !utf8.Valid(original) {
			var err error
			data, err = charmap.Macintosh.NewEncoder().Bytes(data)
			if err != nil {
				return fmt.Errorf("%s uses MacRoman; this trigger needs characters outside that encoding", filepath.Base(path))
			}
		} else if bytes.HasPrefix(original, []byte{0xef, 0xbb, 0xbf}) {
			data = append([]byte{0xef, 0xbb, 0xbf}, data...)
		}
		output[path] = data
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		modes[path] = info.Mode().Perm()
	}
	reader := func(path string) (legacyMacroSource, bool, error) {
		if value, ok := texts[path]; ok {
			return legacyMacroSource{Path: path, Text: value}, true, nil
		}
		return readLegacyMacroSource(path)
	}
	root, _, err := reader(doc.entry.Path)
	if err != nil {
		return err
	}
	candidate := parseLegacyMacroSourcesWithReader([]legacyMacroSource{root}, reader)
	// Existing corpus diagnostics remain visible; edits may not introduce new ones.
	known := map[string]int{}
	diagnosticKey := func(d legacyMacroDiagnostic) string {
		return fmt.Sprintf("%s:%d:%s", d.Location.Path, d.Location.Line, d.Message)
	}
	for _, d := range doc.program.Diagnostics {
		known[diagnosticKey(d)]++
	}
	for _, d := range candidate.Diagnostics {
		key := diagnosticKey(d)
		if known[key] == 0 {
			return fmt.Errorf("validation: %s", d.Error())
		}
		known[key]--
	}
	if len(candidate.Macros) != len(doc.program.Macros) {
		return fmt.Errorf("validation changed the macro structure")
	}
	for i, a := range candidate.Macros {
		// Header columns after earlier same-line changes are not stable. Macro order is.
		if !changed[doc.program.Macros[i].Header] {
			continue
		}
		for j, b := range candidate.Macros {
			if i != j && legacyTriggersConflict(a, b) {
				return fmt.Errorf("%q conflicts with %s:%d", a.Trigger, filepath.Base(b.Header.Path), b.Header.Line)
			}
		}
	}
	paths := make([]string, 0, len(output))
	for path := range output {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	written := []string{}
	for _, path := range paths {
		if err := legacyMacroAtomicWriteFile(path, output[path], modes[path]); err != nil {
			for _, prior := range written {
				if restoreErr := legacyMacroAtomicWriteFile(prior, doc.originals[prior], modes[prior]); restoreErr != nil {
					return fmt.Errorf("save failed: %v; could not restore %s: %v", err, prior, restoreErr)
				}
			}
			return fmt.Errorf("save %s: %w", filepath.Base(path), err)
		}
		written = append(written, path)
	}
	return nil
}

// Keep semantic attachment separate from source placement. Editing a trigger
// never moves comments, including comments between a declaration and its body.
func attachLegacyMacroComments(program *legacyMacroProgram) {
	type anchor struct {
		offset   int
		location legacyMacroLocation
	}
	for _, source := range program.Files {
		lines, _ := legacyMacroSourceLines(source)
		var anchors []anchor
		for _, line := range lines {
			tokens, diagnostic := tokenizeLegacyMacroLine(line)
			if diagnostic != nil || len(tokens) == 0 {
				continue
			}
			token := tokens[0]
			anchors = append(anchors, anchor{line.Offsets[token.Column-1], tokenLocation(line, token)})
		}
		for index := range program.Comments {
			comment := &program.Comments[index]
			if comment.Location.Path != source.Path {
				continue
			}
			for _, code := range anchors {
				if code.location.Line == comment.Location.Line && code.offset < comment.Start || code.offset >= comment.End {
					comment.AttachedTo = code.location
					break
				}
			}
		}
	}
}
