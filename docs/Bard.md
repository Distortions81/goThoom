# Bard tunes

Open **Tools → Bard** to manage tunes shared by all your characters.

Choose **New** to name a tune, or **Import** to copy an existing `.tune` or
`.txt` file into the library. **Open Folder** opens `Tunes/` in your user data
folder. Files contain UTF-8 Clan Lord Tune Format text. The filename supplies
the title; the preferred instrument is saved in a companion `.json` file.
Use **Refresh** after changing files outside the client, and the titlebar
magnifier to search tune names.

Select a tune and choose its instrument. Carried instruments are marked
**inventory**; other instruments are available for local preview. Take an
instrument out of its case in-game to make it available for performance.
The picker follows the selected character's inventory.

**Edit** opens the text editor with syntax colors, search, undo/redo, and
**Check** for notation and instrument compatibility. The editor's **Preview**
plays the current draft without saving it. **Save** keeps your changes.
Font size and colors follow the shared editor settings.

The Bard window's **Preview** plays the saved file locally. **Stop Preview**
stops that preview. Music must be enabled and audible in the Audio mixer.

**Play in Game** equips the chosen instrument, waits for the server's equipment
confirmation, and performs the saved tune in the selected character's session.
Long tunes are split into up to five commands automatically, keeping musical
tokens intact. If a tune exceeds that limit, shorten it using loops.
**Stop Playing** cancels unsent parts and sends the game's stop command.
Closing the Bard window or switching session tabs also stops its performance.
The instrument remains equipped afterward.

## Writing a tune

Write notation directly, without `/use` commands or a macro wrapper:

```text
<A short practice tune>
@120 (cdef gab/c)2
```

- `a` through `g` are eighth notes; uppercase letters are quarter notes.
- A digit from `1` through `9` overrides a note's length in sixteenth notes.
- `p` is a rest, `#` is sharp, and `.` is flat.
- `/` selects the high octave, `\` the low octave, and `=` resets it.
- Parentheses repeat a phrase; `(cde)2` plays it twice.
- `[ceg]` adds a chord on instruments that support chords.
- `@120` sets the tempo. Put titles and other prose inside `<comments>`.

Comments may contain Unicode text. They are kept in the file and omitted from
performance commands. MIDI and multi-instrument project files must be converted
to plain CL tune notation before importing.
