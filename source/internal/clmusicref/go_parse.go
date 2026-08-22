package clmusicref

// parseRefInternal is a minimal Go implementation of the tune string parser
// used in tests and, via ParseRef, at runtime. It focuses on timing issues:
// notes, rests, ties, and inline tempo (@, @+N, @-N, @N). It returns a timeline
// of RefEvent with millisecond timing.
func parseRefInternal(s string, baseTempo int) []RefEvent {
	tempo := baseTempo
	if tempo <= 0 {
		tempo = 120
	}
	startMS := 0
	i := 0
	b := []byte(s)
	var out []RefEvent

	noteOffset := func(ch byte) int {
		switch ch {
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

	for i < len(b) {
		c := b[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			i++
			continue
		case '@':
			i++
			sign := byte(0)
			if i < len(b) && (b[i] == '+' || b[i] == '-') {
				sign = b[i]
				i++
			}
			val := 0
			for i < len(b) && b[i] >= '0' && b[i] <= '9' {
				val = val*10 + int(b[i]-'0')
				i++
			}
			if val == 0 {
				// restore to base tempo when no value present
				tempo = baseTempo
				if tempo <= 0 {
					tempo = 120
				}
			} else {
				switch sign {
				case '+':
					tempo += val
				case '-':
					tempo -= val
				default:
					tempo = val
				}
				if tempo < 1 {
					tempo = 1
				}
			}
			continue
		case 'p':
			i++
			beats := 2.0
			if i < len(b) && b[i] >= '1' && b[i] <= '9' {
				beats = float64(b[i] - '0')
				i++
			}
			durMS := int((beats/4.0)*(60000.0/float64(tempo)) + 0.5)
			startMS += durMS
			continue
		}

		// Note
		isNote := false
		base := -1
		upper := false
		if (c >= 'a' && c <= 'g') || (c >= 'A' && c <= 'G') {
			isNote = true
			upper = (c >= 'A' && c <= 'G')
			low := c
			if upper {
				low = c - 'A' + 'a'
			}
			base = noteOffset(low)
			i++
		}
		if !isNote {
			i++
			continue
		}
		pitch := base + (4+1)*12 // octave 4
		beats := 2.0
		if upper {
			beats = 4.0
		}
		tied := false
		for i < len(b) {
			m := b[i]
			if m == '#' {
				pitch++
				i++
			} else if m == '.' {
				pitch--
				i++
			} else if m == '_' {
				tied = true
				i++
			} else if m >= '1' && m <= '9' {
				beats = float64(m - '0')
				i++
			} else {
				break
			}
		}
		durFull := int((beats/4.0)*(60000.0/float64(tempo)) + 0.5)
		gap := int((1500.0 / float64(tempo)) + 0.5)
		noteMS := durFull
		if !tied {
			noteMS -= gap
			if noteMS < 0 {
				noteMS = 0
			}
		}

		if tied && len(out) > 0 {
			// Extend previous event when same key and contiguous
			prev := &out[len(out)-1]
			if len(prev.Keys) == 1 && prev.Keys[0] == pitch {
				prev.DurMS += durFull
			} else {
				out = append(out, RefEvent{StartMS: startMS, DurMS: noteMS, Keys: []int{pitch}})
			}
		} else {
			out = append(out, RefEvent{StartMS: startMS, DurMS: noteMS, Keys: []int{pitch}})
		}
		startMS += durFull
	}
	return out
}

// ParseRef parses a tune string at baseTempo using the internal Go parser.
// It mirrors the previous C implementation but avoids cgo overhead.
func ParseRef(s string, baseTempo int) ([]RefEvent, error) {
	return parseRefInternal(s, baseTempo), nil
}
