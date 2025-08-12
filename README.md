# ClanLord Client

A pre-alpha open source (MIT) client for the Clan Lord MMORPG.

This repository hosts the Go implementation of the client. The
`old_mac_client/` directory contains a historical C implementation provided
for reference only (do not modify).

## Quick Start

### Requirements

- Go 1.24 or newer
- OpenGL and X11 development libraries

On Debian/Ubuntu:

```bash
sudo apt-get update
sudo apt-get install -y golang-go build-essential libgl1-mesa-dev libglu1-mesa-dev xorg-dev
```

### Build

From the repository root run:

```bash
go build
```

### Run

Launch the client with:

```bash
go run .
```

To build release binaries for Linux and Windows, use:

```bash
scripts/build_binaries.sh
```

## Command-line Flags

The Go client accepts the following flags:

- `-host` – server address (default `server.deltatao.com:5010`)
- `-clmov` – play back a `.clMov` movie file instead of connecting to a server
- `-pgo` – create `default.pgo` by playing `test.clMov` at 60 fps for 30 seconds
- `-client-version` – client version number (`kVersionNumber`, default `1440`)
- `-debug` – enable debug logging (default `true`)
- `-scale` – screen scale factor (default `2`)
- `-interp` – enable movement interpolation
- `-onion` – cross-fade sprite animations
- `-noFastAnimation` – draw a mobile's previous animation frame when available
- `-linear` – use linear filtering instead of nearest-neighbor rendering
- `-night` – force night level (0-100)

## Data and Logging

- The default server is `server.deltatao.com:5010`; override it with `-host`.
- Missing `CL_Images` or `CL_Sounds` archives in `data` are fetched automatically from `https://www.deltatao.com/downloads/clanlord`.
  They are saved as `CL_Images` and `CL_Sounds`.

## Window Position and Pinning

Windows and UI items can be anchored to the screen by setting their `PinTo` field.
When `PinTo` is `PIN_NONE` (the default), `Position` specifies the absolute
top-left coordinates. When pinned, `Position` is treated as an offset from the
chosen corner, edge, or screen center and the element can no longer be moved or
resized.

### Examples

```go
// Center-pinned window. Position {0,0} places it exactly in the middle.
win := eui.NewWindow()
win.PinTo = eui.PIN_MID_CENTER
win.Position = eui.Point{X: 0, Y: 0}
// Offset 100 pixels right and 50 down from center.
win.Position = eui.Point{X: 100, Y: 50}

// Unpinned window. Position is absolute screen coordinates.
free := eui.NewWindow()
free.Position = eui.Point{X: 100, Y: 50}
```

