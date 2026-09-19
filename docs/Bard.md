# Bard tunes

Open **Tools → Bard Tools** to manage tunes shared by all your characters.
Choose **Help** for an offline, topic-by-topic guide to the controls, tune
notation, instruments, group performances, sharing parts, and troubleshooting.

**Three Lanterns** is an included original trio for Pine Flute, Starbuck Harp,
and Gutbucket Bass. It lasts about 48 seconds. The bass starts alone, the harp
joins after two seconds, and the flute after four; all three finish together.
Use **Preview All** to hear the arrangement, or select one part on each of three
characters to test an in-game trio. The numbered comments mark two-second bars.
The tune is added to `Tunes/` on startup once. Your edits are preserved, and
deleting it keeps it out of the library on later starts.

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
Choose its instrument, or open **Duet / Trio…** and choose **Your part** in an
ensemble arrangement. The main window shows the selected part.
Changing the instrument saves that part's choice inside the tune
file. Save or close an unsaved editor draft before using the instrument picker.
Carried instruments are marked **inventory**. When you carry an instrument case,
other instruments are marked **try case**: Play in Game will attempt to retrieve
the selected instrument. The case's contents are not known until retrieval
succeeds. Without a case, instruments you are not carrying are available for
local preview only. The picker follows the selected character's inventory.

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
part, instrument, and any other performers. Confirm to retrieve and equip the
instrument and perform your selected part. If your inventory is full when an
instrument needs to come out of the case, the client puts one carried instrument
away first, preferring an unequipped one. If none can be stored, free a slot and
try again. The client waits for the server to confirm each transfer and equipment
change before sending music.
Each performer's music is split into up to five commands automatically, keeping
musical tokens intact. If a part exceeds that limit, shorten it using loops.
**Stop Playing** cancels unsent parts and sends the game's stop command.
Switching session tabs keeps each character's performance running.
**Stop Playing** applies to the selected character; closing Bard stops all
performances started through Bard Tools.
The instrument remains equipped afterward.

**Put All Instruments Away** stops the selected character's performance and
returns all carried instruments to their case, one at a time. The previous
left-hand item is restored after successful case use. **Stop Playing** cancels
that character's remaining transfers; closing Bard cancels transfers for all
characters. Completed transfers stay in place. Switching tabs keeps transfers
running on their original character. If the case does not accept or produce an
instrument, the operation stops and the status bar asks you to check the game's
response.

With several session tabs, Bard follows the selected character and remembers
each session's song, part, partners, and receiving choice while the tool is
open. To play a duet or trio with your own characters, choose and start each
character's part using the normal controls, then switch to the next tab. Each
character still lists the other performers in **Play with**. Local preview
stops when you switch tabs.

## Duos, trios, and larger arrangements

Put each performer's music after a named `<@part: name>` line, followed by its
`<@instrument: name>`. Parts play together from the same starting point. Use rests
for delayed entrances; a new part does not continue the previous part's timing.
Each part has its own tempo, defaulting to 120 unless its notation changes it.

```text
<@title: Moonlight>
<@composer: Someone>
<@tags: duet, quiet>

<@part: Melody>
<@instrument: Pine Flute>
@90 c4e4g8

<@part: Accompaniment>
<@instrument: Lucky Lyra>
@90 c8g8
```

Open **Duet / Trio…** to set up an ensemble. Each performer selects **Your part**
there and enters the other performers in **Play with**, then returns to the main
Bard window to choose **Play in Game**. Use full names when sharing parts; unique name prefixes
also work for playback. A trio with Blue and Pixy
would enter `Blue, Pixy`; Blue and Pixy each enter the other two performers.
Use **Choose players…** to select visible players, listed nearest first.
Check up to two partners, then choose **OK** to apply the names or **Cancel**
to keep your previous choices. You can also enter names separated by commas.
As you type a name, a gray suggestion can appear from recent players in the
selected session. Press **Tab** to accept it, then add a comma for another name.
Leave the field empty to perform your selected part alone. The partner list also
applies when your tune contains only one part, including a received part.

