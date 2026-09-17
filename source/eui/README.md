# EUI

EUI is a retained-mode UI toolkit for Ebitengine 2.10: create widgets once,
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
Set `ConstrainToSize` when a non-scrollable flow must not expand to its children,
or when a fixed-width control should clip a long caption instead of widening.

## Building UI

- `NewColumn`, `NewRow`, `NewOverlay`, `NewLabel`, and `NewActionButton` cover basic composition.
  Overlay children share one origin, so icon actions can sit inside a larger control without changing its layout width.
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
Set `BeforeClose` to a callback returning false to keep a window open while
resolving unsaved changes. `OnClose` runs only when closing is allowed.

## Editing text

`NewInput` provides single-line editing. `NewTextArea` provides a bounded,
multiline viewport with horizontal and vertical scrolling:

```go
editor, events := eui.NewTextArea()
editor.Size = eui.Point{X: 480, Y: 260}
editor.Text = "First line\nSecond line"
editor.AcceptTab = true // Optional: Tab indents; Ctrl+Tab still moves focus.
events.Handle = func(event eui.UIEvent) {
    if event.Type == eui.EventInputChanged {
        draft = editor.Text
    }
}
win.AddItem(editor)
```

Both controls support click placement, a blinking caret, drag selection,
Shift-click, double-click word selection, and triple-click line selection.
Typing and pasting replace the selection. Arrow keys, Home/End, word movement,
Backspace/Delete, and Shift selection work with held-key repeat. Movement and
deletion keep combined accents and emoji together. Up/Down preserve the desired
horizontal position across short lines; Page Up/Down move through multiline text.

Use Ctrl+A/X/C/V/Z on Windows and Linux, or Command on macOS, for select all,
cut, copy, paste, and undo. Shift+Ctrl/Command+Z and Ctrl+Y redo. Mac Option moves
or deletes by word, and Command+Left/Right moves to the line boundaries.
Command+Up/Down and Ctrl+Home/End move to the document boundaries.
Password inputs mask their text and do not copy, cut, or retain undo history.

Single-line inputs leave Enter available for the window's default button.
Multiline inputs insert newlines. Tab normally changes focus; with `AcceptTab`,
it inserts a tab or indents selected lines, and Shift+Tab removes indentation.
Tabs display at four-column stops. Wheel scrolling moves multiline text
vertically; Shift+wheel moves horizontally. Caret movement and selection dragging
scroll text into view. Multiline controls show vertical and horizontal scrollbars
when needed; drag a thumb or click its track to page. Lines remain unwrapped.

Use `editor.FindText(query, start, backward)` to select and reveal a literal,
case-insensitive match, wrapping through the document. `start` is a rune offset.
It leaves keyboard focus in the search box. A searchable window opens its search
box with its magnifier or Ctrl/Command+F. Connect `OnSearch` for query changes and
`OnSearchNext` for Enter/Shift+Enter in search or F3/Shift+F3 while editing.

`EventInputChanged` and `TextPtr` receive user edits, including undo/redo.
Assigning a different `Text` value directly starts a fresh undo history.
Undo history is bounded to 100 snapshots and 2 MiB per undo/redo stack.
`ExternalTextEditing` reserves keyboard changes for the application, which is
useful for chat input with completion and history. The standalone example includes
both ordinary and multiline inputs. File saving, syntax highlighting, soft
wrapping, and IME composition UI are outside this control's current scope.

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

This copies this directory and the shared `internal/inputkeys` package into a
temporary Go module, resolves dependencies, runs tests, and builds the example. Use Go 1.27.1; desktop
builds disable Cgo. Tests need a display; on headless
Linux run the command with `xvfb-run -a`.

To publish a separate repository, copy this directory, the shared
`internal/inputkeys` package, and the root MIT `LICENSE`. Create `go.mod` with
the chosen repository module path, and update the `gothoom/` import prefixes
throughout the copied packages. Run `go mod tidy`, tests,
and the example. The extraction script records the currently tested dependency
versions. No separate repository, branch, tag, or release is created here.
