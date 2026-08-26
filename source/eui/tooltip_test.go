package eui

import (
	"image/color"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

func TestTooltipColorsRemainHighContrast(t *testing.T) {
	background, foreground, border := tooltipColors()
	if background != (color.RGBA{16, 18, 22, 255}) {
		t.Fatalf("background = %v", background)
	}
	if foreground != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("foreground = %v", foreground)
	}
	if border != (color.RGBA{208, 212, 220, 255}) {
		t.Fatalf("border = %v", border)
	}
}

func TestTooltipTextIsTruncatedAndWrapped(t *testing.T) {
	if err := EnsureFontSource(goregular.TTF); err != nil {
		t.Fatalf("font source: %v", err)
	}
	raw := strings.Repeat("tooltip words ", 20)
	short := truncateTooltip(raw)
	if len([]rune(short)) > tooltipMaxRunes {
		t.Fatalf("tooltip has %d runes, max %d", len([]rune(short)), tooltipMaxRunes)
	}
	if !strings.HasSuffix(short, " ...") {
		t.Fatalf("truncated tooltip = %q", short)
	}

	face := textFace(12)
	wrapped := wrapTooltip(short, face, tooltipMaxWidth)
	if !strings.Contains(wrapped, "\n") {
		t.Fatalf("tooltip was not wrapped: %q", wrapped)
	}
	for _, line := range strings.Split(wrapped, "\n") {
		if width, _ := text.Measure(line, face, 0); width > tooltipMaxWidth {
			t.Fatalf("line width %f exceeds %d: %q", width, tooltipMaxWidth, line)
		}
	}
}
