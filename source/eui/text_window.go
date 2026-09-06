package eui

import (
	"math"
	"strings"

	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

// TextWindowOptions supplies content styling and application behavior for a text
// window. Set FontSize in logical UI units. Fonts default to FontSource().
// FirstChanged may skip unchanged rows; pass zero when styling or callbacks
// change. Width and font changes always invalidate that optimization.
type TextWindowOptions struct {
	FontSize      float64
	FontSource    *text.GoTextFaceSource
	AlternateRows bool
	FirstChanged  int
	OnURLClick    func(string)
	InputText     string
	InputEditable bool
	// InputUnderlines receives wrapped input and returns rune-indexed spans.
	// A nil callback leaves the input without annotations.
	InputUnderlines func(string) []TextSpan
}

type textWindowWrapConfig struct {
	width    float64
	faceSize float64
	source   *text.GoTextFaceSource
}

type textWindowWrapEntry struct {
	text       string
	lineCount  int
	generation uint64
}

// TextWindowWrapCache keeps wrapping work proportional to new messages.
// The zero value is ready to use; keep one cache per window.
// Existing messages are rewrapped only when the effective width or font
// changes.
type TextWindowWrapCache struct {
	config     textWindowWrapConfig
	entries    map[string]textWindowWrapEntry
	generation uint64
}

func (cache *TextWindowWrapCache) begin(config textWindowWrapConfig) bool {
	changed := cache.entries == nil || cache.config != config
	if cache.entries == nil {
		cache.entries = make(map[string]textWindowWrapEntry)
	} else if cache.config != config {
		clear(cache.entries)
	}
	cache.config = config
	cache.generation++
	if cache.generation == 0 {
		// Practically unreachable, but keep the generation marker valid after
		// integer wraparound.
		clear(cache.entries)
		cache.generation = 1
	}
	return changed
}

func (cache *TextWindowWrapCache) wrap(raw string, face text.Face, width float64) (string, int) {
	if cached, ok := cache.entries[raw]; ok {
		cached.generation = cache.generation
		cache.entries[raw] = cached
		return cached.text, cached.lineCount
	}

	_, lines := WrapText(raw, face, width)
	wrapped := strings.Join(lines, "\n")
	lineCount := len(lines)
	if lineCount < 1 {
		lineCount = 1
	}
	cache.entries[raw] = textWindowWrapEntry{
		text:       wrapped,
		lineCount:  lineCount,
		generation: cache.generation,
	}
	return wrapped, lineCount
}

func (cache *TextWindowWrapCache) finish(messageCount int) {
	if messageCount == 0 {
		clear(cache.entries)
		return
	}
	// Allow a small amount of slack so appending one message does not require
	// scanning the cache. Once stale entries accumulate, discard everything
	// not used by the current message set.
	if len(cache.entries) <= messageCount+64 {
		return
	}
	for raw, cached := range cache.entries {
		if cached.generation != cache.generation {
			delete(cache.entries, raw)
		}
	}
}

// NewTextWindow creates a standardized text window with optional input bar.
func NewTextWindow(title string, hz HZone, vz VZone, withInput bool) (*WindowData, *ItemData, *ItemData) {
	win := NewWindow()
	win.Size = Point{X: 410, Y: 450}
	win.Title = title
	win.Closable = true
	win.Resizable = true
	win.Movable = true
	win.SetZone(hz, vz)
	// Only the inner list should scroll; disable window scrollbars to avoid overlap
	win.NoScroll = true

	flow := &ItemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Fixed: true}
	win.AddItem(flow)

	list := &ItemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Scrollable: true, Fixed: true}
	flow.AddItem(list)

	var input *ItemData
	if withInput {
		input = &ItemData{ItemType: ITEM_FLOW, FlowType: FLOW_VERTICAL, Fixed: true, Scrollable: true}
		flow.AddItem(input)
	}

	win.AddWindow(false)
	return win, list, input
}

