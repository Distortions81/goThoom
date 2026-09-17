package main

import "gothoom/eui"

type sourceHighlightKind uint8

const (
	sourceHighlightPlain sourceHighlightKind = iota
	sourceHighlightComment
	sourceHighlightString
	sourceHighlightKeyword
	sourceHighlightNumber
	sourceHighlightVariable
	sourceHighlightBinding
)

func sourceHighlightPalette(colors eui.SyntaxColors) [7]eui.Color {
	return [7]eui.Color{{}, colors.Comments, colors.Strings, colors.Keywords,
		colors.Numbers, colors.Variables, colors.Bindings}
}

func sourceEditorColors() eui.SyntaxColors {
	if gs.EditorUseCustomColors && gs.EditorSyntaxColors != nil {
		return *gs.EditorSyntaxColors
	}
	return eui.CurrentSyntaxColors()
}
