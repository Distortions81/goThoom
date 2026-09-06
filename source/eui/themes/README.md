# Themes

This directory holds the built-in color palettes and style themes used by EUI. Themes are JSON files that control the appearance and spacing of all widgets. You can load them at runtime or create your own variants. Set `eui.AutoReload = true` to have changes picked up automatically while editing.

## Loading Themes

Apply a palette and style from Go code:

```go
if err := eui.LoadTheme("AccentDark"); err != nil {
    log.Println(err)
}
if err := eui.LoadStyle("Breeze"); err != nil {
    log.Println(err)
}
```

With `eui.AutoReload` enabled the `themes` folder is watched and files are reloaded on change.

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
  "RecommendedStyle": "Breeze"
}
```

Each widget block (`Window`, `Button`, `Text`, `Checkbox`, `Radio`, `Input`, `Slider`, `Dropdown`, `Progress`, `Tab`) accepts the following fields:

- `TextColor` – color used for text labels
- `Color` – main background fill
- `HoverColor` – color when the pointer is over the widget
- `ClickColor` – color when the widget is clicked
- `OutlineColor` – color of the outline if enabled
- `DisabledColor` – background color when the widget is disabled
- `DisabledTextColor` – optional separate disabled caption color; omitted palettes retain the legacy single-color disabled appearance
- `SelectedColor` – color for selected state (tabs, sliders, dropdowns)
- `MaxVisible` – for dropdowns, the maximum visible entries (0 shows as many as fit on screen)

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

## Breeze-inspired defaults

`AccentDark` and `AccentLight` use the neutral surfaces and blue accent of
[KDE Breeze Dark](https://github.com/KDE/breeze/blob/master/colors/BreezeDark.colors)
and [Breeze Light](https://github.com/KDE/breeze/blob/master/colors/BreezeLight.colors).
Both recommend the `Breeze` style: thin borders, restrained corner rounding,
and consistent control padding. Hover and disabled colors are adapted to EUI.
The accent remains adjustable, and another style can still be chosen separately.

## Built-in Themes

Each palette has a distinct purpose:

| Palette | Character | Recommended style |
| --- | --- | --- |
| AccentDark | Breeze charcoal surfaces and an adjustable blue accent | Breeze |
| AccentLight | Breeze light gray surfaces and an adjustable blue accent | Breeze |
| NeonNight | Dark violet, purple borders, and luminous cyan selections | Rounded |
| Midnight | Pure black backgrounds with restrained silver accents | Flat |
| Paper | Warm ivory and sepia for a softer light appearance | Rounded |
| Forest | Evergreen surfaces and muted sage accents | Breeze |
| Dusk | Warm plum surfaces and rose accents | Rounded |
| HighContrast | Black and white with yellow selections and strong boundaries | HighContrast |
| Arcade | Navy surfaces, lime selections, and bold orange borders for a playful dark look | HighContrast |
| Candy | Soft rose surfaces, berry ink, and mint hover states | Borderless |
| SeaGlass | Pale green surfaces with quiet teal accents for a calm light look | Borderless |
| Coffee | Espresso surfaces, cream text, and muted caramel accents | Borderless |

Styles can be paired with any palette:

| Style | Geometry |
| --- | --- |
| Breeze | Thin borders, modest rounding, balanced padding |
| Flat | Compact controls, quiet fills, minimal button borders |
| Borderless | Soft filled controls; no window, control, or tab outlines |
| Rounded | Softer corners and roomier content padding |
| Outline | Square, unfilled buttons with visible control boundaries |
| HighContrast | Square corners, two-pixel borders, generous content padding |

Retired names in saved settings map to their closest replacement. For example,
`Black` loads `Midnight`, `ForestMist` loads `Forest`, and `SoftNeutral` loads
`Paper`. Old rounded styles load `Rounded`, outline styles load `Outline`,
and `SolidBlock` loads `HighContrast`. A user-authored file with an old name
still takes priority. `AccentDark`, `AccentLight`, and `NeonNight` retain their
names.

Use `eui.ListThemes()` and `eui.ListStyles()` to get these names at runtime.

## Creating Your Own

1. Copy an existing file from `palettes` or `styles` as a starting point.
2. Adjust the values or add new color names in the `Colors` map.
3. Save the file under the appropriate directory with a new name.
4. Call `eui.LoadTheme("YourTheme")` and `eui.LoadStyle("YourStyle")` to apply them. Enabling `eui.AutoReload` helps when iterating on your design.

On first run the client writes an `Example.json` palette and style alongside this README. Copy and modify them to get started quickly.

## Other Customizations

- **Background and splash images** – place `background.png` and/or `splash.png` at the root of the user data folder to override the startup visuals.
- **Sound font** – drop a `soundfont.sf2` file into the user data folder to replace the default music instrument set. The Download Files window can fetch a recommended one or you can supply any General MIDI sound font.
- **TTS voices** – download Piper voices (`.tar.gz` archives or `.onnx` with matching `.onnx.json`) and place them in `piper/voices` under the user data folder. Use the Download Files window for English voices or fetch others from online voice collections.

## Readability and style switching

Built-in palettes keep large surfaces neutral or gently tinted. Normal, hovered,
and selected surfaces keep at least 4.5:1 text contrast; disabled captions have
separate muted colors. Buttons, inputs, tabs, selection text, and dropdowns
choose contrasting text when a custom accent would otherwise hide it. Outline
styles use the window surface for text contrast when a control has no fill.

Styles load omitted properties from the compiled defaults, so switching styles
in a different order produces the same result. Inputs, dropdowns, checkboxes,
and radio buttons use fills to remain recognizable in Borderless; other styles
can add outlines. Candy, SeaGlass, and Coffee recommend Borderless, and it can
be selected with any palette. Existing
palette and style names remain loadable; `Example.json` files are editable
templates and are omitted from the selection lists.

## Live previews

In Settings and the setup wizard, hovering a palette or style previews the UI.
Click an option to keep it. Moving away, pressing Escape, clicking outside, or
closing the window restores the previous palette, style, and custom accent.
Hovering never writes a preference. The open menu keeps its original font size,
spacing, and position so previewing cannot move options under the pointer.

## Color picker

Appearance and Text Colors show compact color swatches. Click one to open a
picker with a wheel and controls for hue, saturation, brightness, and opacity.
Apply saves the color; Cancel or closing the picker leaves the original color
intact.

## Visual checks

From `source/`, render the built-in palettes and styles with normal, hovered,
pressed, disabled, selected, and custom-accent states:

```sh
GOTHOOM_RENDER_THEME_GALLERY=1 GOTHOOM_THEME_GALLERY_DIR=/tmp/gothoom-themes \
  xvfb-run -a go test ./eui -run '^TestRenderThemeGallery$' -count=1
