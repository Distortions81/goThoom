package main

// Emulation details:
// - We mirror CTuneBuilder’s tick math: a sixteenth = floor(9000/tempo) ticks
//   (1/600s per tick). Non‑tied notes sound for ((beats-1)*sixteenth) + 90% of
//   a sixteenth; ties sound for the full duration (beats*sixteenth). All note
//   starts and durations are computed in ticks and converted to time at the end.
// - Chords are scheduled at the current melody cursor without advancing it.
//   Only rests/notes advance time. Chords require a melody timeline to exist
//   (gating), but may ring across melody rests (they’re only clipped to the end
//   of the melody timeline or at the next chord boundary for long‑chords).
// - Instrument rules are enforced (melody/chord capability, 3‑octave ranges,
//   Orga drum G/B restriction, polyphony), and volume uses linear 0–10 steps.

import (
	"strings"
	"time"
	"unicode"
)

// classicNotesFromTune parses a Clan Lord tune using logic modeled after the
// classic client's CTuneBuilder. It returns absolute-scheduled notes with
// durations in milliseconds based on the provided tempo and instrument.
// This implementation aims to be faithful enough for playback parity:
// - Digits 1..9 are beat-units; quarter note = 4 units; lowercase default 2, uppercase 4
// - Note length is ~90% of the event duration (ratio_NoteRest=90)
// - Rests (p) advance time by their duration (default 2)
// - Chords [..] may have an optional duration digit; default 4; optional '$' for long-chord
// - Ties '_' on single notes remove the inter-note gap by merging durations
// - Octave modifiers + - = / \ as in classic builder (central=0, hi=/, low=\)
// - Accidentals '#' (sharp) and '.' (flat)
// - Tempo change @ [+|-|=][value]; bare '@' resets to 120
// - Loops with alternate endings are expanded textually: (body|1end1|2end2!def)N
// - Comments <...> are ignored; nested is allowed
var strictCLTF = true

