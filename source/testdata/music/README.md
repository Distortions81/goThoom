# Classic parser fixtures

`classic_parser.json` records note events and parse errors from the original
`CTuneBuilder::BuildTune` at ClanLordClient commit `6ba334c` (2023-12-24).
Each case uses tempo 120 and velocity 100, with the indicated classic instrument.
Start and duration values are in the classic clock's 1/600-second ticks.

The capture harness compiled the original `CTuneBuilder` declaration and method
bodies from `TuneHelper_cl.h` and `TuneHelper_cl.cp`. A memory-backed `CDataHandle`
and recording implementations of `qtma_StuffNoteEvent`, `qtma_StuffXNoteEvent`,
and `qtma_StuffRestEvent` replaced QuickTime. Instrument metadata was supplied
for Lucky Lyra, Starbuck Harp, Temple Organ, and Conch. No audible synthesis was
performed. Restricted-note instruments were not used in these captures.

Recorded starts exclude the classic client's fixed 300-tick initial pause.
The trailing playback pause is not a note event. Zero-duration sustained-note
toggles are omitted. Invalid cases compare error codes, not partially emitted
notes or the classic parser's inconsistent error positions.

The main package's `TestClassicParserReference` checks actual playback parsing
against these fixtures. Bard validation and command round trips are checked
against the same independent event expectations. Do not regenerate expected
values from goThoom's parser.

To recapture the existing cases, use a ClanLordClient checkout at the commit
above, Python 3, and a C++17 compiler:

```sh
python3 source/testdata/music/capture_classic_parser.py /path/to/ClanLordClient
```

The capture script extracts the original declaration and method bodies, builds
its temporary harness, and overwrites only `classic_parser.json`. No macOS SDK
or QuickTime installation is required. To add a case, add its `instrument` and
`tune` to that JSON file and recapture. Avoid unterminated comments: the original
parser can read beyond the input buffer for those; goThoom checks them separately.
