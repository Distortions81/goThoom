package eui

import (
	"encoding/json"
	"fmt"
)

// SyntaxColors is the palette shared by source-code editors.
type SyntaxColors struct {
	Comments  Color
	Strings   Color
	Keywords  Color
	Numbers   Color
	Variables Color
	Bindings  Color
}

// MarshalJSON keeps saved editor colors exact, including their opacity.
func (colors SyntaxColors) MarshalJSON() ([]byte, error) {
	hex := func(c Color) string { return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A) }
	return json.Marshal(map[string]string{
		"Comments": hex(colors.Comments), "Strings": hex(colors.Strings),
		"Keywords": hex(colors.Keywords), "Numbers": hex(colors.Numbers),
		"Variables": hex(colors.Variables), "Bindings": hex(colors.Bindings),
	})
}

// DefaultSyntaxColors supplies readable defaults for palettes that omit syntax
// colors, based on the palette's input background.
func DefaultSyntaxColors(background Color) SyntaxColors {
	if 299*int(background.R)+587*int(background.G)+114*int(background.B) >= 128000 {
		return SyntaxColors{
			Comments: NewColor(45, 105, 45, 255), Strings: NewColor(145, 65, 20, 255),
			Keywords: NewColor(35, 70, 170, 255), Numbers: NewColor(115, 55, 145, 255),
			Variables: NewColor(0, 100, 115, 255), Bindings: NewColor(140, 55, 105, 255),
		}
	}
	return SyntaxColors{
		Comments: NewColor(145, 200, 145, 255), Strings: NewColor(235, 190, 130, 255),
		Keywords: NewColor(120, 190, 255, 255), Numbers: NewColor(205, 165, 240, 255),
		Variables: NewColor(120, 215, 220, 255), Bindings: NewColor(235, 160, 200, 255),
	}
}

// CurrentSyntaxColors returns a copy of the active theme's editor palette.
func CurrentSyntaxColors() SyntaxColors {
	if currentTheme != nil {
		return currentTheme.Syntax
	}
	return baseTheme.Syntax
}
