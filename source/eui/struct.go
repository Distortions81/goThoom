package eui

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
)

type Color color.RGBA

func (c Color) RGBA() (r, g, b, a uint32) {
	cc := color.RGBA(c)
	return cc.RGBA()
}

func (c Color) ToRGBA() color.RGBA { return color.RGBA(c) }

type windowData struct {
	Title    string
	Position point
	Size     point

	zone *windowZone

	snapAnchor       point
	snapAnchorActive bool

	Padding   float32
	Margin    float32
	Border    float32
	BorderPad float32
	Fillet    float32
	Outlined  bool

	Open, Hovered, Flow,
	Closable, Movable, Resizable, Maximizable, Searchable,
	HoverClose, HoverDragbar, HoverMax, HoverSearch,
	searchOpen, AutoSize bool

	// Scroll position and behavior
	Scroll    point
	NoScroll  bool
	NoScale   bool
	NoBGColor bool
	// Docked marks a pane whose position and size are owned by a tiled
	// workspace. Docked panes do not expose standalone move/resize hit areas.
	Docked          bool
	AlwaysDrawFirst bool
	NoCache         bool

	// ShowTooltipIndicators marks controls with hover help in configuration windows.
	ShowTooltipIndicators bool

	TitleHeight float32

	// Visual customization
	BGColor, TitleBGColor, TitleColor, TitleTextColor, BorderColor,
	SizeTabColor, DragbarColor, CloseBGColor Color

	// Dragbar behavior
	DragbarSpacing float32
	ShowDragbar    bool

	HoverTitleColor, HoverColor, ActiveColor Color

	Contents []*itemData

	DefaultButton *itemData

	Theme *Theme

	// Render caches the pre-rendered image for this window when Dirty is
	// false.
	Render *ebiten.Image
	Dirty  bool

	// DeferRepaint lets large background panes share a per-frame repaint
	// budget. Interaction, resizing and initial painting remain immediate.
	DeferRepaint     bool
	repaintRequested time.Time
	repaintReason    string
	repaintStats     RepaintStats

	// refreshInterval coalesces repeated invalidations for large, mostly
	// static windows. The first change is immediate; further changes within
	// the interval are rendered together.
	refreshInterval time.Duration
	lastRefresh     time.Time
	refreshPending  bool

	// Drop shadow styling
	ShadowSize    float32
	ShadowColor   Color
	ShadowFalloff float32 // Positive exponent; larger values fade faster away from the edge.

	// RenderCount tracks how often the window has been drawn.
	RenderCount int

	// SearchText holds the current text in the window's search box.
	SearchText string

	// HasIndeterminate reports whether any contained progress bar is
	// currently indeterminate.
	HasIndeterminate bool

	// OnClose is an optional callback invoked when the window is closed,
	// either by user action or programmatically. The callback runs before the
	// window is removed from the active list.
	OnClose func()

	// OnOpen is an optional callback invoked when a closed window is opened.
	// It runs before the first Refresh so clients can rebuild deferred content
	// without exposing a stale frame.
	OnOpen func()

	// OnResize is an optional callback invoked when the window's size changes
	// due to user interaction or programmatic updates.
	OnResize func()

	// OnMaximize is an optional callback invoked when the user clicks the
	// titlebar maximize button. If unset, a default Maximize() is performed.
	OnMaximize func()

	// OnSearch is an optional callback invoked on every change of the search
	// text when the search box is active.
	OnSearch func(string)

	// Opacity controls the overall window opacity when composited to the
	// screen. Range [0,1], where 1 is fully opaque. Defaults to 1.
	Opacity float32
}

