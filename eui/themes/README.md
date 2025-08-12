# Themes

This directory holds the built-in color palettes and style themes used by EUI. Themes are JSON files that control the appearance and spacing of all widgets. You can load them at runtime or create your own variants.

## Palettes

Color palettes live under `themes/palettes`. Each file defines a `Colors` map followed by style blocks for each widget type. Colors may be written as `#RRGGBBAA` hexadecimal strings or as HSV triples (`h,s,v`). Entries in the `Colors` map can be referenced by name in later fields.

### Structure

```json
{
  "Comment": "Optional description",
  "Colors": {
    "background": "210,0.16,0.15",
    "panel": "214.3,0.15,0.19",
    "accent": "200.6,0.74,0.91"
  },
  "Window": {
    "Padding": 8,
    "BGColor": "background",
    "TitleBGColor": "panel",
    "ActiveColor": "accent"
  },
  "Button": { "TextColor": "accent", "Color": "panel" },
  ...
  "RecommendedStyle": "RoundHybrid"
}
```

Each widget block (`Window`, `Button`, `Text`, `Checkbox`, `Radio`, `Input`, `Slider`, `Dropdown`, `Tab`) accepts the following fields:

- `TextColor` – color used for text labels
- `Color` – main background fill
- `HoverColor` – color when the pointer is over the widget
- `ClickColor` – color when the widget is clicked
- `OutlineColor` – color of the outline if enabled
- `DisabledColor` – color when the widget is disabled
- `SelectedColor` – color for selected state (tabs, sliders, dropdowns)
- `MaxVisible` – for dropdowns, the maximum visible entries

The `Window` block also supports `TitleColor`, `TitleBGColor`, `BorderColor`, `SizeTabColor`, `DragbarColor`, `HoverTitleColor`, `HoverColor`, `ActiveColor` and `TitleTextColor`.

`RecommendedStyle` hints at a style theme that pairs well with the palette.

## Styles

Style themes are stored in `themes/styles`. They modify padding, border radius and other geometry.

### Structure

```json
{
  "SliderValueGap": 16,
  "DropdownArrowPad": 8,
  "TextPadding": 8,
  "Fillet": { "Button": 8, "Input": 4 },
  "Border": { "Button": 1 },
  "BorderPad": { "Button": 4 },
  "Filled": { "Button": true },
  "Outlined": { "Button": true },
  "ActiveOutline": { "Tab": true }
}
```

- `SliderValueGap` – space between the slider knob and value text
- `DropdownArrowPad` – padding before the dropdown arrow icon
- `TextPadding` – internal padding used by text widgets
- `Fillet` – corner rounding radius per widget
- `Border` – border width around widgets
- `BorderPad` – space between the border and widget content
- `Filled` – whether the widget background is filled
- `Outlined` – whether an outline is drawn
- `ActiveOutline` – highlight outline when active (tabs)

## Built-in Themes

Palettes:
`AccentDark`, `AccentLight`, `Black`, `ConcreteGray`, `CorporateBlue`, `ForestMist`, `HighContrast`, `NeonNight`, `OceanWave`, `SlateNight`, `SoftNeutral`, `SolarFlare`.

Styles:
`CleanLines`, `MinimalFade`, `MinimalPro`, `MonoEdge`, `NeoRounded`, `RoundFlat`, `RoundHybrid`, `RoundOutline`, `SharpEdge`, `SoftRound`, `SolidBlock`, `SquareFlat`, `SquareOutline`, `ThinOutline`.

Use `eui.ListThemes()` and `eui.ListStyles()` to get these names at runtime.

## Creating Your Own

1. Copy an existing file from `palettes` or `styles` as a starting point.
2. Adjust the values or add new color names in the `Colors` map.
3. Save the file under the appropriate directory with a new name.
4. Call `eui.LoadTheme("YourTheme")` and `eui.LoadStyle("YourStyle")` to apply them. Enabling `eui.AutoReload` helps when iterating on your design.

