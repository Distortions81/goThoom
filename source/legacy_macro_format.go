package main

import (
	"fmt"
	"strings"
)

// Formatting is deliberately limited to indentation and trailing whitespace.
// Token spelling, line order, quoted text, and comment contents are preserved.
func formatLegacyMacro(value string) (string, error) {
	lines, comments, err := legacyMacroFormatLines(value)
	if err != nil {
		return "", err
	}
	program := legacyMacroProgram{}
	byLine := make(map[int]legacyMacroLine, len(lines))
	for _, line := range lines {
		byLine[line.Number] = line
		if len(line.Tokens) > 0 && !(line.Tokens[0].Quote == 0 && legacyMacroKeyword(line.Tokens[0].Text, "include")) {
			program.Lines = append(program.Lines, line)
		}
	}
	parseLegacyMacroDeclarations(&program)
	if len(program.Diagnostics) > 0 {
		d := program.Diagnostics[0]
		return "", fmt.Errorf("line %d: %s", d.Location.Line, d.Message)
	}
	raw := strings.Split(value, "\n")
	depth, offset, commentIndex := 0, 0, 0
	var controls []legacyMacroControl
	for index, original := range raw {
		line := byLine[index+1]
		first := ""
		if len(line.Tokens) > 0 && line.Tokens[0].Quote == 0 {
			first = line.Tokens[0].Text
		}
		control := legacyMacroLineControl(line)
		if first == "}" {
			depth = max(0, depth-1)
			if depth == 0 {
				controls = nil
			}
		}
		if depth > 0 && (control == legacyMacroControlEndIf || control == legacyMacroControlEndRandom) && len(controls) > 0 {
			controls = controls[:len(controls)-1]
		}
		indent := depth + len(controls)
		if depth > 0 && (control == legacyMacroControlElse || control == legacyMacroControlElseIf || control == legacyMacroControlOr) {
			indent = max(depth, indent-1)
		}
		// Leave every physical line touched by a block comment intact. This
		// includes comments inside strings and ASCII-art comment blocks.
		for commentIndex < len(comments) && comments[commentIndex].End <= offset {
			commentIndex++
		}
		protected := commentIndex < len(comments) && comments[commentIndex].Start < offset+len(original)+1
		if !protected {
			body := strings.TrimLeft(original, " \t")
			tail := 0
			if len(line.Tokens) > 0 {
				tail = line.Tokens[len(line.Tokens)-1].EndColumn - 1
			}
			if !strings.Contains(line.Text[tail:], "//") {
				body = strings.TrimRight(body, " \t")
			}
			if body == "" {
				raw[index] = ""
			} else {
				raw[index] = strings.Repeat("\t", indent) + body
			}
		}
		if depth > 0 && (control == legacyMacroControlIf || control == legacyMacroControlRandom) {
			controls = append(controls, control)
		}
		if first == "{" {
			depth++
		}
		offset += len(original) + 1
	}
	formatted := strings.Join(raw, "\n")
	// Refuse any transformation that changed a token or its physical line.
	after, _, err := legacyMacroFormatLines(formatted)
	if err != nil {
		return "", err
	}
	for _, line := range after {
		before := byLine[line.Number].Tokens
		if len(before) != len(line.Tokens) {
			return "", fmt.Errorf("line %d: formatting would change macro tokens", line.Number)
		}
		for i, token := range line.Tokens {
			if token.Text != before[i].Text || token.Quote != before[i].Quote {
				return "", fmt.Errorf("line %d: formatting would change macro tokens", line.Number)
			}
		}
	}
	return formatted, nil
}

func legacyMacroFormatLines(value string) ([]legacyMacroLine, []legacyMacroComment, error) {
	var comments []legacyMacroComment
	lines, diagnostics := legacyMacroSourceLinesWithComments(legacyMacroSource{Text: value}, &comments)
	if len(diagnostics) > 0 {
		return nil, nil, fmt.Errorf("line %d: %s", diagnostics[0].Location.Line, diagnostics[0].Message)
	}
	for index := range lines {
		tokens, diagnostic := tokenizeLegacyMacroLine(lines[index])
		if diagnostic != nil {
			return nil, nil, fmt.Errorf("line %d: %s", diagnostic.Location.Line, diagnostic.Message)
		}
		lines[index].Tokens = tokens
	}
	return lines, comments, nil
}
