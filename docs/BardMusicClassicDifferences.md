# Bard music differences from the classic client

## Purpose

This document records known differences between goThoom's bard-music
implementation and the classic Clan Lord client. It is a reference for later
compatibility work; it does not prescribe that every difference must be
removed. Some goThoom additions may be useful extensions, but they should be
kept separate from the classic instrument definitions and parsing rules.

The comparison uses classic-client commit `6ba334c` from 2023-12-24. The
classic instrument resource is in:

```text
mac_client/client/source/Resources/ClanLord.r
```

The relevant goThoom implementation is in:

```text
source/tune.go
source/classic_tune.go
source/synth.go
```

## MIDI program numbering

The classic resource stores the conventional one-based General MIDI number.
goThoom sends its table value directly as the zero-based value in a MIDI
Program Change message. Therefore the goThoom program should normally be one
less than the value in the classic resource.

Only classic instruments 2, 9, 11, 13, and 22 currently have the expected
zero-based program in goThoom. The other 18 program mappings differ:

| Instrument | Name | Classic resource | Expected goThoom | Current goThoom |
| ---: | --- | ---: | ---: | ---: |
| 0 | Lucky Lyra | 47 | 46 | 47 |
| 1 | Bone Flute | 73 | 72 | 73 |
| 3 | Torjo | 106 | 105 | 106 |
| 4 | Xylo | 13 | 12 | 13 |
| 5 | Gitor | 25 | 24 | 25 |
| 6 | Reed Flute | 76 | 75 | 76 |
| 7 | Temple Organ | 17 | 16 | 17 |
| 8 | Conch | 94 | 93 | 94 |
| 10 | Centaur Organ | 77 | 76 | 19 |
| 12 | Tuborn | 59 | 58 | 59 |
| 14 | Orga Drum | 117 | 116 | 117 |
| 15 | Casserole | 115 | 114 | 115 |
| 16 | Violène | 41 | 40 | 41 |
| 17 | Pine Flute | 78 | 77 | 78 |
| 18 | Groanbox | 22 | 21 | 22 |
| 19 | Gho-To | 108 | 107 | 108 |
| 20 | Mammoth Violène | 44 | 43 | 44 |
| 21 | Gutbucket Bass | 33 | 32 | 33 |

Centaur Organ is not an ordinary one-off numbering error. The classic client
uses Bottle Blow, while goThoom currently selects program 19, an organ patch.
Before changing it, compare the classic sound with the intended goThoom
SoundFont and determine whether the substitution was intentional.

## Chord polyphony

These chord-capable instruments use a different simultaneous-note limit:

| Instrument | Name | Classic | Current goThoom |
| ---: | --- | ---: | ---: |
| 2 | Starbuck Harp | 10 | 6 |
| 7 | Temple Organ | 10 | 6 |
| 8 | Conch | 1 | 6 |
| 13 | Bagpipe | 3 | 6 |
| 15 | Casserole | 4 | 6 |
| 16 | Violène | 2 | 6 |
| 19 | Gho-To | 3 | 6 |
| 20 | Mammoth Violène | 2 | 6 |

The classic resource stores polyphony 1 for Glass Jug. Glass Jug is
melody-only, so the classic client's effective chord polyphony is zero and
goThoom's value of zero has the same effect.

## Instrument definitions that match

Across the 23 classic instruments, goThoom matches the classic resource for:

- octave offsets;
- chord and melody velocity factors;
- melody-only versus chord-capable flags;
- long-chord support flags; and
- the Orga Drum restriction to G and B notes in each allowed octave.

The Orga Drum restriction is currently selected by checking for program 117.
If its MIDI program is corrected to zero-based 116, that check must also be
changed. Prefer explicit restriction metadata or a stable classic instrument
ID instead of identifying an instrument by its playback program.

## Additional goThoom instruments

The classic resource contains instruments 0 through 22. goThoom also accepts
these six indices:

| Instrument | goThoom extension |
| ---: | --- |
| 23 | Vibra Sustained |
| 24 | Church Organ |
| 25 | String Ensemble 1 |
| 26 | String Ensemble 2 |
| 27 | Choir Aahs |
| 28 | Warm Pad |

These should remain clearly identified as extensions if retained. Code that
validates an instrument number should distinguish classic protocol
compatibility from local or diagnostic playback features.

## Tune parser behavior

The instrument table is not the only compatibility difference:

- When a chord exceeds the instrument's polyphony, the classic client reports
  a polyphony or invalid-chord error. goThoom silently keeps only the notes
  that fit.
- When a melody-only instrument receives a chord, the classic client reports
  an invalid chord. goThoom silently ignores the chord.
- When `$` requests a long chord on an instrument without long-chord support,
  the classic client reports an unsupported-instrument error. goThoom plays
  the chord with an ordinary finite duration.
- goThoom's tune conversion returns only scheduled notes, so it currently has
  no path for returning these classic validation errors to the caller or the
  user.

These differences affect both audible output and error handling. Tests should
cover the returned diagnostic as well as the rendered note sequence.

## Suggested compatibility work

1. Represent classic resource numbers separately from zero-based synthesizer
   program numbers, then add a table test covering all 23 instruments.
2. Resolve Centaur Organ intentionally rather than treating it as a mechanical
   decrement.
3. Copy the classic polyphony limits and test boundary and overflow chords for
   every differing instrument.
4. Replace the Orga Drum program-number special case with explicit note
   restrictions in the instrument definition.
5. Change tune parsing to return structured compatibility errors, and cover
   melody-only chords, unsupported long chords, and polyphony overflow.
6. Decide whether instruments 23 through 28 remain supported extensions and
   keep that choice explicit in validation and documentation.
7. Compare representative rendered notes with the bundled and user-selected
   SoundFonts. Correct MIDI programs do not guarantee perceptual parity across
   different SoundFonts.
