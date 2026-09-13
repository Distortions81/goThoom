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