```

This opt-in test also checks clipped border pixels and monochrome icon contrast.

## Shadows and glows

The palette's `Window` and `Dropdown` blocks accept these optional fields:

```json
"ShadowSize": 16,
"ShadowColor": "#00000080",
"ShadowFalloff": 2
```

`ShadowSize` is the outward reach in logical pixels; zero disables the effect.
`ShadowColor` accepts a named color or straight RGB plus alpha (`#RRGGBBAA`).
Black or another dark color produces a shadow; a bright color produces a glow.
Alpha controls strength. `ShadowFalloff` controls the fade from the edge: 1 is
linear, 2 is a softer falloff, and larger values concentrate the effect near
the window. Omitted or nonpositive falloff uses 2; supported exponents are
0.25 through 8, with an outward reach capped at 128 screen pixels.

These effects preview with the palette. NeonNight and Arcade use colored glows;
HighContrast and Midnight disable outer effects. The global Window Shadows
setting disables both shadows and glows. Docked panes have no outer shadow.
Window shadows are composited outside the window cache using shared corner and
edge masks, so moving or resizing a window does not require a full-size shadow
texture.

## Shared control rendering

Repeated control shapes share white coverage masks, tinted with their current
theme or state color when drawn. Rounded fills and borders, slider thumbs and
tracks, checkboxes, radio buttons, checkmarks, small lines, and dropdown arrows
reuse these masks. Larger rounded shapes stretch their straight middle bands;
small controls keep a complete mask. Pixel-aligned square fills and borders
use shared image quads directly.

The least-recently-used cache is limited to 256 entries and 2 MiB of mask pixels
(excluding graphics-driver overhead). Keys include pixel geometry, so UI scale
and border changes select the appropriate mask; changing only colors does not
create new entries. Switching Potato GPU mode clears the cache. Unusually large
or complex shapes fall back to vector rendering. Whole-window caching still
avoids redrawing unchanged controls, and open dropdowns can reuse the shapes too.

From `source/`, compare cached coverage against the vector paths and measure a
repeated-control workload:

```sh
GOTHOOM_RENDER_PRIMITIVE_CACHE=1 \
  xvfb-run -a go test ./eui -run '^TestRenderPrimitiveCache$' -count=1 -v
```

The check covers clipping, multiple scales, translucent colors, cache reuse,
and both cache limits. It allows one antialiasing sample at a few boundary
pixels where translating a vector curve changes floating-point rounding.
Its timing includes readback and is a focused rendering comparison, not a
measurement of the client's overall frame rate.
