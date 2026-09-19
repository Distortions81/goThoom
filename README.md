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

<img src="dev-screenshots/Pebble Pockets__2026-09-07-15-05-49.png" alt="goThoom game client" />

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
**Settings → Files → Open User Data Folder**. Files from older portable
installations can be copied there manually when needed.

Diagnostics are recorded in the user data folder's `Diagnostics` directory
when goThoom has an event to report; a normal start and exit do not create a
log. Use **Settings → Files → Open Diagnostics Folder** to find the current
`goThoom.log` and its five rotated backups when reporting a problem.
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

- **Movement:** Left-click in the game view to walk toward the cursor. Keyboard
  movement, its run key, fullscreen, the command palette, and session switching
  can all be rebound or disabled under **Actions → Hotkeys**.
- **Chat:** The input bar is open by default: type and press Enter to send.
  **Input bar always open** keeps it available while using other controls and
  after sending. When that setting is off, Escape closes it and Enter reopens
  it. Up and Down browse message history.
  Use `:smile:`, `:thumbs_up:`, or pasted emoji. Emoji travel as readable names
  and, when emoji display is enabled, appear in color in chat and speech bubbles.
  The command icon at the right of the input bar lists available client,
  script, macro, and server commands by source, with a short explanation of
  each. It also lists input-bar completion, history, and editing shortcuts in
  a popup that grows to fit the display and wraps long help; choose a command
  to start it in your draft. Emoji display is off by default.
  When enabled, its picker appears
  beside the command icon with groups on the left; choosing an emoji appends
  its name to your draft and closes the picker.
  Standard Ctrl editing shortcuts work on Windows and Linux; use Command on
  macOS for Select All, Cut, Copy, and Paste.
- **Windows:** Use **Settings → Display → Window Layout** to arrange the game,
  Players, Inventory, Chat, and Console panes. The **Windows** toolbar button
  opens the layout editor directly in tiled mode. In floating mode, use the
  **Windows** toolbar button or **Settings → Display → Show / Hide Windows** to
  open or close individual panes; these four panes omit title-bar close buttons
  to prevent accidental closure. The **Actions** toolbar menu opens
  Hotkeys, Shortcuts, scripts, macros, and saved data.
- **UI scale:** The base UI scale moves in 0.1 steps so window and control
  geometry remains stable. Retina and HiDPI display scaling is applied on top.
- **Multiple sessions:** Each character connection has a tab above the one game
  view. Use **+** to open another tab, up to ten. The main tab's **X** disconnects
  and returns it to login. Other tabs close with confirmation and disconnect
  that session; the last remaining tab stays open.
  The selected tab owns rendering, input, shared panels, and audio while every
  other session stays connected and keeps processing updates. See the
  [multi-session guide](docs/MultiSession.md).
- **Inventory:** Click to select, double-click to equip or unequip, and
  Shift-double-click to use. Right-click for more actions.
- **Players:** Right-click a player for common actions such as Thank, Share,
  Info, Pull, and Push.
- **Copying text:** Right-click a chat or console line to copy it. The input bar
  also has a right-click menu for paste, copy, and clear. Use Command+C/V on
  macOS or Ctrl+C/V on Windows and Linux. Selected text takes priority over
  copying the entire input bar.
- **Audio:** Use the Mixer to control game, music, speech, and notification
  volume independently.
- **Snapshots:** Click **Tools → Snap** to name a capture, choose the game view or entire
  client window, optionally hide name tags, choose whether to skip speech bubbles,
  and save as PNG or JPEG. The options
  window hides before capture; name-tag settings return to normal afterward.
  The suggested filename starts with the current character's name and a timestamp.
  Files go into the user data folder's `Screenshots` directory; **Open Folder**
  opens that location. Duplicate names receive a number.

The toolbar **Tools** menu opens the full online user manual, and **Record**
starts or stops a session recording. A command reference is available
in [docs/CommandsHelp.md](docs/CommandsHelp.md). Persistent options without a
dedicated control, automatically managed settings, and session-only controls
are inventoried in [docs/Settings.md](docs/Settings.md).

### Mac keyboard

Legacy macros use `command` for Command (⌘), `option` for Option (⌥), and
`control` for Control. Hotkeys and Go script bindings use `Meta`, `Alt`, and
`Ctrl`; `Command` and `Cmd` are also accepted there. The keyboard tester labels
Command and Option **Cmd** and **Opt** on Mac.
Existing Control shortcuts still work.

