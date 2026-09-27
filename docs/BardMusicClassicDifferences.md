# Bard music differences from the classic client

## Purpose

This document records known differences between goThoom's bard-music
implementation and the classic Clan Lord client. It is a reference for later
compatibility work. Bard instrument validation intentionally supports the 23
classic instruments, numbered 0 through 22.

The comparison uses classic-client commit `6ba334c` from 2023-12-24. The
classic instrument resource is in:

```text
mac_client/client/source/Resources/ClanLord.r
```

The relevant goThoom implementation is in:

```text
source/tune.go
source/classic_tune.go
source/tune_notation.go
source/synth.go
```

## Instrument program numbering

The classic resource stores conventional one-based General MIDI numbers.
goThoom preserves those resource values for instruments 0 through 22 and
derives the zero-based synthesizer program by subtracting one. A table test
covers every classic instrument.

Centaur Organ therefore uses the classic Bottle Blow mapping: resource number
77 and synthesizer program 76.

## Instrument definitions that match

Across the 23 classic instruments, goThoom matches the classic resource for:

- General MIDI program numbers;
- octave offsets;
- chord and melody velocity factors;
- melody-only versus chord-capable flags;
- long-chord support flags;
- effective chord polyphony limits; and
- the Orga Drum restriction to G and B notes in each allowed octave.

The Orga Drum G/B restriction is explicit instrument metadata and does not
depend on the selected synthesizer program.

## Chord validation

Chord validation returns a structured error code and byte position. Playback
rejects a tune with a validation error and reports the classic error message.

- A chord containing more notes than the instrument permits returns
  `invalid_chord`.
- A new note that cannot obtain a voice while earlier chord notes are still
  active returns `polyphony_overflow`.
- A chord sent to a melody-only instrument returns `invalid_chord`.
- `$` on an instrument without long-chord support returns
  `unsupported_instrument`.

Long-chord notes occupy voices until the same pitch toggles them off or the
song ends. Octave changes inside chords persist for following notes, matching
the classic parser.

## Repeated chord pitches

The parser preserves separate finite note events for repeated pitches, matching
classic `CTuneBuilder::StuffChord`. For example, `[=cc]2e` produces two C events
and one E event. Both C events retain their normal velocity and duration; a
later finite chord with the same pitch also leaves earlier finite events intact.
Duplicate entries still count toward the instrument's per-chord note limit.

Repeated sustained pitches follow the classic toggle rule: a second `$` note
of the same pitch ends the first after its full elapsed duration. A finite note
of that pitch ends the sustained note and starts a new finite event.

Playback still suppresses overlapping events of the same pitch within a part,
so preserving both events in the parser does not yet make doubled notes louder.
QuickTime's audible handling of those duplicate events has not been verified.

## Notation and timing

Playback and Bard Tools use the same notation reader. Spaces and nested
comments may occur between a note and its modifiers, inside a tempo value, or
before a chord length or loop count. For example, `c4#`, `c # 4`, and `c#4`
produce the same note. Sending a tune in multiple commands preserves these
modifiers and repeat counts.

`_` gives its own note its full written duration. Repeated pitches remain
separate events: `c_c` starts two notes, with no gap before the second. Melody
and chord timing both retain the classic 1/600-second clock.

Finite chords retain their duration even in a chord-only tune or after the last
melody note. `[ceg]` plays a chord by itself; in `[c]8e`, C lasts about 986.67 ms
at 120 BPM while E lasts about 236.67 ms. The reported part duration includes
explicit rests and any finite chord tail. Sustained `$` notes end when toggled
off or at the end of the notation's melody/rest timeline.

Melody and chord volumes independently stay between 1 and 10. At velocity 100,
`{9{c` produces velocity 10. `%` restores the line to 10; `%0` is invalid.

## Notation validation

Both incoming playback and Bard validation reject:

- absolute tempos outside 60–180, missing relative tempo values, and tempo
  changes inside chords or while chord voices are occupied;
- duplicate numbered or default loop endings;
- a sharp or flat that exceeds the allowed pitch range, even if a later
  modifier would bring it back into range;
- unknown characters, misplaced modifiers, unmatched delimiters, and
  unterminated loops, chords, or comments.

Loop endings are also checked during the classic parser's discovery pass, even
when a repeat count never selects them. That pass validates note modifiers,
chord sizes, long-chord support, and oversized tempo values without advancing
time or applying octave, volume, or tempo changes.

Relative tempo changes clamp at 60 and 180, matching classic behavior. Six
nested loops are supported. Input and expanded-notation limits also apply to
incoming music. Parse errors retain byte positions in the original notation,
including comments and spaces, rather than positions in expanded loop copies.

## Reference checks

The [classic parser fixtures](../source/testdata/music/README.md) contain 101
cases captured from the original `CTuneBuilder::BuildTune` at commit `6ba334c`.
A C++ harness recorded its QuickTime event requests without synthesizing audio.
`TestClassicParserReference` compares actual goThoom playback parsing and Bard
command round trips against those captured pitches, velocities, tick timings,
and error codes. These checks run in the normal Go test suite.

Recorded-concert checks also cover multipart notation that splits chords,
comments, modifiers, and octave instructions across messages. The five complete
trio and duo tracks checked against classic contain 3,042 matching note events.

Comparisons exclude the classic client's fixed 300-tick opening pause and its
automatic closing pause. goThoom's music stream supplies its own release tail;
those playback pauses are not included in the notation's reported duration.
These event comparisons do not establish audible equivalence with QuickTime.

## SoundFont verification

The default `soundfont.sf2` contains every corrected classic program. Sample
notes for Starbuck Harp, Conch, Centaur Organ, and Orga Drum all rendered with
nonzero output. Its Centaur presets are distinct: program 76 is Bottle Blow
and program 19 is Pipe Organ.

These checks establish preset availability and audible sample output, not
perceptual equivalence with QuickTime Musical Instruments.

## Instrument audition

Generate a MIDI reference and a WAV rendered from the same five-note sequence
for each classic instrument:

```sh
cd source
CGO_ENABLED=0 go run . \
  -exportInstrumentAudition ../bard-instrument-audition \
  -auditionSoundFont soundfont.sf2
```

This writes `bard-instrument-audition.mid` and
`bard-instrument-audition.wav`. Each instrument occupies four seconds in index
order from 0 through 22. The MIDI track also contains an instrument marker with
the classic resource number and zero-based synthesizer program at the start of
every segment. The main README links those files and a 92-second WAV rendered
from the same MIDI with QuickTime Musical Instruments for Windows.
