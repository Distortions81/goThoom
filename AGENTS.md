# AGENTS

This repo includes a minimal Go client under `gothoom/`. To build or run the Go program you need Go version 1.24 or later.


## Installing dependencies

1. Install Go 1.24 or later. On Debian/Ubuntu you can run:
   ```bash
   sudo apt-get install -y golang-go build-essential libgl1-mesa-dev libglu1-mesa-dev xorg-dev
   ```
   `libgl1-mesa-dev`, `libglu1-mesa-dev`, and `xorg-dev` provide the OpenGL and X11 libraries required by Ebiten.
   On other distributions install the equivalent development packages.
2. Fetch Go module dependencies:
   ```bash
   cd gothoom
   go mod download
   ```

The `mac_client` directory contains a reference implementation written in C and should *never* be modified. It is only for reference!

## Deprecated Ebiten calls to avoid

- `op.ColorM.Scale`
- `op.ColorM.Translate`
- `op.ColorM.Rotate`
- `op.ColorM.ChangeHSV`
- `ebiten.UncappedTPS`
- `ebiten.CurrentFPS`
- `ebiten.CurrentTPS`
- `ebiten.DeviceScaleFactor`
- `ebiten.GamepadAxis`
- `ebiten.GamepadAxisNum`
- `ebiten.GamepadButtonNum`
- `ebiten.InputChars`
- `ebiten.IsScreenFilterEnabled`
- `ebiten.IsScreenTransparent`
- `ebiten.IsWindowResizable`
- `ebiten.MaxTPS`
- `ebiten.ScheduleFrame`
- `ebiten.ScreenSizeInFullscreen`
- `ebiten.SetFPSMode`
- `ebiten.SetInitFocused`
- `ebiten.SetMaxTPS`
- `ebiten.SetScreenFilterEnabled`
- `ebiten.SetScreenTransparent`
- `ebiten.SetWindowResizable`
- `ebiten.GamepadIDs`
- `(*ebiten.Image).Dispose`
- `(*ebiten.Image).ReplacePixels`
- `(*ebiten.Image).Size`
- `(*ebiten.Shader).Dispose`
- `ebiten.TouchIDs`
