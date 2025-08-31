# goThoom

An open-source (MIT) client for the classic **[Clan Lord](https://www.deltatao.com/clanlord/)** MMORPG

<img width="125" height="125" alt="goThoom" src="https://github.com/user-attachments/assets/b036f99a-668b-408e-8a43-524a0659a260" />
<img src="dev-screenshots/Screenshot_20250829_132759.png"/>

> Status: actively developed, cross-platform builds provided in Releases.

---

## Why this exists

- The original client is old and finicky.
- Single binary, fast rendering (OpenGL), and fewer weird dependencies.
- Higher framerates (linear interpolation), de-dithering of old graphics, multi-platform.

---

## Download

**Easiest:** grab the latest build from **Releases** on this repo (Windows, macOS, Linux).

If you build from source, you'll need Go **1.24+** and OpenGL/X11 dev libs on Linux. See "Build from source."

---

## Quick start (players)

1. **Download** a release for your OS and unzip it.  
2. **Run** the app:
   - **Windows/macOS:** launch the executable/app bundle.
   - **Linux:** `./gothoom` (you may need `chmod +x gothoom`).
3. On first run, the client **auto-fetches missing game assets** (images, sounds) into `data/`. No manual wrangling.

### Optional extras
- Drop a `background.png` and/or `splash.png` into `data/` for a custom look.

### Text-to-speech voices
Piper voices are stored in `data/piper/voices`. The client and `scripts/download_piper.sh` support voice archives in `.tar.gz` format and automatically extract and remove the archives. If a voice archive isn't available, the script falls back to downloading raw `.onnx` models with matching `.onnx.json` configs.

---

## Using the UI

- Windows: Click the `Windows` toolbar button to toggle common panels: Players, Inventory, Chat, Console, Help, Hotkeys, Macros, Mixer, Settings, and more. Window layout and open/closed state persist between runs.
- Actions: Use the `Actions` toolbar drop-down for Hotkeys, Macros, Triggers, or Plugins. Dedicated buttons provide quick access to Settings, Help, Snapshot, Mixer, and Exit.
- Movement: Left-click to walk, or use WASD/arrow keys (hold Shift to run). An optional "Click-to-Toggle Walk" sets a target with one click.
- Input bar: Press Enter to type; press Enter again to send. Esc cancels. Up/Down browse history. While typing, Ctrl-V pastes and Ctrl-C copies the whole line. Right-click the input bar for Paste / Copy Line / Clear Line (Paste and Clear switch to typing mode and refresh immediately).
- Chat/Console: Chat and Console are separate windows by default. Right-click any chat or console line to copy it; the line briefly highlights. You can merge chat into the console in Settings.
- Inventory: Single-click selects. Double-click equips/unequips; Shift + double-click uses. Right-click an item for a context menu: Equip/Unequip, Examine, Show, Drop, Drop (Mine). If a shortcut is assigned to an item, its key appears like `[Q]` before the name.
- Players: Single-click selects a player. Right-click a name for Thank, Curse, Anon Thank…, Anon Curse…, Share, Unshare, Info, Pull, or Push. Tags in the list: `>` sharing, `<` sharee, `*` same clan.
- Mixer: Adjust Main/Game/Music/TTS volumes and enable/disable channels.
- Quality: Pick a preset, or tweak motion smoothing, denoising, blending.

Tip: The input bar auto-expands as you type and has a context menu for quick paste/copy/clear.

---

## In‑game Commands (quick reference)

Click and drag to move. Type \HELP <COMMAND>. The commands are: \ACTION, \AFFILIATIONS, \ANONCURSE, \ANONTHANK, \BAG, \BOOT, \BUG, \BUY, \CURSE, \DEPART, \DROP, \EQUIP, \EXAMINE, \GIVE, \HELP, \INFO, \KARMA, \MONEY, \NAME, \NARRATE, \NEWS, \OPTIONS, \PONDER, \POSE, \PRAY, \PULL, \PUSH, \REPORT, \SELL, \SHARE, \SHOW, \SKY, \SLEEP, \SPEAK, \STATUS, \SWEAR, \THANK, \THINK, \THINKCLAN, \THINKGROUP, \THINKTO, \TIP, \UNEQUIP, \UNSHARE, \USE, \USEITEM, \WHISPER, \WHO, \WHOCLAN, \YELL

Full command help (arguments and examples):
- Local: docs/CommandsHelp.md
- External (very detailed): https://clump.clanlord.net/library/index.php?title=Command_Reference

---

## Power-user tricks

You can run the client with flags:

- `-clmov` - play a recorded `.clMov` movie file
- `-pcap`  - replay network frames from a `.pcap/.pcapng` (good for testing UI/parse)  
- `-pgo`   - create `default.pgo` by playing `test.clMov` at 30fps for 30s  
- `-debug` - verbose logging
- `-dumpMusic` - save played music as WAV
- `-imgDump` - dump loaded images as PNG to `dump/img`
- `-sndDump` - dump loaded sounds as WAV to `dump/snd`

Examples:
```bash
# Replay a capture to kick the tires
go run . -pcap reference-client.pcapng
```

---

## Plugins

goThoom can load optional plugins at startup using [yaegi](https://github.com/traefik/yaegi), a Go interpreter.
Place `.go` files inside the `plugins/` directory. Each plugin is evaluated and may
define an `Init()` function that runs after client initialization. Every plugin
must declare a unique `PluginName` string and can classify itself with optional
`PluginCategory` and `PluginSubCategory` fields. Suggested categories include
"Quality Of Life", "Profession", "Fun", and "Equipment". For example,
`PluginCategory = "Profession"` with `PluginSubCategory = "Healer"`.

Plugins only see a small, approved API exposed through the `gt` package:

```go
import "gt"

func Init() {
    gt.Logf("plugin active")
    gt.AddHotkey("ctrl+h", "/hello")
    _ = gt.ClientVersion
}
```

Currently exposed symbols:

- `gt.Logf(format, ...any)` – write to the client log
- `gt.AddHotkey(combo, command)` – bind a hotkey to a slash command
- `gt.RegisterCommand(name, func(args string))` – handle a local slash command
- `gt.RunCommand(cmd)` – echo and send a command immediately
- `gt.EnqueueCommand(cmd)` – queue a command silently for the next tick
- `gt.ClientVersion` – current client version (read/write)

Hotkey command strings may include `@`, which expands to the name of the last
right-clicked mobile.

All plugin code runs in the same process but is sandboxed to this approved list of
functions and variables.

---

## Build from source (devs)

### Linux (Debian/Ubuntu)
```bash
sudo apt-get update
sudo apt-get install -y golang-go build-essential libgl1-mesa-dev libglu1-mesa-dev xorg-dev
go build
./gothoom
```
Requirements: Go **1.24+**, OpenGL + X11 development libraries.

### Dependency bundle for faster setup
To prefetch all Debian packages and Go modules into a single archive for
offline installs, run:
```bash
scripts/build_dep_bundle.sh
```
Unpack `gothoom_deps.tar.gz` on another machine and install the `.deb`
files from `apt/`. Set `GOMODCACHE` to the extracted `go/mod` directory to
reuse the module cache when running `go mod download`.

### Cross-platform release builds
A helper script builds **Linux + Windows** binaries (and can sign Windows EXEs and macOS `.app` bundles). On Ubuntu it will install missing tools like `zip`/`osslsigncode` automatically. Set cert env vars, then run:
```bash
export WINDOWS_CERT_FILE=certs/fullchain.pem
export WINDOWS_KEY_FILE=certs/privkey.pem   # optional: WINDOWS_KEY_PASS, WINDOWS_CERT_NAME, WINDOWS_TIMESTAMP_URL
# macOS signing (defaults: ad-hoc identity, repo entitlements)
export MAC_SIGN_IDENTITY="-"                # '-' for ad-hoc; set to your certificate name to sign
export MAC_ENTITLEMENTS=scripts/goThoom.entitlements  # override for custom entitlements
scripts/build_binaries.sh
```

The script uses [rcodesign](https://gregoryszorc.com/projects/apple-codesign/) to ad-hoc sign macOS `.app` bundles when available. Install it on Linux with:

```bash
curl -L -o rcodesign.tar.gz https://gregoryszorc.com/projects/apple-codesign/releases/latest/linux-x86_64.tar.gz
tar -xf rcodesign.tar.gz && sudo mv rcodesign /usr/local/bin/
```

This helper uses [`go-winres`](https://github.com/tc-hib/go-winres) to embed
`goThoom.png` as the Windows executable icon. To build manually with the icon:

```bash
go install github.com/tc-hib/go-winres@latest   # once
go-winres simply --icon goThoom.png --arch amd64 --manifest gui
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o gothoom.exe
```

`MAC_SIGN_IDENTITY` uses `-` by default for ad-hoc signatures. Set it to
your certificate name to sign with a real identity. `MAC_ENTITLEMENTS`
defaults to `scripts/goThoom.entitlements`; point it elsewhere (or to
`/dev/null`) to use custom entitlements.

---

## Troubleshooting

- **Missing assets**: the client will fetch `CL_Images` / `CL_Sounds` archives on first run. If you interrupted it, delete partial files in `data/` and relaunch.
- **Linux can’t start**: ensure OpenGL and X11 dev libs are installed (see commands above).
- **Weird graphics/audio**: try `-debug` to see logs, and file an issue with your OS/GPU/driver info.

---

## Contributing

PRs welcome. Keep changes focused and testable. If you’re adding protocol or UI tweaks, include a small `.pcap` or `.clMov` so others can reproduce quickly. The repo includes tests for text parsing, sound, synthesis, and more—use them.

---

## License

MIT. Game assets and “Clan Lord” are property of their respective owners; this project ships **a client**, not server content.

---

## Credits

Built in Go with a sprinkle of pragmatism and a lot of late-night packet spelunking. If you enjoy this, star the repo or link it.
