package main

import (
	"time"
	"unicode"
)

// The classic clock uses 600 ticks/second and floor(9000/tempo) ticks per
// sixteenth. Only melody notes and rests advance the cursor. Finite chords
// retain their own durations; sustained chords end on a matching pitch or at
// the end of the notation's timeline.
type tuneParseErrorCode string

const (
	tuneErrorInvalidNote            tuneParseErrorCode = "invalid_note"
	tuneErrorToneOverflow           tuneParseErrorCode = "tone_overflow"
	tuneErrorInvalidChord           tuneParseErrorCode = "invalid_chord"
	tuneErrorPolyphonyOverflow      tuneParseErrorCode = "polyphony_overflow"
	tuneErrorUnsupportedInstrument  tuneParseErrorCode = "unsupported_instrument"
	tuneErrorInvalidTempo           tuneParseErrorCode = "invalid_tempo"
	tuneErrorInvalidTempoChange     tuneParseErrorCode = "invalid_tempo_change"
	tuneErrorModifierNeedValue      tuneParseErrorCode = "modifier_need_value"
	tuneErrorDuplicateEnding        tuneParseErrorCode = "duplicate_ending"
	tuneErrorDuplicateDefaultEnding tuneParseErrorCode = "duplicate_default_ending"
	tuneErrorEndingInChord          tuneParseErrorCode = "ending_in_chord"
	tuneErrorEndingOutsideLoop      tuneParseErrorCode = "ending_outside_loop"
	tuneErrorInvalidEndingIndex     tuneParseErrorCode = "invalid_ending_index"
	tuneErrorTooManyLoops           tuneParseErrorCode = "too_many_marks"
	tuneErrorUnmatchedLoop          tuneParseErrorCode = "unmatched_mark"
	tuneErrorUnterminatedLoop       tuneParseErrorCode = "unterminated_loop"
	tuneErrorUnterminatedChord      tuneParseErrorCode = "unterminated_chord"
	tuneErrorUnmatchedComment       tuneParseErrorCode = "unmatched_comment"
	tuneErrorUnterminatedComment    tuneParseErrorCode = "unterminated_comment"
	tuneErrorTooLarge               tuneParseErrorCode = "tune_too_large"
)

type tuneParseError struct {
	Code     tuneParseErrorCode
	Position int // byte offset in the original notation, before loop expansion
}

func (e *tuneParseError) Error() string {
	switch e.Code {
	case tuneErrorInvalidNote:
		return "invalid note or misplaced modifier"
	case tuneErrorToneOverflow:
		return "a sharp or flat puts this note outside the instrument's range"
	case tuneErrorInvalidChord, tuneErrorPolyphonyOverflow:
		return "too many chord notes are playing for this instrument"
	case tuneErrorUnsupportedInstrument:
		return "long chords are not supported on this instrument"
	case tuneErrorInvalidTempo:
		return "tempo must be between 60 and 180"
	case tuneErrorInvalidTempoChange:
		return "the tempo cannot change while chords are playing"
	case tuneErrorModifierNeedValue:
		return "a relative tempo change needs a value"
	case tuneErrorDuplicateEnding:
		return "a loop has duplicate numbered endings"
	case tuneErrorDuplicateDefaultEnding:
		return "a loop has more than one default ending"
	case tuneErrorEndingInChord:
		return "a chord cannot contain a loop ending"
	case tuneErrorEndingOutsideLoop:
		return "a loop ending must be inside a loop"
	case tuneErrorInvalidEndingIndex:
		return "use |1 through |9 for a loop ending"
	case tuneErrorTooManyLoops:
		return "too many nested loops (maximum six)"
	case tuneErrorUnmatchedLoop:
		return "unmatched loop ending"
	case tuneErrorUnterminatedLoop:
		return "unclosed loop"
	case tuneErrorUnterminatedChord:
		return "unclosed chord"
	case tuneErrorUnmatchedComment:
		return "unmatched comment ending"
	case tuneErrorUnterminatedComment:
		return "unclosed comment"
	case tuneErrorTooLarge:
		return "the expanded tune is too large"
	default:
		return "invalid tune"
	}
}

type activeChordNote struct {
	noteIndex  int
	logicalEnd int
	long       bool
}

var strictCLTF = true

