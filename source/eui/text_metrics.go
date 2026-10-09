package eui

import (
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type textContentMetrics struct {
	width  float64
	height float32
}

type textMeasureCache struct {
	value string
	face  *text.GoTextFace
	textContentMetrics
}

func measureTextContent(value string, face text.Face) textContentMetrics {
	var width float64
	lines := 0
	for line := range strings.SplitSeq(value, "\n") {
		w, _ := text.Measure(line, face, 0)
		width = max(width, w)
		lines++
	}
	metrics := face.Metrics()
	lineHeight := float32(math.Ceil(metrics.HAscent + metrics.HDescent + 2))
	return textContentMetrics{width: width, height: lineHeight * float32(lines)}
}

// Changing the font source, font size, or UI scale selects another cached face,
// so direct item edits need no invalidation hook. Caller-supplied faces are
// measured directly: their features and variations can change in place.
func (item *itemData) textContentSize(face *text.GoTextFace) textContentMetrics {
	cache := item.textMeasure
	if cache != nil && cache.value == item.Text && cache.face == face {
		return cache.textContentMetrics
	}
	metrics := measureTextContent(item.Text, face)
	if cache == nil {
		cache = &textMeasureCache{}
		item.textMeasure = cache
	}
	*cache = textMeasureCache{value: item.Text, face: face, textContentMetrics: metrics}
	return metrics
}