| Action | Mac shortcut |
| --- | --- |
| Copy / paste | Command+C / Command+V |
| Open command palette | Command+Shift+P (rebindable in Hotkeys) |
| Move by word in the game input | Option+Left / Option+Right |
| Move to the beginning / end of the game input | Command+Left / Command+Right |
| Delete the preceding word | Option+Delete (Backspace) |
| Delete back to the beginning of the game input | Command+Delete (Backspace) |

### Linux file dialogs

Opening movie files and choosing storage folders requires Zenity or Qarma.
On Debian or Ubuntu, install Zenity with `sudo apt install zenity`.
Windows and macOS use native dialogs without an extra installation.
## Downloads and customization

Open **Download Files** in the client to install optional extras:

- A SoundFont for higher-quality music.
- Piper voices for local text-to-speech.

For bard-instrument comparison, the repository includes a
[23-instrument MIDI audition](bard-instrument-audition.mid) and the matching
[goThoom SoundFont render](bard-instrument-audition.wav). A
[QuickTime Musical Instruments render](bard-instrument-audition-quicktime.wav)
of the same MIDI provides a classic-client reference. The
[side-by-side stereo comparison](bard-instrument-compare-LR.wav) makes the two
renders easy to compare directly. Each instrument has the same four-second
slot in all files, in instrument-index order from 0 through 22.

You can also customize goThoom without modifying the program:

- Place `background.png` in the user data folder to use a custom background.
- Use **Settings → Display → Edit Color Theme / Edit Style Theme** to edit
  the active theme. Built-in themes get an editable copy in your user data
  folder under `themes/palettes/` or `themes/styles/`. **Check** validates the
  draft; **Save & Apply** saves and selects it. **Format** tidies the JSON.
  Example files and format documentation are included in the themes folder.