func tuneTicksDuration(ticks int) time.Duration {
	return time.Duration((int64(ticks)*int64(time.Second) + 300) / 600)
}

func classicNotesFromTune(tune string, inst instrument, tempo int, velocity int) []Note {
	notes, _ := parseClassicTune(tune, inst, tempo, velocity)
	return notes
}

func parseClassicTune(tune string, inst instrument, tempo int, velocity int) ([]Note, *tuneParseError) {
	notes, _, err := parseClassicTuneTimeline(tune, inst, tempo, velocity)
	return notes, err
}

func parseClassicTuneTimeline(tune string, inst instrument, tempo int, velocity int) ([]Note, time.Duration, *tuneParseError) {
	tokens, err := readTuneTokens(tune)
	if err != nil {
		return nil, 0, err
	}
	tokens = expandTuneTokens(tokens)
	if tempo <= 0 {
		tempo = 120
	}
	octave, volMel10, volCh10, cursor := 0, 10, 10, 0
	var notes []Note
	active := map[int]activeChordNote{}
	var chord []int
	var chordStart tuneToken
	inChord := false
	fail := func(err *tuneParseError) ([]Note, time.Duration, *tuneParseError) {
		return notes, tuneTicksDuration(cursor), err
	}
	for _, token := range tokens {
		for key, voice := range active {
			if !voice.long && voice.logicalEnd <= cursor {
				delete(active, key)
			}
		}
		text, c := token.text, token.text[0]
		unit := 9000 / tempo
		switch c {
		case '+', '-', '=', '/', '\\':
			if token.silent {
				continue
			}
			switch c {
			case '+':
				octave = min(octave+1, 1)
			case '-':
				octave = max(octave-1, -1)
			case '=':
				octave = 0
			case '/':
				octave = 1
			case '\\':
				octave = -1
			}
		case '%', '{', '}':
			if token.silent {
				continue
			}
			volume := &volMel10
			if inChord {
				volume = &volCh10
			}
			digit := 0
			if len(text) > 1 {
				digit = int(text[1] - '0')
			}
			switch c {
			case '%':
				*volume = digit
				if digit == 0 {
					*volume = 10
				}
			case '{':
				*volume -= max(digit, 1)
			case '}':
				*volume += max(digit, 1)
			}
			*volume = max(1, min(*volume, 10))
		case '@':
			if !token.silent && (inChord || len(active) > 0) {
				return fail(token.failure(tuneErrorInvalidTempoChange, 0))
			}
			nextTempo, parseErr := classicTempo(token, tempo)
			if parseErr != nil {
				return fail(parseErr)
			}
			tempo = nextTempo
		case 'p':
			if token.silent {
				continue
			}
			units := 2
			if len(text) > 1 {
				units = int(text[1] - '0')
			}
			cursor += units * unit
		case '[':
			inChord, chordStart, chord = true, token, nil
		case ']':
			inChord = false
			units, long := 4, false
			if len(text) > 1 {
				if text[1] == '$' {
					long = true
				} else {
					units = int(text[1] - '0')
				}
			}
			if strictCLTF && long && !inst.longChord {
				return fail(token.failure(tuneErrorUnsupportedInstrument, 1))
			}
			if token.silent || len(chord) == 0 || inst.chord == 0 {
				continue
			}
			// Reuse the classic pitch slot without erasing finite note events.
			// A repeated sustained pitch instead ends the earlier long note.
			occupied := make(map[int]bool, len(active))
			for key, voice := range active {
				occupied[key] = voice.long
			}
			for _, key := range chord {
				if strictCLTF && !allowedNoteForInst(inst, key) {
					continue
				}
				if wasLong, found := occupied[key]; found {
					delete(occupied, key)
					if wasLong && long {
						continue
					}
				}
				if strictCLTF && len(occupied) >= inst.polyphony {
					return fail(chordStart.failure(tuneErrorPolyphonyOverflow, 0))
				}
				occupied[key] = long
			}
			for _, key := range chord {
				if strictCLTF && !allowedNoteForInst(inst, key) {
					continue
				}
				if voice, found := active[key]; found {
					if voice.long {
						notes[voice.noteIndex].Duration = tuneTicksDuration(cursor) - notes[voice.noteIndex].Start
					}
					delete(active, key)
					if voice.long && long {
						continue
					}
				}
				notes = append(notes, Note{Key: key, Velocity: max(0, min(127, (velocity*inst.chord/100)*volCh10/10)), Start: tuneTicksDuration(cursor), Duration: tuneTicksDuration((units-1)*unit + unit*90/100)})
				active[key] = activeChordNote{noteIndex: len(notes) - 1, logicalEnd: cursor + units*unit, long: long && inst.longChord}
			}
		default: // The shared reader permits only note tokens here.
			key, units, linked, parseErr := classicNote(token, inst, octave)
			if parseErr != nil {
				return fail(parseErr)
			}
			if inChord {
				chord = append(chord, key)
				if strictCLTF && (!inst.hasChords || len(chord) > inst.polyphony) {
					return fail(chordStart.failure(tuneErrorInvalidChord, 0))
				}
				continue
			}
			if token.silent {
				continue
			}
			if !strictCLTF || inst.hasMelody && allowedNoteForInst(inst, key) {
				ticks := (units-1)*unit + unit*90/100
				if linked {
					ticks = units * unit
				}
				notes = append(notes, Note{Key: key, Velocity: max(0, min(127, (velocity*inst.melody/100)*volMel10/10)), Start: tuneTicksDuration(cursor), Duration: tuneTicksDuration(ticks)})
			}
			cursor += units * unit
		}
	}
	for _, voice := range active {
		if voice.long {
			notes[voice.noteIndex].Duration = tuneTicksDuration(cursor) - notes[voice.noteIndex].Start
		}
	}
	// Keep explicit rests in the reported duration, and include finite chord
	// tails even when there is no melody or the last melody note ends earlier.
	duration := tuneTicksDuration(cursor)
	out := notes[:0]
	for _, note := range notes {
		if note.Duration <= 0 {
			continue
		}
		out = append(out, note)
		duration = max(duration, note.Start+note.Duration)
	}
	return out, duration, nil
}