In **Duet / Trio**, choose the part to send beside each partner's name, then
choose **Send Parts**. **Don't send** skips that partner. Parts travel in private
sunstone messages; equip working sunstones. Each message contains plain Clan
Lord tune notation with a `<...>` comment naming the song, instrument,
and segment number, such as **1 of 3**, **2 of 3**, and **3 of 3**. For example:
`<1 of 3 | Moonlight | Pine Flute | 4Au>`.
The final three-character Base58 CRC-16 checksum groups segments of the same
part and checks that the assembled music is intact.

Players using another client can copy the message bodies in numbered order,
including the comments, into their tune editor or macro. Omit the chat log's
sender names and timestamps. The comments are valid Clan Lord tune comments;
no decoding or goThoom installation is needed. Individual notes and notation
tokens stay intact at message boundaries. Equip the instrument named in the
comments and use your client's normal music playback commands.

Messages use up to 180 wire bytes including the comment, keeping segments short
enough for private sunstone delivery. The available space for notes
varies with the length of the song name.

In goThoom, receiving assembles and saves the part automatically.
**Cancel Sending** cancels the selected character's unsent messages.
Switching tabs keeps sending on the original character; closing Bard cancels
unsent messages for every character.
A sent message does not confirm that the other player received it.

**Receive parts from partners** is on when you first open **Duet / Trio…**.
It saves and selects parts sent by the full names in your **Play with** list.
Turn it off to stop accepting parts. Use **Choose players…**
or enter their full names; a name prefix does not grant permission to send you
parts. Only private thoughts are accepted, and blocked or ignored players are
excluded. Each complete, valid part becomes a new tune, preserving its song
name, instrument, and notes. Existing tunes and editor drafts are
left intact. Receiving does not equip instruments or start playback: review the
part and choose **Play in Game** when ready.

Receiving is separate for each session and stays enabled when you switch tabs
or close Duet / Trio. A received part is selected only for its recipient; it
does not change another character's selection. Reconnecting that character or
closing Bard turns receiving off.
Incomplete transfers expire after two minutes; ask your partner to resend them.
Each part is limited to 8 KiB and 64 segments, with one incomplete transfer per
partner. Receiving pauses before saving more than 16 parts; enable it again in
**Duet / Trio** when you are ready for more.

The client adds Clan Lord's `/with` options to the music commands so playback
waits for the other performers. Performer names stay out of the tune file.
**Stop Playing** remains available while an ensemble is waiting or playing;
choose it to end your participation. The game supports up to two other
performers in one synchronized group, as described in the
[Bards' Guild instrument guide](https://clanlordbard.org/instruments.html).
Files and local previews can contain more parts.

## Song details and comments

Use `<...>` for comments, including comments that span multiple lines. Put
structured metadata on its own line as `<@name: value>`:

- `<@title: name>` supplies the library title; without it, the filename is used.
- `<@composer: name>` supplies the composer.
- `<@tags: tag, tag>` supplies comma-separated tags, matched without regard to case.
- `<@instrument: name>` names the instrument. Before any parts, it provides the
  default; inside a part, it applies only to that performer.
- `<@part: name>` begins a performer's music and ends the preceding part. Names must
  be nonempty and unique within the file.

Put title, composer, and tags before the first part. A file without part markers
is a solo tune. Check reports misspelled or unknown metadata keys. Ordinary
comments can contain any text, including Unicode, and are never sent to the game.
Comments can be nested; semicolons inside them are ordinary comment text.
In metadata values, write `&lt;`, `&gt;`, and `&amp;` for literal `<`, `>`, and `&`.
Other clients treat these metadata lines as ordinary Clan Lord comments. Copy
one performer's part at a time; the part markers do not arrange simultaneous
playback in other clients.

Existing `;` comments and `;@name: value` metadata remain supported. A `;`
comment runs to the end of the line.

Older tune files without instrument metadata still read their companion
`.json` preference. If neither is present, the default is Lucky Lyra. An explicit
instrument in the tune file takes precedence.

## Writing a tune

Write notation directly, without `/use` commands or a macro wrapper:

```text
<@title: A short practice tune>
<@instrument: Pine Flute>
<Start gently.>
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
