package main

import (
	"fmt"
	"strings"
)

// Lint is advisory: classic macros may use dynamic names and control flow.
// Do not execute code or guess whether an arbitrary variable/function exists.
func lintLegacyMacroProgram(program legacyMacroProgram) []legacyMacroDiagnostic {
	var warnings []legacyMacroDiagnostic
	report := func(line legacyMacroLine, message string) {
		warnings = append(warnings, legacyMacroDiagnostic{Location: tokenLocation(line, line.Tokens[0]), Message: message})
	}
	checkArguments := func(line legacyMacroLine) {
		if len(line.Tokens) == 0 || line.Tokens[0].Quote != 0 {
			return
		}
		name, count := strings.ToLower(line.Tokens[0].Text), len(line.Tokens)-1
		switch name {
		case "pause", "call", "goto":
			if count != 1 {
				report(line, name+" requires one argument")
			}
		case "set", "setglobal":
			if count < 2 {
				report(line, name+" requires a variable and value")
			}
		case "equip", "unequip":
			if count < 1 {
				report(line, name+" requires an item or slot")
			}
		case "if":
			if count != 3 {
				report(line, "if requires a value, comparison, and value")
			}
		case "else":
			if legacyMacroLineControl(line) == legacyMacroControlElseIf && count != 4 {
				report(line, "else if requires a value, comparison, and value")
			}
		case "random":
			if count > 1 {
				report(line, "random accepts only an optional no-repeat argument")
			}
		case "label":
			if count != 1 || line.Tokens[1].Quote != 0 {
				report(line, "label requires one unquoted name")
			}
		}
	}
	for _, line := range program.TopLevel {
		checkArguments(line)
	}
	for _, macro := range program.Macros {
		type block struct {
			kind     legacyMacroControl
			line     legacyMacroLine
			elseSeen bool
		}
		var stack []block
		labels := map[string]bool{}
		for _, line := range macro.Body {
			checkArguments(line)
			switch control := legacyMacroLineControl(line); control {
			case legacyMacroControlIf, legacyMacroControlRandom:
				stack = append(stack, block{kind: control, line: line})
			case legacyMacroControlElse, legacyMacroControlElseIf, legacyMacroControlOr:
				want, name := legacyMacroControlIf, "if"
				if control == legacyMacroControlOr {
					want, name = legacyMacroControlRandom, "random"
				}
				if len(stack) == 0 || stack[len(stack)-1].kind != want {
					report(line, line.Tokens[0].Text+" has no matching "+name)
				} else if want == legacyMacroControlIf {
					last := &stack[len(stack)-1]
					if last.elseSeen {
						report(line, "branch follows an unconditional else")
					}
					last.elseSeen = last.elseSeen || control == legacyMacroControlElse
				}
			case legacyMacroControlEndIf, legacyMacroControlEndRandom:
				want, name := legacyMacroControlIf, "if"
				if control == legacyMacroControlEndRandom {
					want, name = legacyMacroControlRandom, "random"
				}
				if len(stack) == 0 || stack[len(stack)-1].kind != want {
					report(line, "end "+name+" has no matching "+name)
				} else {
					stack = stack[:len(stack)-1]
				}
			case legacyMacroControlLabel:
				if len(line.Tokens) == 2 && line.Tokens[1].Quote == 0 {
					name := line.Tokens[1].Text
					if labels[name] {
						report(line, fmt.Sprintf("duplicate label %q; goto uses the first", name))
					}
					labels[name] = true
				}
			}
		}
		for _, block := range stack {
			name := "if"
			if block.kind == legacyMacroControlRandom {
				name = "random"
			}
			report(block.line, name+" has no matching end "+name)
		}
	}
	return warnings
}
