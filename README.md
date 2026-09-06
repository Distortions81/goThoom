# goThoom

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/Distortions81/goThoom)](https://github.com/Distortions81/goThoom/releases)

goThoom is a modern, open-source client for the classic
[Clan Lord](https://www.deltatao.com/clanlord/) MMORPG. It runs on Windows,
macOS, and Linux.

[Download the latest release](https://github.com/Distortions81/goThoom/releases/latest)
· [Website](https://gothoom.m45sci.xyz/)
· [User manual](https://gothoom.m45sci.xyz/help)
· [Video overview](https://youtu.be/MrGdcqIl3a4)

<img src="dev-screenshots/Screenshot_20260831_034828.png" alt="goThoom game client" />

## Get started

1. Download the archive for your platform from
   [Releases](https://github.com/Distortions81/goThoom/releases/latest).
2. Extract the archive anywhere you like.
3. Run goThoom. No installer is required.

Missing or outdated game assets are downloaded automatically into the user
data folder.
The first-run setup wizard previews graphics changes and recommends settings
for your computer.

The user data folder is `%LOCALAPPDATA%\goThoom` on Windows,
`$XDG_DATA_HOME/goThoom` on Linux (normally `~/.local/share/goThoom`), and
`~/Library/Containers/com.goThoom.client` on macOS. Open it from
**Settings → Files → Open User Data Folder**. Upgrading from an older Windows
or Linux release copies existing portable data there on first launch without
deleting the original files.

Diagnostics are written by default to the user data folder's `Diagnostics`
directory. Use **Settings → Files → Open Diagnostics Folder** to find the
current `goThoom.log` and its five rotated backups when reporting a problem.
Use **Settings → Files → File Paths** to place assets and audio, logs, legacy
macros, or Go scripts in alternate folders. goThoom can copy the existing files
and verifies that a new folder is readable and writable before saving it.

## Highlights

- Smooth movement, animation blending, and optional artwork processing.
- A resizable game view that stays sharp on modern displays.
- Modern game audio, MIDI playback and optional text-to-speech.
- Session recording and `.clmov` playback with speed and seeking.
- Configurable windows, controls, shortcuts, notifications, and themes.
- Built-in legacy macro support and an approachable Go scripting system.
- Native graphics backends, including DirectX on Windows and Metal on macOS.
- Installer-free release archives with persistent user data kept separately.

## Using goThoom

- **Movement:** Left-click in the game view to walk toward the cursor.
- **Chat:** The input bar is open by default: type and press Enter to send.
  Escape closes it; Enter reopens it. Up and Down browse message history.
- **Windows:** Use the **Windows** toolbar menu to open Players, Inventory,
  Chat, Console, Help, Hotkeys, Shortcuts, Mixer, Settings, and other panels.
- **Inventory:** Click to select, double-click to equip or unequip, and
  Shift-double-click to use. Right-click for more actions.
- **Players:** Right-click a player for common actions such as Thank, Share,
  Info, Pull, and Push.
- **Copying text:** Right-click a chat or console line to copy it. The input bar
  also has a right-click menu for paste, copy, and clear.
- **Audio:** Use the Mixer to control game, music, speech, and notification
  volume independently.
- **Snapshots:** Click **Snap** to name a capture, choose the game view or entire
  client window, optionally hide name tags, and save as PNG or JPEG. The options
  window hides before capture; name-tag settings return to normal afterward.
  The suggested filename starts with the current character's name and a timestamp.
  Files go into the user data folder's `Screenshots` directory; **Open Folder**
  opens that location. Duplicate names receive a number.

The toolbar **Help** button opens the full online user manual. A compact guide
remains available from **Windows → Help**, and a command reference is available
in [docs/CommandsHelp.md](docs/CommandsHelp.md). Persistent options without a
dedicated control, automatically managed settings, and session-only controls
are inventoried in [docs/Settings.md](docs/Settings.md).

## Downloads and customization

Open **Download Files** in the client to install optional extras:

- A SoundFont for higher-quality music.
- Piper voices for local text-to-speech.

You can also customize goThoom without modifying the program:

- Place `background.png` in the user data folder to use a custom background.
- Put custom color palettes in the user data folder's `themes/palettes/` and
  styles in `themes/styles/`. Example files and format documentation are
  created for you.
- Enable **Potato GPU (low VRAM)** in Settings → Graphics on devices with small
  texture limits, such as Raspberry Pi or older GPUs.

## Macros and scripts

Settings are global by default. On the Login screen, select a character, open
**Edit Character**, and enable **Keep settings separate** to give that login an
independent window layout, appearance, rendering, audio,
notifications, and related preferences. Character profiles are stored in
`profiles.json` in the user data folder; the existing `enabled.json` files
remain the source of truth for explicit per-character script and macro
selections.

### Legacy macros

Open **Actions → Legacy Macros** to browse the bundled macro library. Enable a
macro globally or for a selected character. Use **Refresh List** after adding,
removing, or renaming files, and **Reload Macros** after editing an enabled
macro. Your own `.mac` or `.txt` files can be added to `Macros/Library/`. Enable
**Allow continuous macros** for classic macros that intentionally loop without
pausing or producing output.

Macro metadata and examples are documented in
[METADATA.md](source/testdata/legacy_macros/web/METADATA.md). Legacy text
encoding details are in
[LegacyTextCompatibility.md](docs/LegacyTextCompatibility.md).

### Go scripts

Open **Actions → Scripts** to configure, validate, reload, or stop scripts.
Scripts may be a single `.go` file, a folder with assets, or a ZIP package. The
embedded examples are copied to an empty `Scripts` folder in the user data
directory and never replace existing scripts.

Script-author documentation lives in
[source/script_library/README.md](source/script_library/README.md), with the complete API in
[source/gt2/API_REFERENCE.md](source/gt2/API_REFERENCE.md).

For VS Code completion and type checking, download
[`goThoom-Script-Template.zip`](https://github.com/Distortions81/goThoom/releases/latest/download/goThoom-Script-Template.zip)
from the latest release.

## Local multiplayer testing

Run a small demo server for login, walking, and chat with
`cd source && go run ./cmd/demoserver`. See [DemoServer.md](docs/DemoServer.md)
for client setup and connecting friends.

## License

goThoom is released under the [MIT License](LICENSE). Clan Lord and its game
assets belong to their respective owners; this repository provides a client,
not server content.

## Credits

Built in Go with a sprinkle of pragmatism and a lot of late-night packet
spelunking. If you enjoy goThoom, consider starring the repository or sharing
it with another Clan Lord player.