- Install optional PNG or ZIP sprite packs in the user data folder's `hdimg`
  directory and enable **Settings → Experimental → Use sprite pack
  files**. Subfolders are supported. Packs are off by default; see the
  [installation guide](https://gothoom.m45sci.xyz/help/performance.html#sprite-packs).
- Enable and choose **Replacement Effects** under **Settings → Experimental**
  for animated shaders on supported effects and scenery. This is separate from
  sprite packs and also defaults off.
- Use **Potato GPU (4096px Limit)** under **Settings → Performance → Caching**
  only for GPUs with small texture limits. It changes texture allocation, not
  the artwork resolution or overall quality preset.

For original/new comparisons, live reloads, reference exports, and making your
own pack, see the [artwork authoring guide](https://gothoom.m45sci.xyz/help/artwork.html).

## Notes and pronunciation corrections

Open **Tools → Personal Notes** for global or player-specific notes. Each note
has a subject, comma-separated tags, and a plain-text body. Use the player and
scope filters to choose which notes appear, and search subjects and tags with
the titlebar magnifier. **Details** changes the subject, tags, or owner;
**Open** edits the body with search, undo/redo, and **Save**. Notes keep your
spacing and are stored locally under `Notes/` in the user data folder.
**Trash** moves a note into `Notes/Trash/`; restore it by moving its whole
`note-…` directory back into `Notes/`, then choose **Refresh**.

Use **Settings → TTS → Edit TTS corrections** to edit `tts_substitute.txt`.
Write one `original=replacement` per line; lines beginning with `#` are comments.
**Check** validates the draft. **Save** keeps it on disk; **Save & Apply** also
updates substitutions for future speech.

Go scripts, notes, TTS substitutions, and theme/style files use Unicode (UTF-8).
The editors preserve an existing UTF-8 BOM and newline style. Only legacy
macros accept MacRoman files.

## Bard tunes

Open **Tools → Bard Tools** to create, import, and edit UTF-8 tune files in `Tunes/`.
Tunes can include song details, tags, and named parts for different instruments.
Preview an ensemble together or listen to one part. **Play in Game** equips a
carried instrument, or retrieves it from your instrument case, and performs your
selected part, splitting long songs automatically. **Duet / Trio…** sets up
partners and shares assigned parts through private sunstone messages. Optional
receiving saves parts from listed partners; you choose when to play them.
Bard Tools remembers each session's selections and keeps performances running
when you switch character tabs. The editor also previews unsaved drafts and
checks every part's notation.
The included **Three Lanterns** trio provides a short practice arrangement for
Pine Flute, Starbuck Harp, and Gutbucket Bass.
See the [bard guide](docs/Bard.md) for notation and performance controls.

## Macros and scripts

Settings are shared by every character and session tab. The existing
`enabled.json` files remain the source of truth for explicit per-character
script and macro selections.

Use the editor’s gear shortcut to open **Settings → Text** and adjust
**Editor Text Size**. **Ctrl+- / Ctrl++** also resizes editor text; **Ctrl+=**
and keypad +/− also work. On Mac, Command works too. **Ctrl+scroll up/down** over
the text resizes it without scrolling. **Settings → Text → Editor Text Size**
controls the same preference, saved for all editors (default 11).

Choose **Settings → Text → Editor Colors** for syntax colors. Leave
**Use custom syntax colors** unchecked to follow the color theme, or enable it
to choose colors shared by macro, Go script, TTS, and theme/style editors. Custom choices are
saved and retained when you switch back to theme colors.

### Legacy macros

Open **Actions → Legacy Macros** to browse the macro library.
Use **New Macro** to name a new file and open it in the editor; enable it with
**Global** or **Player** when ready. Macros supplied
with the client are labeled **Included with goThoom**. Missing included macros
are added automatically, and unchanged copies update automatically. Your edits
are preserved. Enable a macro globally or for a selected character. Use **Edit**
to change its source in-game, **Save** to keep a draft on disk, or **Save & Reload**
to check it, save it, and reload the selected session's enabled macros.
The editor colors comments, strings, keywords, numbers, variables, and keybindings.
**Check** also reports lint warnings. **Format** tidies indentation and trailing
whitespace. Macros format automatically when opened and saved, when syntax
allows it; opening keeps formatting in the draft until you save.
Use **Refresh** after adding, removing, or renaming files, and **Reload Macros**
after editing an enabled macro externally.
Your own `.mac` or `.txt` files can be added to `Macros/Library/`. Enable
**Allow continuous macros** for classic macros that intentionally loop without
pausing or producing output.

Macro metadata and examples are documented in
[METADATA.md](source/testdata/legacy_macros/web/METADATA.md). Legacy text
encoding details are in
[LegacyTextCompatibility.md](docs/LegacyTextCompatibility.md). The
[macro input compatibility reference](docs/LegacyMacroInputCompatibility.md)
lists key names, aliases, and hardware limits.

### Go scripts

Open **Actions → Scripts** to configure, validate, reload, or stop scripts.
Use a script's **Edit** action for the in-game editor, with search, scrollbars,
clipboard shortcuts, and undo/redo. It colors Go comments, strings, rune literals,
keywords, and numbers. **Check** validates the draft, **Save** keeps
it on disk, and **Save & Reload** reloads that script in the selected session if
it is running. **Format** applies standard Go formatting. Scripts format when
opened and saved; opening keeps the changes in the draft until you save.
Undo restores the previous draft.
On first enable, a permission review opens with requested access checked and
unused capabilities greyed out. Choose **Grant** to accept the selected access
or **Block all** to deny it and disable the script. Commands, key bindings,
server commands, data/events, movement, UI, input, notifications, storage, and
background timers have separate controls. Use **Info → Permissions** to change
these later. Decisions are saved separately in `Scripts/permissions.json`.
Use **Info → Settings** for a script’s preferences, key bindings, and local
command names. Key bindings and command names apply across characters; use
**Apply** to save an edit or **Reset** to restore the script’s default.
Use **Player** to enable a script for the character named in the Scripts window,
or **All** to enable it for every character. Scripts run during a logged-in
session. For setup, permission choices, and examples, see the
[automation manual](https://gothoom.m45sci.xyz/help/automation.html).
Scripts may be a single `.go` file, a folder with assets, or a ZIP package.
Scripts supplied with the client are labeled **Included with goThoom**.
On startup or **Refresh**, missing included scripts are added to `Scripts` in
the user data directory, and unchanged copies update automatically. Your edits
are preserved. Adding or updating a script does not enable it.

Script-author documentation lives in
[source/script_library/README.md](source/script_library/README.md), with the complete API in
[source/gt2/API_REFERENCE.md](source/gt2/API_REFERENCE.md).

For VS Code completion and type checking, download
[`goThoom-Script-Template.zip`](https://github.com/Distortions81/goThoom/releases/latest/download/goThoom-Script-Template.zip)
from the latest release.

## License

goThoom is released under the [MIT License](LICENSE). Clan Lord and its game
assets belong to their respective owners; this repository provides a client,
not server content.

## Asset reference

For local world-building experiments, the [asset catalog](docs/assets/README.md)
documents reviewed low-numbered image IDs and a tool for exporting local previews.
Extracted game artwork is not included in the repository.

## Credits

Built in Go with a sprinkle of pragmatism and a lot of late-night packet
spelunking. If you enjoy goThoom, consider starring the repository or sharing
it with another Clan Lord player.