type itemData struct {
	Parent       *itemData
	ParentWindow *windowData
	// Name is used when the item is part of a tabbed flow
	Name       string
	Text       string
	Label      string
	Tooltip    string
	tooltipRaw string
	tooltipW   float32 // cached tooltip text width
	tooltipH   float32 // cached tooltip text height
	Position   point
	Size       point
	Alignment  alignType
	PinTo      pinType
	FontSize   float32
	Face       text.Face
	LineSpace  float32 //Multiplier, 1.0 = no gap between lines
	ItemType   itemTypeData

	Value      float32
	MinValue   float32
	MaxValue   float32
	IntOnly    bool
	HideValue  bool // Hide a slider's value caption when an adjacent field supplies it.
	RadioGroup string

	Hovered, Checked, Focused,
	Disabled, Invisible bool
	Clicked     time.Time
	FlowType    flowType
	Scroll      point
	ScrollMarks []float32

	// Dropdown specific
	Options []string
	// OptionImages optionally adds a leading image to matching dropdown or
	// context-menu rows. Missing and nil entries retain the shared text inset.
	OptionImages []*ebiten.Image
	Selected     int
	Open         bool
	MaxVisible   int
	HoverIndex   int

	// Preview menus retain their opening geometry until dismissed.
	dropdownLayout *dropdownLayout

	// HeaderCount marks the number of initial options that are shown as
	// non-interactive headers in dropdowns/context menus. These indices are
	// rendered with the disabled text color, are not hover-highlighted, and
	// do not trigger selection on click.
	HeaderCount int

	OnSelect func(int)
	// OnHover receives an option index, then Selected when hover ends.
	// For dropdowns HoverIndex is -1 when the pointer leaves or the menu closes.
	OnHover func(int)

	Fixed, Scrollable bool

	ImageName string
	Image     *ebiten.Image
	// SmoothImage enables filtered scaling for antialiased vector-derived
	// images. Pixel-art images keep nearest-neighbor scaling by default.
	SmoothImage bool
	// TintImage draws a monochrome button icon in the current text color.
	TintImage bool
	// ColorSwatch fills a button with WheelColor, keeping its caption readable.
	ColorSwatch bool

	//Style
	Padding, Margin float32

	Fillet            float32
	Border, BorderPad float32
	Filled, Outlined  bool
	ActiveOutline     bool
	AuxSize           point
	AuxSpace          float32
	Vertical          bool

	TextColor, Color, HoverColor,
	ClickColor, OutlineColor, DisabledColor, SelectedColor Color
	// DisabledTextColor keeps disabled captions readable above DisabledColor.
	// Zero preserves the legacy single-color disabled appearance.
	DisabledTextColor Color
	ForceTextColor    bool

	Action        func()
	OnColorChange func(Color)
	WheelColor    Color
	TextPtr       *string
	Underlines    []TextSpan
	// Prediction is rendered after Text in the disabled text color. It is
	// display-only and is not included in selection or cursor positions.
	Prediction string
	SecretText string
	HideText   bool
	CursorPos  int
	// SelectableText allows text to be drag-selected and copied. EditableText is
	// required separately for ITEM_TEXT values that accept keyboard changes.
	SelectableText         bool
	EditableText           bool
	SelectStart, SelectEnd int
	selecting              bool
	// OnURLClick is called when a HTTP(S) URL in a text item is clicked.
	OnURLClick func(string)
	Handler    *EventHandler
	Contents   []*itemData

	// Tabs allows a flow to contain multiple tabbed flows. Only the
	// flow referenced by ActiveTab will be drawn and receive input.
	Tabs      []*itemData
	ActiveTab int
	// TabColumns wraps the tab strip after this many tabs. Zero keeps one row.
	TabColumns int
	// TabRowOffset indents every other wrapped tab row in logical pixels.
	TabRowOffset float32

	Theme *Theme
	// DrawRect stores the last drawn rectangle of the item in screen
	// coordinates so input handling can use the exact same area that was
	// rendered.
	DrawRect rect

	// Render caches the pre-rendered image for this item when Dirty is
	// false. Flows are never cached.
	Render *ebiten.Image
	Dirty  bool

	// Drop shadow styling
	ShadowSize    float32
	ShadowColor   Color
	ShadowFalloff float32 // Positive exponent; larger values fade faster away from the edge.

	// RenderCount tracks how often the item has been drawn.
	RenderCount int

	// Indeterminate indicates that the widget should render an animated
	// barber-pole style progress when exact value is unknown.
	Indeterminate bool

	// Slider drag tracking
	dragStart      point
	dragStartInit  bool
	dragStartValue float32
}

type roundRect struct {
	Size, Position point
	Fillet, Border float32
	Filled         bool
	Color          Color
}

type rect struct {
	X0, Y0, X1, Y1 float32
}

type point struct {
	X, Y float32
}

type TextSpan struct {
	Start          int
	End            int
	MatchTextColor bool
}

type flowType int

const (
	FLOW_HORIZONTAL = iota
	FLOW_VERTICAL

	FLOW_HORIZONTAL_REV
	FLOW_VERTICAL_REV
)

type alignType int

const (
	ALIGN_NONE = iota
	ALIGN_LEFT
	ALIGN_CENTER
	ALIGN_RIGHT
)

type pinType int

const (
	PIN_TOP_LEFT = iota
	PIN_TOP_CENTER
	PIN_TOP_RIGHT

	PIN_MID_LEFT
	PIN_MID_CENTER
	PIN_MID_RIGHT

	PIN_BOTTOM_LEFT
	PIN_BOTTOM_CENTER
	PIN_BOTTOM_RIGHT
)

type dragType int

const (
	PART_NONE = iota

	PART_BAR
	PART_CLOSE
	PART_MAXIMIZE
	PART_SEARCH

	PART_TOP
	PART_RIGHT
	PART_BOTTOM
	PART_LEFT

	PART_TOP_RIGHT
	PART_BOTTOM_RIGHT
	PART_BOTTOM_LEFT
	PART_TOP_LEFT

	PART_SCROLL_V
	PART_SCROLL_H
)

type itemTypeData int

const (
	ITEM_NONE = iota
	ITEM_FLOW
	ITEM_TEXT
	ITEM_BUTTON
	ITEM_CHECKBOX
	ITEM_RADIO
	ITEM_INPUT
	ITEM_SLIDER
	ITEM_DROPDOWN
	ITEM_COLORWHEEL
	ITEM_IMAGE
	ITEM_IMAGE_FAST
	ITEM_PROGRESS
)

// Exported type aliases for library consumers

type WindowData = windowData

type ItemData = itemData

type RoundRect = roundRect

type Rect = rect

type Point = point

type FlowType = flowType
type AlignType = alignType
type PinType = pinType
type DragType = dragType
type ItemTypeData = itemTypeData