func classicNotesFromTune(tune string, inst instrument, tempo int, velocity int) []Note {
	if tempo <= 0 {
		tempo = 120
	}
	s := stripComments(tune)
	s = expandLoopsClassic(s)
	// State
	octave := 0      // central octave
	volMel10 := 10   // melody line volume 0..10
	volCh10 := 10    // chord line volume 0..10 (set inside chords)
	curMelTicks := 0 // melody timeline cursor in 1/600s ticks

	// helper: classic sixteenth-note unit in 1/600s ticks (integer trunc like classic)
	unitTicks := func() int { return int(9000.0 / float64(tempo)) }
	// convert ticks to time.Duration
	// Convert ticks -> duration with rounding (minimize cumulative drift)
	ticksToDur := func(t int) time.Duration {
		num := int64(t) * int64(time.Second)
		return time.Duration((num + 600/2) / 600)
	}

	// Output notes and helpers for long-chord sustain and chord gating
	var notes []Note
	chordIdx := []int{}   // indices of notes that came from chord line (for gating)
	activeLong := []int{} // indices of active long-chord notes
	// Tie-merge state for single-note melodies
	lastMelIdx := -1
	lastMelKey := -1
	lastMelEnd := 0

	i := 0
	getDur := func(def float64) float64 {
		if i < len(s) && s[i] >= '1' && s[i] <= '9' {
			d := float64(s[i] - '0')
			i++
			return d
		}
		return def
	}

	for i < len(s) {
		c := s[i]
		// whitespace
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		// octave
		switch c {
		case '+':
			if octave < 1 {
				octave++
			}
			i++
			continue
		case '-':
			if octave > -1 {
				octave--
			}
			i++
			continue
		case '=':
			octave = 0
			i++
			continue
		case '/':
			octave = 1
			i++
			continue
		case '\\':
			octave = -1
			i++
			continue
		}
		// tempo change
		if c == '@' {
			i++
			sign := byte(0)
			if i < len(s) && (s[i] == '+' || s[i] == '-' || s[i] == '=') {
				sign = s[i]
				i++
			}
			val := 0
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				val = val*10 + int(s[i]-'0')
				i++
			}
			if val == 0 && sign == 0 {
				tempo = 120
			} else {
				newTempo := tempo
				switch sign {
				case '+':
					newTempo += val
				case '-':
					newTempo -= val
				default:
					newTempo = val
				}
				// Clamp per CLTF spec to 60..180
				if newTempo < 60 {
					newTempo = 60
				}
				if newTempo > 180 {
					newTempo = 180
				}
				tempo = newTempo
			}
			continue
		}
		// volume modifiers (% {}): outside chords affect melody volume only
		if c == '%' || c == '{' || c == '}' {
			i++
			d := 0
			if i < len(s) && s[i] >= '1' && s[i] <= '9' {
				d = int(s[i] - '0')
				i++
			}
			switch c {
			case '%':
				if d == 0 {
					volMel10 = 10
				} else {
					volMel10 = d
				}
			case '{':
				if d == 0 {
					volMel10 -= 1
				} else {
					volMel10 -= d
				}
			case '}':
				if d == 0 {
					volMel10 += 1
				} else {
					volMel10 += d
				}
			}
			if volMel10 < 0 {
				volMel10 = 0
			}
			if volMel10 > 10 {
				volMel10 = 10
			}
			continue
		}
		// rest
		if c == 'p' {
			i++
			b := getDur(2)
			durTicks := int(b) * unitTicks()
			// rest advances melody time; clears tie context
			curMelTicks += durTicks
			lastMelIdx = -1
			lastMelKey = -1
			lastMelEnd = 0
			continue
		}
		// chord
		if c == '[' {
			i++
			ks := []int{}
			innerOct := octave
			// chord-local: allow volume controls to set chord-line volume (persistent)
			for i < len(s) && s[i] != ']' {
				if s[i] == '+' || s[i] == '-' || s[i] == '=' || s[i] == '/' || s[i] == '\\' {
					switch s[i] {
					case '+':
						if innerOct < 1 {
							innerOct++
						}
					case '-':
						if innerOct > -1 {
							innerOct--
						}
					case '=':
						innerOct = 0
					case '/':
						innerOct = 1
					case '\\':
						innerOct = -1
					}
					i++
					continue
				}
				// chord-line volume controls
				if s[i] == '%' || s[i] == '{' || s[i] == '}' {
					tok := s[i]
					i++
					d := 0
					if i < len(s) && s[i] >= '1' && s[i] <= '9' {
						d = int(s[i] - '0')
						i++
					}
					switch tok {
					case '%':
						if d == 0 {
							volCh10 = 10
						} else {
							volCh10 = d
						}
					case '{':
						if d == 0 {
							volCh10 -= 1
						} else {
							volCh10 -= d
						}
					case '}':
						if d == 0 {
							volCh10 += 1
						} else {
							volCh10 += d
						}
					}
					if volCh10 < 0 {
						volCh10 = 0
					}
					if volCh10 > 10 {
						volCh10 = 10
					}
					continue
				}
				if isNoteLetter(s[i]) {
					key, _ := parseNotePitch(s, &i, innerOct)
					if key >= 0 {
						ks = append(ks, key)
					}
					continue
				}
				// skip unknown within chord
				i++
			}
			if i < len(s) && s[i] == ']' {
				i++
			}
			b := getDur(4)
			long := false
			if i < len(s) && s[i] == '$' {
				long = true
				i++
			}
			if len(ks) > 0 {
				// Enforce instrument chord capability
				if strictCLTF && !inst.hasChords {
					// instrument cannot play chords; ignore chord event entirely
					continue
				}
				// finalize any previously active long-chord notes at this chord boundary
				if len(activeLong) > 0 {
					for _, idx := range activeLong {
						end := curMelTicks
						if end < int(notes[idx].Start.Milliseconds()) {
							end = int(notes[idx].Start.Milliseconds())
						}
						notes[idx].Duration = ticksToDur(end) - notes[idx].Start
					}
					activeLong = activeLong[:0]
				}
				// schedule chord notes at current melody time without advancing melody timeline
				u := unitTicks()
				// compute audible note duration with classic 90% rule
				noteTicks := 0
				if int(b) > 1 {
					noteTicks = (int(b)-1)*u + (u*90)/100
				} else {
					noteTicks = (u * 90) / 100
				}
				baseVel := velocity * inst.chord / 100
				// Linear mapping per classic: 0..10 => 0%..100% of base
				v := (baseVel * volCh10) / 10
				if v < 1 {
					v = 1
				} else if v > 127 {
					v = 127
				}
				// Enforce polyphony limit (considering currently sustained long-chord notes)
				remaining := inst.polyphony
				if !strictCLTF || remaining <= 0 {
					remaining = len(ks)
				} else {
					if remaining > len(ks) { /* ok */
					}
					if remaining > 0 {
						// reduce by active long-chord notes
						if len(activeLong) < remaining {
							remaining = remaining - len(activeLong)
						} else {
							remaining = 0
						}
					}
				}
				count := 0
				for _, k := range ks {
					if strictCLTF {
						key := k + inst.octave*12
						if !allowedNoteForInst(inst, key) {
							continue
						}
						// reassign key after filter
						if remaining > 0 && count >= remaining {
							break
						}
						n := Note{Key: key, Velocity: v, Start: ticksToDur(curMelTicks), Duration: ticksToDur(noteTicks)}
						notes = append(notes, n)
						chordIdx = append(chordIdx, len(notes)-1)
						if long && inst.longChord {
							activeLong = append(activeLong, len(notes)-1)
						}
						count++
					} else {
						key := k + inst.octave*12
						n := Note{Key: key, Velocity: v, Start: ticksToDur(curMelTicks), Duration: ticksToDur(noteTicks)}
						notes = append(notes, n)
						chordIdx = append(chordIdx, len(notes)-1)
						if long && inst.longChord {
							activeLong = append(activeLong, len(notes)-1)
						}
					}
				}
			}
			continue
		}
		// single note
		if isNoteLetter(c) {
			key, tied := parseNotePitch(s, &i, octave)
			b := 2.0
			if unicode.IsUpper(rune(c)) {
				b = 4.0
			}
			// optional duration override
			if i < len(s) && s[i] >= '1' && s[i] <= '9' {
				b = float64(s[i] - '0')
				i++
			}
			// schedule melody note at current time, advance melody time
			durTicks := int(b) * unitTicks()
			// compute audible note duration per classic: tied => full else add 90% of the last unit
			noteTicks := 0
			if tied && lastMelIdx >= 0 && lastMelKey == (key+inst.octave*12) && lastMelEnd == curMelTicks {
				// extend previous note fully by event duration
				notes[lastMelIdx].Duration += ticksToDur(durTicks)
				lastMelEnd = curMelTicks + durTicks
				curMelTicks += durTicks
				continue
			} else {
				u := unitTicks()
				if int(b) > 1 {
					noteTicks = (int(b)-1)*u + (u*90)/100
				} else {
					noteTicks = (u * 90) / 100
				}
			}
			// Enforce instrument melody capability
			if strictCLTF && !inst.hasMelody {
				curMelTicks += durTicks
				lastMelIdx = -1
				lastMelKey = -1
				lastMelEnd = 0
				continue
			}
			// Range and instrument-specific restrictions
			midiKey := key + inst.octave*12
			if strictCLTF && !allowedNoteForInst(inst, midiKey) {
				curMelTicks += durTicks
				lastMelIdx = -1
				lastMelKey = -1
				lastMelEnd = 0
				continue
			}
			baseVel := velocity * inst.melody / 100
			// Linear mapping per classic
			v := (baseVel * volMel10) / 10
			if v < 1 {
				v = 1
			} else if v > 127 {
				v = 127
			}
			n := Note{Key: midiKey, Velocity: v, Start: ticksToDur(curMelTicks), Duration: ticksToDur(noteTicks)}
			notes = append(notes, n)
			lastMelIdx = len(notes) - 1
			lastMelKey = midiKey
			lastMelEnd = curMelTicks + durTicks
			curMelTicks += durTicks
			continue
		}
		// unknown: skip
		i++
	}
	// finalize any active long-chord notes at end-of-song (melody end)
	if len(activeLong) > 0 {
		for _, idx := range activeLong {
			end := curMelTicks
			if end < int(notes[idx].Start.Milliseconds()) {
				end = int(notes[idx].Start.Milliseconds())
			}
			notes[idx].Duration = ticksToDur(end) - notes[idx].Start
		}
	}

	// chord–melody gating: if there is no melody timeline at all, drop all chord notes;
	// otherwise, truncate chord notes to the melody end time.
	if curMelTicks == 0 {
		// filter out chord-originated notes
		if len(chordIdx) == 0 {
			return notes
		}
		keep := make([]bool, len(notes))
		for i := range keep {
			keep[i] = true
		}
		for _, idx := range chordIdx {
			if idx >= 0 && idx < len(keep) {
				keep[idx] = false
			}
		}
		out := make([]Note, 0, len(notes)-len(chordIdx))
		for i, n := range notes {
			if keep[i] {
				out = append(out, n)
			}
		}
		return out
	}
	// Otherwise clip chord notes that exceed melody end
	for _, idx := range chordIdx {
		if idx < 0 || idx >= len(notes) {
			continue
		}
		st := int(notes[idx].Start.Milliseconds())
		en := st + int(notes[idx].Duration.Milliseconds())
		melEndMS := int((time.Duration(curMelTicks) * time.Second / 600).Milliseconds())
		if en > melEndMS {
			en = melEndMS
		}
		if en < st {
			en = st
		}
		notes[idx].Duration = ms(en - st)
	}
	// Drop any zero-duration notes (edge cases after clipping)
	out := notes[:0]
	for _, n := range notes {
		if n.Duration <= 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func ms(x int) time.Duration { return time.Duration(int64(x)) * time.Millisecond }

// allowedNoteForInst enforces classic instrument note-range and special-case
// restrictions. Range: 3 octaves centered at instrument.octave offset.
// Orga drum (program 117) allows only G and B across the range.
func allowedNoteForInst(inst instrument, midi int) bool {
	if !strictCLTF {
		return true
	}
	base := 60 + inst.octave*12
	min := base - 12
	max := base + 24
	if midi < min || midi > max {
		return false
	}
	// Orga drum: only G (7) and B (11) permitted
	if inst.program == 117 {
		off := ((midi % 12) + 12) % 12
		return off == 7 || off == 11
	}
	return true
}

// map cdefgab to semitone offset from C (0)
func isNoteLetter(b byte) bool {
	return (b >= 'a' && b <= 'g') || (b >= 'A' && b <= 'G')
}

func noteOffsetClassic(r rune) int {
	switch unicode.ToLower(r) {
	case 'c':
		return 0
	case 'd':
		return 2
	case 'e':
		return 4
	case 'f':
		return 5
	case 'g':
		return 7
	case 'a':
		return 9
	case 'b':
		return 11
	}
	return -1
}

func parseNotePitch(s string, i *int, octave int) (int, bool) {
	// starting at *i points to note letter
	if *i >= len(s) {
		return -1, false
	}
	c := rune(s[*i])
	off := noteOffsetClassic(c)
	if off < 0 {
		return -1, false
	}
	*i = *i + 1
	// accidental / tie / duration parsed outside except tie here
	// handle immediate modifiers: '#' sharp, '.' flat, '_' tie marker recorded via return bool
	tied := false
	// consume chain of # . _ modifiers until non-mod char
	for *i < len(s) {
		ch := s[*i]
		switch ch {
		case '#':
			off++
			*i++
		case '.':
			off--
			*i++
		case '_':
			tied = true
			*i++
		default:
			goto done
		}
	}
done:
	// base MIDI middle C=60, octave 0 is central (C4)
	midi := 60 + off + octave*12
	return midi, tied
}

// stripComments removes <...> (nested) from s
func stripComments(s string) string {
	b := strings.Builder{}
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '<' {
			depth++
			continue
		}
		if c == '>' {
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// expandLoopsClassic expands ( ... )N with optional indexed endings |1..|9 and default !
func expandLoopsClassic(s string) string {
	// recursive descent
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '(' {
			out.WriteByte(s[i])
			i++
			continue
		}
		// find matching ) at same depth
		i++
		start := i
		depth := 1
		for i < len(s) && depth > 0 {
			if s[i] == '(' {
				depth++
			} else if s[i] == ')' {
				depth--
			}
			if depth == 0 {
				break
			}
			i++
		}
		if i >= len(s) { // unmatched, write rest
			out.WriteString(s[start-1:])
			break
		}
		content := s[start:i]
		i++ // skip ')'
		// optional count digit
		count := 1
		if i < len(s) && s[i] >= '1' && s[i] <= '9' {
			count = int(s[i] - '0')
			i++
		}
		// split content at top-level endings
		mainBody, endings, defEnd := splitEndings(content)
		// expand
		for iter := 1; iter <= count; iter++ {
			out.WriteString(expandLoopsClassic(mainBody))
			if e, ok := endings[iter]; ok {
				out.WriteString(expandLoopsClassic(e))
			} else if defEnd != "" {
				out.WriteString(expandLoopsClassic(defEnd))
			}
		}
	}
	return out.String()
}

func splitEndings(content string) (main string, endings map[int]string, def string) {
	endings = map[int]string{}
	// scan content at depth 0 to locate |digit and !
	type seg struct {
		idx   int
		kind  byte
		label int
	}
	splits := []seg{}
	depth := 0
	for i := 0; i < len(content); i++ {
		c := content[i]
		if c == '[' || c == '<' || c == '(' {
			depth++
		}
		if c == ']' || c == '>' || c == ')' {
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && (c == '|' || c == '!') {
			if c == '|' && i+1 < len(content) && content[i+1] >= '1' && content[i+1] <= '9' {
				splits = append(splits, seg{idx: i, kind: '|', label: int(content[i+1] - '0')})
			} else if c == '!' {
				splits = append(splits, seg{idx: i, kind: '!'})
			}
		}
	}
	if len(splits) == 0 {
		return content, endings, ""
	}
	main = content[:splits[0].idx]
	for si := 0; si < len(splits); si++ {
		start := splits[si].idx
		// skip marker and label
		if splits[si].kind == '|' {
			start += 2
		} else {
			start += 1
		}
		end := len(content)
		if si+1 < len(splits) {
			end = splits[si+1].idx
		}
		segTxt := content[start:end]
		if splits[si].kind == '|' {
			endings[splits[si].label] = segTxt
		} else {
			def = segTxt
		}
	}
	return
}