func classicTempo(token tuneToken, current int) (int, *tuneParseError) {
	value, sign := 0, byte(0)
	for i := 1; i < len(token.text); i++ {
		c := token.text[i]
		if c == '+' || c == '-' || c == '=' {
			sign = c
			continue
		}
		value = value*10 + int(c-'0')
		if value > 180 {
			return 0, token.failure(tuneErrorInvalidTempo, i)
		}
	}
	if token.silent {
		return current, nil
	}
	if value == 0 {
		if sign == '=' {
			return 0, token.failure(tuneErrorInvalidTempo, 0)
		}
		if sign != 0 {
			return 0, token.failure(tuneErrorModifierNeedValue, 0)
		}
		return 120, nil
	}
	switch sign {
	case '+':
		return min(180, current+value), nil
	case '-':
		return max(60, current-value), nil
	}
	if value < 60 {
		return 0, token.failure(tuneErrorInvalidTempo, 0)
	}
	return value, nil
}

func classicNote(token tuneToken, inst instrument, octave int) (key, units int, linked bool, err *tuneParseError) {
	key = 60 + noteOffsetClassic(rune(token.text[0])) + (octave+inst.octave)*12
	units = 2
	if token.text[0] >= 'A' && token.text[0] <= 'G' {
		units = 4
	}
	for i := 1; i < len(token.text); i++ {
		switch token.text[i] {
		case '#':
			key++
			if strictCLTF && (key > 60+inst.octave*12+24 || key >= 96) {
				return 0, 0, false, token.failure(tuneErrorToneOverflow, i)
			}
		case '.':
			key--
			if strictCLTF && key < 60+inst.octave*12-12 {
				return 0, 0, false, token.failure(tuneErrorToneOverflow, i)
			}
		case '_':
			linked = true
		default:
			units = int(token.text[i] - '0')
		}
	}
	return
}

func allowedNoteForInst(inst instrument, midi int) bool {
	if !strictCLTF {
		return true
	}
	base := 60 + inst.octave*12
	if midi < base-12 || midi > base+24 {
		return false
	}
	if inst.allowedNoteClass != 0 {
		off := ((midi % 12) + 12) % 12
		return inst.allowedNoteClass&(1<<off) != 0
	}
	return true
}

func isNoteLetter(b byte) bool { return b >= 'a' && b <= 'g' || b >= 'A' && b <= 'G' }

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
