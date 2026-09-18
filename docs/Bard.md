# Bard tunes

Open **Tools → Bard Tools** to manage tunes shared by all your characters.

Choose **New** to name a tune, or **Import** to copy an existing `.tune` or
`.txt` file into the library. **Open Folder** opens `Tunes/` in your user data
folder. Files contain UTF-8 Clan Lord Tune Format text, with optional song
details and named instrument parts. Use **Refresh** after changing files outside
the client. The titlebar search matches titles, composers, tags, part names, and
instruments. Use **Tag** to filter the library and **Sort by** to order it by
title, composer, tags, or part count.

Use the **x** beside a tune to delete it after confirmation. This permanently
removes the tune and its instrument preference and discards any open draft.

Click a song or its radio button in the framed song list to select it. The
filled radio button and **Selected song** line show which song the controls use.
Choose its instrument, or choose **Your part** in an ensemble
arrangement. Changing the instrument saves that part's choice inside the tune
file. Save or close an unsaved editor draft before using the instrument picker.
Carried instruments are marked
**inventory**; other instruments are available for local preview. Take an
instrument out of its case in-game to make it available for performance.
The picker follows the selected character's inventory.

**Edit** opens the text editor with syntax colors, search, undo/redo, and
**Check** for notation, instrument compatibility, and the length of every part.
A successful check displays **Check passed.** Errors identify the affected part.
The fixed **Status** bar shows results in green and problems in red, without
resizing the window or text area. Hover over a long status to read it in full.
The editor's **Preview** plays all parts in the current draft without saving it.
**Save** keeps your changes.
Font size and colors follow the shared editor settings.

The Bard window's **Preview** plays a saved solo tune locally. For an ensemble,
**Preview All** mixes every part together and **Preview Part** plays only your
selected part. **Stop Preview** stops that preview. Music must be enabled and
audible in the Audio mixer.

The red **Play in Game** button opens a confirmation showing the song, character,
part, instrument, and any other performers. Confirm to equip the instrument and
perform your selected part. The client waits for the server's equipment
confirmation before sending music.
Each performer's music is split into up to five commands automatically, keeping
musical tokens intact. If a part exceeds that limit, shorten it using loops.
**Stop Playing** cancels unsent parts and sends the game's stop command.
Closing the Bard window or switching session tabs also stops its performance.
The instrument remains equipped afterward.

## Duos, trios, and larger arrangements

Put each performer's music after a named `;@part:` line, followed by its
`;@instrument:`. Parts play together from the same starting point. Use rests
for delayed entrances; a new part does not continue the previous part's timing.
Each part has its own tempo, defaulting to 120 unless its notation changes it.

```text
;@title: Moonlight
;@composer: Someone
;@tags: duet, quiet

;@part: Melody
;@instrument: Pine Flute
@90 c4e4g8

;@part: Accompaniment
;@instrument: Lucky Lyra
@90 c8g8
```

For an in-game duo or trio, each performer selects their own part, enters the
other performers in **Play with**, and chooses
**Play in Game**. Use names or unique name prefixes. A trio with Blue and Pixy
would enter `Blue, Pixy`; Blue and Pixy each enter the other two performers.
Use **Choose players…** to select visible players, listed nearest first.
Check up to two partners, then choose **OK** to apply the names or **Cancel**
to keep your previous choices. You can also enter names separated by commas.
As you type a name, a gray suggestion can appear from recent players in the
selected session. Press **Tab** to accept it, then add a comma for another name.
Leave the field empty to perform your selected part alone.

The client adds Clan Lord's `/with` options to the music commands so playback
waits for the other performers. Performer names stay out of the tune file.
**Stop Playing** remains available while an ensemble is waiting or playing;
choose it to end your participation. The game supports up to two other
performers in one synchronized group, as described in the
[Bards' Guild instrument guide](https://clanlordbard.org/instruments.html).
Files and local previews can contain more parts.

## Song details and comments

Ordinary `;` comments run to the end of the line. Put structured metadata on
its own line, starting with `;@`:

- `;@title:` supplies the library title; without it, the filename is used.
- `;@composer:` supplies the composer.
- `;@tags:` supplies comma-separated tags, matched without regard to case.
- `;@instrument:` names the instrument. Before any parts, it provides the
  default; inside a part, it applies only to that performer.
- `;@part:` begins a performer's music and ends the preceding part. Names must
  be nonempty and unique within the file.

Put title, composer, and tags before the first part. A file without part markers
is a solo tune. Check reports misspelled or unknown metadata keys. Ordinary
comments can contain any text, including Unicode, and are never sent to the game.
Existing nested `<comments>` are also supported; semicolons inside them are
ordinary comment text.

Older tune files without instrument metadata still read their companion
`.json` preference. If neither is present, the default is Lucky Lyra. An explicit
instrument in the tune file takes precedence.

## Writing a tune

Write notation directly, without `/use` commands or a macro wrapper:

```text
;@title: A short practice tune
;@instrument: Pine Flute
; Start gently.
@120 (cdef gab/c)2
```

- `a` through `g` are eighth notes; uppercase letters are quarter notes.
- A digit from `1` through `9` overrides a note's length in sixteenth notes.
- `p` is a rest, `#` is sharp, and `.` is flat.
- `/` selects the high octave, `\` the low octave, and `=` resets it.
- Parentheses repeat a phrase; `(cde)2` plays it twice.
- `[ceg]` adds a chord on instruments that support chords.
- `@120` sets the tempo.

MIDI files must be converted to CL tune notation before importing. For music
with several instruments, place each performer's notation in its own named part.
