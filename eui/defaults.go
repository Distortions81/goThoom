package eui

var defaultTheme = &windowData{
	TitleHeight:     24,
	Border:          0,
	Outlined:        false,
	Fillet:          4,
	Padding:         4,
	Margin:          4,
	BorderPad:       0,
	TitleColor:      NewColor(255, 255, 255, 255),
	TitleTextColor:  NewColor(255, 255, 255, 255),
	TitleBGColor:    NewColor(64, 64, 64, 255),
	CloseBGColor:    NewColor(0, 0, 0, 0),
	DragbarSpacing:  5,
	ShowDragbar:     false,
	BorderColor:     NewColor(64, 64, 64, 255),
	SizeTabColor:    NewColor(48, 48, 48, 255),
	DragbarColor:    NewColor(64, 64, 64, 255),
	HoverTitleColor: NewColor(0, 160, 160, 255),
	HoverColor:      NewColor(80, 80, 80, 255),
	BGColor:         NewColor(32, 32, 32, 255),
	ActiveColor:     NewColor(0, 160, 160, 255),

	ShadowSize:  16,
	ShadowColor: NewColor(0, 0, 0, 160),

	Movable: true, Closable: true, Resizable: true, Open: true, AutoSize: false,
	NoBGColor: false,
}

var defaultButton = &itemData{
	Text:      "",
	ItemType:  ITEM_BUTTON,
	Size:      point{X: 128, Y: 64},
	Position:  point{X: 4, Y: 4},
	FontSize:  12,
	LineSpace: 1.2,

	Padding: 0,
	Margin:  4,

	Fillet: 8,
	Filled: true, Outlined: false,
	Border:    0,
	BorderPad: 4,

	TextColor:    NewColor(255, 255, 255, 255),
	Color:        NewColor(48, 48, 48, 255),
	HoverColor:   NewColor(96, 96, 96, 255),
	ClickColor:   NewColor(0, 160, 160, 255),
	OutlineColor: NewColor(48, 48, 48, 255),
}

var defaultText = &itemData{
	Text:      "",
	ItemType:  ITEM_TEXT,
	Size:      point{X: 128, Y: 128},
	Position:  point{X: 4, Y: 4},
	FontSize:  24,
	LineSpace: 1.2,
	Padding:   0,
	Margin:    2,
	TextColor: NewColor(255, 255, 255, 255),
}

var defaultCheckbox = &itemData{
	Text:      "",
	ItemType:  ITEM_CHECKBOX,
	Size:      point{X: 128, Y: 24},
	Position:  point{X: 4, Y: 2},
	AuxSize:   point{X: 16, Y: 16},
	AuxSpace:  4,
	FontSize:  12,
	LineSpace: 1.2,
	Padding:   0,
	Margin:    2,

	Fillet: 8,
	Filled: true, Outlined: false,
	Border:    0,
	BorderPad: 4,

	TextColor:    NewColor(255, 255, 255, 255),
	Color:        NewColor(48, 48, 48, 255),
	HoverColor:   NewColor(96, 96, 96, 255),
	ClickColor:   NewColor(0, 160, 160, 255),
	OutlineColor: NewColor(0, 160, 160, 255),
}

var defaultInput = &itemData{
	ItemType:  ITEM_INPUT,
	Size:      point{X: 128, Y: 24},
	Position:  point{X: 4, Y: 4},
	FontSize:  12,
	LineSpace: 1.2,
	Padding:   0,
	Margin:    2,

	Fillet: 4,
	Filled: true, Outlined: false,
	Border:    0,
	BorderPad: 2,

	TextColor:    NewColor(255, 255, 255, 255),
	Color:        NewColor(48, 48, 48, 255),
	HoverColor:   NewColor(96, 96, 96, 255),
	ClickColor:   NewColor(0, 160, 160, 255),
	OutlineColor: NewColor(0, 160, 160, 255),
}

