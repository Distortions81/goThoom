# AGENTS

This repo includes a Go client under `gothoom/`. To build or run the Go program you need Go version 1.25 from the official Go distribution; avoid the system `golang-go` package.
Do not increment JSON versions in GT_Players.json or settings.json or characters.json. They will be done manually if needed.
Any functions or variables or types exposed to the plugins need to also be put empty stubs into gt so the linters do not complain for users.
Also I prefer to-the-point and simple solutions. We'll get complex if it is needed but I prefer to not over complicate things. "Keep it simple stupid"
Try to avoid completely over-thinking your replies and feel free to stop and ask questions rather than making an assumption.

## Installing dependencies

1. Install required system packages (if missing):
   ```bash
   sudo apt-get install -y build-essential libgl1-mesa-dev libglu1-mesa-dev \
     xorg-dev libxrandr-dev libasound2-dev libgtk-3-dev xdg-utils
   ```
2. Install Go 1.25:
   ```bash
   curl -LO https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
   export PATH="/usr/local/go/bin:$PATH"
   ```
3. **Always** download and extract the prebuilt dependency bundle:
   ```bash
   curl -LO https://m45sci.xyz/u/dist/goThoom/gothoom_deps.tar.gz
   tar -xzf gothoom_deps.tar.gz
   ```
   The archive, produced by `build-scripts/build_dep_bundle.sh`, contains
   resource files used by the client. Extracting it avoids fetching them
   individually.

4. Fetch Go module dependencies:
   ```bash
   cd gothoom
   go mod download
   ```



The `build-scripts` directory provides helper scripts for development.
`build-scripts/build_dep_bundle.sh` regenerates the dependency bundle,
`build-scripts/build_binaries.sh` compiles release binaries, and
`build-scripts/setup_dev_env.sh` bootstraps a development environment.
Run these scripts from the repository root.

## Adding Dependencies
- Document any required system packages here.
- Update `build-scripts/build_dep_bundle.sh` if additional data files need to
  be bundled and regenerate the archive with the script.
- After updates, regenerate the archive by running `build-scripts/build_dep_bundle.sh` from the repo root and re-share `gothoom_deps.tar.gz`.

## Build steps
1. Navigate to the `gothoom` directory if not already there:
   ```bash
   cd gothoom
   ```
2. Compile the program:
   ```bash
   go build
   ```
   This produces the executable `gothoom` in the current directory.
3. You can also run the program directly with:
   ```bash
   go run .
   ```
The module path is `gothoom` and the main package is located in this directory.

The `mac_client` directory contains a reference implementation written in C and should *never* be modified. It is only for reference!

## Quick Commands Reference

Click and drag to move. Type \HELP <COMMAND>. The commands are: \ACTION, \AFFILIATIONS, \ANONCURSE, \ANONTHANK, \BAG, \BOOT, \BUG, \BUY, \CURSE, \DEPART, \DROP, \EQUIP, \EXAMINE, \GIVE, \HELP, \INFO, \KARMA, \MONEY, \NAME, \NARRATE, \NEWS, \OPTIONS, \PONDER, \POSE, \PRAY, \PULL, \PUSH, \REPORT, \SELL, \SHARE, \SHOW, \SKY, \SLEEP, \SPEAK, \STATUS, \SWEAR, \THANK, \THINK, \THINKCLAN, \THINKGROUP, \THINKTO, \TIP, \UNEQUIP, \UNSHARE, \USE, \USEITEM, \WHISPER, \WHO, \WHOCLAN, \YELL

Running the client without a display (i.e. no `$DISPLAY` variable) will exit
with an X11 initialization error.

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