// SizeTextWindowList fits a scrolling list below any fixed sibling rows, such
// as the docked toolbar. Those rows occupy part of the window's client area and
// must not be included in the list's viewport height.
func SizeTextWindowList(list *ItemData, clientW, clientH float32) {
	if list == nil {
		return
	}
	scale := UIScale()
	if scale <= 0 {
		scale = 1
	}
	clientW /= scale
	clientH /= scale
	extraH := float32(0)
	if list.Parent != nil {
		list.Parent.Size.X = clientW
		list.Parent.Size.Y = clientH
		for _, item := range list.Parent.Contents {
			if item == list {
				continue
			}
			item.Size.X = clientW
			extraH += item.Size.Y
		}
	}
	list.Size.X = clientW
	list.Size.Y = max(0, clientH-extraH)
}

// UpdateTextWindow updates only rows at or after firstChanged when the
// width and font configuration is stable. Earlier rows retain their measured
// layout and rendered content.
func UpdateTextWindow(win *WindowData, list, input *ItemData, msgs []string, options TextWindowOptions, wrapCache *TextWindowWrapCache) {
	fontSize, inputMsg := options.FontSize, options.InputText
	faceSrc, alternateRows, firstChanged := options.FontSource, options.AlternateRows, options.FirstChanged
	if wrapCache == nil {
		wrapCache = &TextWindowWrapCache{}
	}

	if list == nil || win == nil {
		return
	}

	// Compute client area (window size minus title bar and padding).
	clientW := win.GetSize().X
	clientH := win.GetSize().Y - win.GetTitleSize()
	// Adjust for window padding/border so child flows fit within clip region.
	s := UIScale()
	if win.NoScale {
		s = 1
	}
	pad := (win.Padding + win.BorderPad) * s
	clientWAvail := clientW - 2*pad
	if clientWAvail < 0 {
		clientWAvail = 0
	}
	clientHAvail := clientH - 2*pad
	if clientHAvail < 0 {
		clientHAvail = 0
	}

	// Compute a row height from actual font metrics (ascent+descent) to
	// avoid clipping at large sizes. Convert pixels to item units.
	ui := UIScale()
	if ui <= 0 {
		ui = 1
	}
	// Match the render-time face size used by EUI text items
	// (EUI renders with size = fontSize*ui + 2). Using the same value here
	// ensures wrap measurements align with what actually gets drawn.
	facePx := float64(float32(fontSize)*ui) + 2
	resolvedFaceSrc := faceSrc
	if resolvedFaceSrc == nil {
		resolvedFaceSrc = FontSource()
	}
	goFace := &text.GoTextFace{Source: resolvedFaceSrc, Size: facePx}
	metrics := goFace.Metrics()
	linePx := math.Ceil(metrics.HAscent + metrics.HDescent + 2) // +2 px padding
	rowUnits := float32(linePx) / ui

	// Prepare wrapping parameters: use the same face for measurement.
	var face text.Face = goFace
	// Reserve a gutter for the vertical scrollbar so wrapped text and
	// hitboxes never encroach beneath it. This applies broadly to chat,
	// console, help, and about windows using this helper.
	sb := ScrollbarWidth()
	contentW := clientWAvail - sb
	if contentW < 0 {
		contentW = 0
	}
	// Use the effective content width in pixels for wrapping.
	wrapWidthPx := float64(contentW - 3*pad)
	contentWUnits := contentW / ui
	clientWUnits := clientWAvail / ui
	clientHUnits := clientHAvail / ui
	if wrapCache.begin(textWindowWrapConfig{
		width:    wrapWidthPx,
		faceSize: facePx,
		source:   resolvedFaceSrc,
	}) {
		firstChanged = 0
	}
	if firstChanged < 0 || firstChanged > len(msgs) || firstChanged > len(list.Contents) {
		firstChanged = 0
	}

	for i := firstChanged; i < len(msgs); i++ {
		msg := msgs[i]
		wrapped, linesN := wrapCache.wrap(msg, face, wrapWidthPx)
		if i < len(list.Contents) {
			if list.Contents[i].Text != wrapped || list.Contents[i].FontSize != float32(fontSize) {
				list.Contents[i].Text = wrapped
				list.Contents[i].FontSize = float32(fontSize)
			}
			list.Contents[i].Face = face
			list.Contents[i].Size.Y = rowUnits * float32(linesN)
			list.Contents[i].Size.X = contentWUnits
			list.Contents[i].SelectableText = true
			list.Contents[i].Filled = alternateRows && i%2 == 1
			list.Contents[i].Color = SubtleAlternateRowColor()
			list.Contents[i].OnURLClick = options.OnURLClick
		} else {
			t, _ := NewText()
			t.Text = wrapped
			t.FontSize = float32(fontSize)
			t.Face = face
			t.Size = Point{X: contentWUnits, Y: rowUnits * float32(linesN)}
			t.SelectableText = true
			t.Filled = alternateRows && i%2 == 1
			t.Color = SubtleAlternateRowColor()
			t.OnURLClick = options.OnURLClick
			// Append to maintain ordering with the msgs index
			list.AddItem(t)
		}
	}
	wrapCache.finish(len(msgs))
	if len(list.Contents) > len(msgs) {
		for i := len(msgs); i < len(list.Contents); i++ {
			list.Contents[i] = nil
		}
		list.Contents = list.Contents[:len(msgs)]
	}

	var scrollInput bool
	if input != nil {
		scrollInput = input.ScrollAtBottom()
		// Soft-wrap the input message to the available width and grow the input area.
		_, inLines := WrapText(inputMsg, face, wrapWidthPx)
		wrappedIn := strings.Join(inLines, "\n")
		var miss []TextSpan
		if options.InputUnderlines != nil {
			miss = options.InputUnderlines(wrappedIn)
		}
		inLinesN := len(inLines)
		if inLinesN < 1 {
			inLinesN = 1
		}
		inputContentH := rowUnits * float32(inLinesN)
		maxInputH := clientHUnits / 2
		if inputContentH > maxInputH {
			input.Size.Y = maxInputH
			input.Scrollable = true
		} else {
			input.Size.Y = inputContentH
			input.Scrollable = false
		}
		input.Size.X = contentWUnits
		if len(input.Contents) == 0 {
			t, _ := NewText()
			t.Text = wrappedIn
			t.FontSize = float32(fontSize)
			t.Face = face
			t.Size = Point{X: contentWUnits, Y: inputContentH}
			t.Filled = true
			t.SelectableText = options.InputEditable
			t.EditableText = options.InputEditable
			t.Underlines = miss
			input.AddItem(t)
		} else {
			if input.Contents[0].Text != wrappedIn || input.Contents[0].FontSize != float32(fontSize) {
				input.Contents[0].Text = wrappedIn
				input.Contents[0].FontSize = float32(fontSize)
			}
			input.Contents[0].Face = face
			input.Contents[0].Size.X = contentWUnits
			input.Contents[0].Size.Y = inputContentH
			input.Contents[0].SelectableText = options.InputEditable
			input.Contents[0].EditableText = options.InputEditable
			input.Contents[0].Underlines = miss
		}
		if scrollInput {
			input.Scroll.Y = 1e9
		}
	}

	// Size the flow to the client area, and the list to fill above any bottom items and optional input.
	var extraH float32
	if list.Parent != nil {
		list.Parent.Size.X = clientWUnits
		list.Parent.Size.Y = clientHUnits
		for _, c := range list.Parent.Contents {
			if c != list && c != input {
				c.Size.X = clientWUnits
				extraH += c.Size.Y
			}
		}
	}
	list.Size.X = clientWUnits
	if input != nil {
		list.Size.Y = max(0, clientHUnits-input.Size.Y-extraH)
	} else {
		list.Size.Y = max(0, clientHUnits-extraH)
	}
	// Do not refresh here unconditionally; callers decide when to refresh.
}
