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
	Scroll          point
	NoScroll        bool
	NoScale         bool
	NoBGColor       bool
	AlwaysDrawFirst bool
	NoCache         bool

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

	// refreshInterval coalesces repeated invalidations for large, mostly
	// static windows. The first change is immediate; further changes within
	// the interval are rendered together.
	refreshInterval time.Duration
	lastRefresh     time.Time
	refreshPending  bool

	// Drop shadow styling
	ShadowSize  float32
	ShadowColor Color

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
	Name      string
	Text      string
	Label     string
	Tooltip   string
	tooltipW  float32 // cached tooltip text width
	tooltipH  float32 // cached tooltip text height
	Position  point
	Size      point
	Alignment alignType
	PinTo     pinType
	FontSize  float32
	Face      text.Face
	LineSpace float32 //Multiplier, 1.0 = no gap between lines
	ItemType  itemTypeData

	Value      float32
	MinValue   float32
	MaxValue   float32
	IntOnly    bool
	RadioGroup string

	Hovered, Checked, Focused,
	Disabled, Invisible bool
	Clicked     time.Time
	FlowType    flowType
	Scroll      point
	ScrollMarks []float32

	// Dropdown specific
	Options    []string
	Selected   int
	Open       bool
	MaxVisible int
	HoverIndex int

	// HeaderCount marks the number of initial options that are shown as
	// non-interactive headers in dropdowns/context menus. These indices are
	// rendered with the disabled text color, are not hover-highlighted, and
	// do not trigger selection on click.
	HeaderCount int

	OnSelect func(int)
	OnHover  func(int)

	Fixed, Scrollable bool

	ImageName string
	Image     *ebiten.Image

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
	ForceTextColor bool

	Action        func()
	OnColorChange func(Color)
	WheelColor    Color
	TextPtr       *string
	Underlines    []TextSpan
	SecretText    string
	HideText      bool
	CursorPos     int
	// SelectableText allows a read-only text item to be drag-selected and copied.
	SelectableText         bool
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
	ShadowSize  float32
	ShadowColor Color

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
	Start int
	End   int
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