var defaultRadio = &itemData{
	Text:      "",
	ItemType:  ITEM_RADIO,
	Size:      point{X: 128, Y: 24},
	Position:  point{X: 4, Y: 2},
	AuxSize:   point{X: 16, Y: 16},
	AuxSpace:  4,
	FontSize:  12,
	LineSpace: 1.2,
	Padding:   0,
	Margin:    2,

	Fillet: 8,
	Filled: true, Outlined: false,
	Border:    0,
	BorderPad: 4,

	TextColor:    NewColor(255, 255, 255, 255),
	Color:        NewColor(48, 48, 48, 255),
	HoverColor:   NewColor(96, 96, 96, 255),
	ClickColor:   NewColor(0, 160, 160, 255),
	OutlineColor: NewColor(0, 160, 160, 255),
}

var defaultSlider = &itemData{
	ItemType: ITEM_SLIDER,
	Size:     point{X: 128, Y: 24},
	Position: point{X: 4, Y: 4},
	AuxSize:  point{X: 8, Y: 16},
	AuxSpace: 4,
	FontSize: 12,
	Padding:  0,
	Margin:   4,

	MinValue: 0,
	MaxValue: 100,
	Value:    0,
	IntOnly:  false,

	Fillet: 4,
	Filled: true, Outlined: false,
	Border:    0,
	BorderPad: 2,

	TextColor:    NewColor(255, 255, 255, 255),
	Color:        NewColor(48, 48, 48, 255),
	HoverColor:   NewColor(96, 96, 96, 255),
	ClickColor:   NewColor(0, 160, 160, 255),
	OutlineColor: NewColor(0, 160, 160, 255),
}

var defaultDropdown = &itemData{
	ItemType: ITEM_DROPDOWN,
	Size:     point{X: 128, Y: 24},
	Position: point{X: 4, Y: 4},
	FontSize: 12,
	Padding:  0,
	Margin:   4,

	Fillet: 4,
	Filled: true, Outlined: false,
	Border:    0,
	BorderPad: 2,

	TextColor:    NewColor(255, 255, 255, 255),
	Color:        NewColor(48, 48, 48, 255),
	HoverColor:   NewColor(96, 96, 96, 255),
	ClickColor:   NewColor(0, 160, 160, 255),
	OutlineColor: NewColor(0, 160, 160, 255),
	MaxVisible:   5,

	ShadowSize:  16,
	ShadowColor: NewColor(0, 0, 0, 160),
}

var defaultColorWheel = &itemData{
	ItemType:      ITEM_COLORWHEEL,
	Size:          point{X: 160, Y: 128},
	Position:      point{X: 4, Y: 4},
	Margin:        4,
	OnColorChange: nil,
	WheelColor:    NewColor(255, 0, 0, 255),
}

var defaultTab = &itemData{
	ItemType:      ITEM_FLOW,
	FontSize:      12,
	Filled:        true,
	Border:        0,
	BorderPad:     2,
	Fillet:        4,
	ActiveOutline: true,
	TextColor:     NewColor(255, 255, 255, 255),
	Color:         NewColor(64, 64, 64, 255),
	HoverColor:    NewColor(96, 96, 96, 255),
	ClickColor:    NewColor(0, 160, 160, 255),
	OutlineColor:  NewColor(0, 160, 160, 255),
}

// base copies preserve the initial defaults so that LoadTheme can reset
// to these values before applying theme overrides.
var (
	baseWindow     = *defaultTheme
	baseButton     = *defaultButton
	baseText       = *defaultText
	baseCheckbox   = *defaultCheckbox
	baseRadio      = *defaultRadio
	baseInput      = *defaultInput
	baseSlider     = *defaultSlider
	baseDropdown   = *defaultDropdown
	baseColorWheel = *defaultColorWheel
	baseTab        = *defaultTab
	baseTheme      = &Theme{
		Window:   baseWindow,
		Button:   baseButton,
		Text:     baseText,
		Checkbox: baseCheckbox,
		Radio:    baseRadio,
		Input:    baseInput,
		Slider:   baseSlider,
		Dropdown: baseDropdown,
		Tab:      baseTab,
	}
)
