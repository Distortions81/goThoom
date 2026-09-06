# EUI

EUI is a retained-mode UI toolkit for Ebitengine 2.9: create widgets once,
update their state in your game loop, and draw the UI over your game. It provides
movable/resizable windows, automatic row/column layout, tabs, scrolling, text
inputs and selection, buttons, sliders, checkboxes, dropdowns, context menus,
tooltips, color pickers, and embedded themes.

The package is currently developed inside goThoom. Everything it needs is under
this directory, including the reusable `renderpool` subpackage; it does not
import client code or require game assets. Its import path is currently
`gothoom/eui`, not a published module URL.

## Quick start

Run the complete example from goThoom's `source` directory:

```sh
go run ./eui/examples/basic
```

Initialize the embedded fonts once, then build a window:

```go
if err := eui.Init(); err != nil {
    return err
}
win := eui.NewWindow()
win.Title = "Tools"
win.AutoSize, win.Movable, win.Closable = true, true, true
win.AddItem(eui.NewColumn(
    eui.NewLabel("Ready"),
    eui.NewActionButton("Hello", func() {
        eui.ShowPopup("Hello", "It works!", []eui.PopupButton{{Text: "OK"}})
    }),
))
win.MarkOpen()
```

Connect these three calls to your `ebiten.Game`:

```go
func (g *Game) Update() error { return eui.Update() }
func (g *Game) Draw(screen *ebiten.Image) {
    // Draw your game first.
    eui.Draw(screen)
}
func (g *Game) Layout(w, h int) (int, int) { return eui.Layout(w, h) }
```

`Layout` handles display scaling. If your game owns a different framebuffer
layout, call `SetScreenSize` with its actual dimensions and manage UI scaling
explicitly. Most widget sizes use logical UI units. Set `Size` when you need an
explicit width or height; `Fixed` and `Scrollable` create bounded viewports.

## Building UI

- `NewColumn`, `NewRow`, `NewLabel`, and `NewActionButton` cover basic composition.
  `NewSection` and `NewSubheading` add consistent configuration headings.
- The original widget constructors return an item and event handler for detailed
  control. Populate the item and assign `events.Handle` for the events you need.
- `ShowPopup` opens a wrapped, selectable message with action buttons and optional
  extra widgets. It is a floating dialog, **not an input-blocking modal**. Actions
  run before close; its window is removed on close.
- `ShowColorPicker` opens an independent HSV/opacity editor. Cancel never applies
  changes. `NewColorSwatch` supplies a picker button; pass a nil chooser to use
  the built-in picker or a callback to manage a shared picker yourself.
- `NewTextWindow` returns a registered, initially closed window, its scrolling
  list, and an optional input display. `UpdateTextWindow` wraps and reuses rows.
  Keep one `TextWindowWrapCache` per window; its zero value works. Options provide
  URL callbacks, editability, and input annotation spans. Input display editing
  still needs application text/keyboard handling; this is not `NewInput`.
  Pass `FirstChanged: 0` when changing styles or callbacks. Call `win.Refresh()`
  after updates when a repaint is needed; the updater does not force one.
- `WrapText` preserves whitespace and breaks oversized words.

Call UI APIs on the game/UI thread. The current library has one global UI
context; independent contexts and concurrent UI mutations are not supported.
`Close` hides ordinary windows so they can reopen; call `RemoveWindow` to retire
them. Temporary popup and color picker windows remove themselves automatically.

## Fonts and themes

`Init` uses embedded Go regular/bold fonts and preserves fonts supplied earlier
with `SetFontSource` / `SetBoldFontSource`. No filesystem assets are required.
Built-in palettes and styles are embedded. `LoadTheme`, `LoadStyle`, and
`SetUserUIScale` configure appearance. `SetUserDataRoot` explicitly enables an
application-specific location for editable theme files and writes examples there.
EUI never changes the process working directory.

## Standalone extraction check

From the repository root:

```sh
./build-scripts/check_eui_standalone.sh
```

This copies **only this directory** into a temporary Go module, resolves its own
dependencies, runs its tests, and builds the example. Use Go 1.26.6 and the usual
Ebitengine platform development dependencies. Tests need a display; on headless
Linux run the command with `xvfb-run -a`.

To publish a separate repository, copy this directory and the root MIT `LICENSE`,
create `go.mod` with the chosen repository module path, and update the import
prefix in `potato.go` and `examples/basic/main.go`. Run `go mod tidy`, tests,
and the example. The extraction script records the currently tested dependency
versions. No separate repository, branch, tag, or release is created here.
