# Bard file compatibility plan

## Intended behavior

Use `.gttune` for goThoom's editable songs and arrangements. Keep the current
UTF-8 text format: Clan Lord notation, ordinary comments, and `<@...>` metadata
for titles, composers, tags, instruments, and simultaneous named parts.

Continue reading existing goThoom `.tune` and `.txt` files in place. New imports
become separate `.gttune` files. Never rename, overwrite, or delete the original
as part of import. File extensions narrow the picker; the contents determine
which importer can read a file. Do not advertise native mTooth or Tune Helper
support until real fixtures pass.

A musical part is one performer's voice. A classic `/part` command is one
consecutive transmission segment of that voice. Import and export must preserve
that distinction, including loops that span transmission segments.

## Phase 1: Native extension and existing text files

- [x] Add one shared extension policy for `.gttune`, `.tune`, and `.txt`, ignoring
  case. Default new songs and received parts to `.gttune`.
- [x] Update the library and file picker; retain explicit legacy filenames and
  continue editing existing files without automatic migration.
- [x] Import supported notation text into a new `.gttune`. Preserve comments,
  song metadata, named parts, and instruments, including old companion JSON
  instrument preferences. Reject unsupported content before creating a file.
- [x] Install the bundled trio as `.gttune` on fresh installations. Respect
  existing legacy copies, edits, case variants, and records of deleted copies;
  changing extensions must not install a duplicate or resurrect a deleted song.
- [x] Update the built-in help and file-format documentation.
- [x] Test native/legacy discovery, filename collisions, import fidelity,
  unchanged originals, and bundled-tune upgrade behavior.

Completion: users can create, receive, import, edit, preview, and play native
files while their existing library still works. No new notation dialect or
format-version marker is needed for this extension-only change.

## Phase 2: Plain-text interchange

- [ ] Add Export Part as plain Clan Lord `.txt` and Copy Classic Macro actions.
  Choose a named part explicitly for arrangements; never flatten simultaneous
  voices into one sequential song.
- [ ] Export metadata as ordinary comments and preserve instrument information.
  Use the existing token-aware command splitting and five-command validation
  for classic macro output. Keep player names out of portable song files;
  live partner configuration remains separate.
- [ ] Add import for bare `/use` command sequences, including `/tempo` and
  `/part`. Preserve command order, tempo behavior, octave marks, rests, and
  constructs crossing segment boundaries. Do not treat `/c` as a command.
- [ ] Read a bounded subset of static music macros without executing them.
  Reject variable expansion, conditional or computed song construction, and
  unrelated commands with a useful diagnostic rather than guessing.
- [ ] Show an import summary with detected format, title, voices, instruments,
  warnings, and destination. Let the user resolve missing instruments and
  ambiguous sections before saving.
- [ ] Test round trips by comparing each part's notes, timing, and instruments,
  as well as textual preservation where no conversion is needed.

Completion: a solo or each voice of an arrangement can move between goThoom and
classic text/macro workflows without changing the performed music.

## Phase 3: mTooth and older project files

- [ ] Obtain representative, redistributable mTooth native files and text
  exports, plus any available CL Tune Helper files. Record their provenance,
  encoding, format identifiers, and expected music. Confirm actual extensions
  from samples instead of inventing them.
- [ ] Detect mTooth export sections so notation, macro, and sheet-music copies
  of the same song are not imported three times. Preserve useful comments and
  measure markers; offer a choice when multiple distinct songs are present.
- [ ] Implement native project readers only for verified structures. Preserve
  recoverable title, instrument, voices, and music; explain unsupported options
  in the import summary. Never follow or open external MIDI paths automatically.
- [ ] Add explicit legacy text decoding when required by fixtures (for example,
  MacRoman). Show the detected/selected encoding and write the converted copy
  as UTF-8. Continue accepting CR, LF, and CRLF line endings.
- [ ] Add an All files picker option with content detection and clear unknown
  format errors. Maintain file-size, nesting, expansion, and voice-count limits.
- [ ] Document exactly which formats/variants have fixture coverage. Keep
  unsupported formats clearly distinguished from malformed supported files.

Completion: supported legacy files import with reviewable results and no
silent loss of voices, tempo changes, or source data.

## Later work

Direct MIDI import is separate: it needs channel-to-part mapping, quantization,
range/polyphony checks, and explicit reporting of musical compromises. Native
`.gttune` support does not imply MIDI conversion. Add a format identifier/version
only when a future content change needs one, with old-reader behavior specified.

## Validation and delivery

Run focused tests for each phase and the full Go suite for changes to parsing,
file handling, or shared behavior. Use existing classic music reference tests
for notation semantics. Verify `git diff --check` and inspect new import/export
UI at normal and narrow widths. Do not start playback as part of import.
Commit and push only when requested. Update the phase checkboxes and record
remaining work here as implementation proceeds.

## References

- [Clan Lord notation](https://www.donaldsonworkshop.com/baraboo/cltf.html)
- [mTooth User's Guide](https://www.donaldsonworkshop.com/baraboo/mToothHelp.html):
  distinguishes native documents from exports containing several CL formats.
- [Bards' Guild instrument commands](https://clanlordbard.org/instruments.html)
- [Published classic song example](https://clanlordbard.org/library.html)

## Progress

Phase 1 is implemented. New songs and received parts default to `.gttune`;
existing `.tune` and `.txt` files remain readable and editable. Notation imports
create native copies and carry resolved instruments into metadata. Fresh
bundled-song installs use the native extension while retaining the legacy
install-history key to honor prior edits and deletions.

Validation: focused Bard tests and `CGO_ENABLED=0 go test ./...` pass;
`git diff --check` passes. New regression cases cover mixed libraries,
case-insensitive collisions, unchanged import sources and sidecars, inherited
instruments, unsupported content, and native/legacy bundled-song histories.

Phases 2 and 3 are not implemented. Next: per-part plain-text export and classic
macro output, followed by command-sequence import. Native mTooth and Tune Helper
readers still require verified sample files.
