package main

import "strings"

const (
	maxTuneTextBytes     = 256 << 10
	maxExpandedTuneBytes = 100000
	maxTuneLoops         = 6 // CTuneBuilder::max_Marks
)

// A token retains source positions even when comments, spaces, or repeated
// loop bodies separate a modifier from the note it belongs to.
type tuneToken struct {
	text        string
	positions   []int
	spaceBefore bool
	silent      bool // Validate an ending during the classic loop's first pass.
}

func (t tuneToken) failure(code tuneParseErrorCode, offset int) *tuneParseError {
	return &tuneParseError{Code: code, Position: t.positions[offset]}
}

func tuneDigit(c byte) bool    { return c >= '1' && c <= '9' }
func tuneSpace(c byte) bool    { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }
func tuneModifier(c byte) bool { return c == '#' || c == '.' || c == '_' || tuneDigit(c) }

// readTuneTokens checks the instrument-independent grammar once for both
// incoming playback and Bard command splitting. Classic ignores whitespace
// and nested comments even inside a note's modifiers or a tempo value.
func readTuneTokens(value string) ([]tuneToken, *tuneParseError) {
	fail := func(code tuneParseErrorCode, pos int) ([]tuneToken, *tuneParseError) {
		return nil, &tuneParseError{Code: code, Position: pos}
	}
	if len(value) > maxTuneTextBytes {
		return fail(tuneErrorTooLarge, maxTuneTextBytes)
	}
	var text []byte
	var positions []int
	var spaces []bool
	depth, commentStart := 0, 0
	space := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c == '<' {
			if depth == 0 {
				commentStart = i
			}
			depth++
			continue
		}
		if c == '>' {
			if depth == 0 {
				return fail(tuneErrorUnmatchedComment, i)
			}
			depth--
			continue
		}
		if depth > 0 {
			continue
		}
		if tuneSpace(c) {
			space = true
			continue
		}
		text = append(text, c)
		positions = append(positions, i)
		spaces = append(spaces, space)
		space = false
	}
	if depth != 0 {
		return fail(tuneErrorUnterminatedComment, commentStart)
	}
	s := string(text)
	type frame struct {
		kind          byte
		position      int
		cost          int
		endings       [10]bool
		defaultEnding bool
	}
	stack := []frame{{}}
	var tokens []tuneToken
	for i := 0; i < len(s); {
		start := i
		c := s[i]
		i++
		top := &stack[len(stack)-1]
		inChord := top.kind == '['
		switch {
		case isNoteLetter(c):
			for i < len(s) && tuneModifier(s[i]) {
				i++
			}
		case c == '%' || c == '{' || c == '}' || c == 'p' && !inChord:
			if i < len(s) && tuneDigit(s[i]) {
				i++
			}
		case c == '@':
			value := 0
			for i < len(s) {
				if s[i] >= '0' && s[i] <= '9' {
					value = min(181, value*10+int(s[i]-'0'))
				} else if value != 0 || !strings.ContainsRune("+-=", rune(s[i])) {
					break
				}
				i++
			}
		case c == '(' || c == '[':
			if inChord {
				return fail(tuneErrorInvalidNote, positions[start])
			}
			if c == '(' && len(stack) > maxTuneLoops {
				return fail(tuneErrorTooManyLoops, positions[start])
			}
			stack = append(stack, frame{kind: c, position: positions[start]})
		case c == ')' || c == ']':
			want := byte('(')
			if c == ']' {
				want = '['
			}
			if top.kind != want {
				code := tuneErrorInvalidNote
				if c == ')' {
					code = tuneErrorUnmatchedLoop
				}
				return fail(code, positions[start])
			}
			count := 1
			if i < len(s) && tuneDigit(s[i]) {
				if c == ')' {
					count = int(s[i] - '0')
				}
				i++
			} else if c == ']' && i < len(s) && s[i] == '$' {
				i++
			}
			cost := top.cost * count
			stack = stack[:len(stack)-1]
			stack[len(stack)-1].cost += cost
		case c == '|' || c == '!':
			if inChord {
				return fail(tuneErrorEndingInChord, positions[start])
			}
			if top.kind != '(' {
				return fail(tuneErrorEndingOutsideLoop, positions[start])
			}
			if c == '!' {
				if top.defaultEnding {
					return fail(tuneErrorDuplicateDefaultEnding, positions[start])
				}
				top.defaultEnding = true
			} else {
				if i >= len(s) || !tuneDigit(s[i]) {
					return fail(tuneErrorInvalidEndingIndex, positions[start])
				}
				index := int(s[i] - '0')
				if top.endings[index] {
					return fail(tuneErrorDuplicateEnding, positions[start])
				}
				top.endings[index] = true
				i++
			}
		case strings.ContainsRune("+-=/\\", rune(c)):
		default:
			return fail(tuneErrorInvalidNote, positions[start])
		}
		stack[len(stack)-1].cost += i - start
		if stack[len(stack)-1].cost > maxExpandedTuneBytes {
			return fail(tuneErrorTooLarge, positions[start])
		}
		tokens = append(tokens, tuneToken{text: s[start:i], positions: positions[start:i], spaceBefore: spaces[start]})
	}
	if len(stack) > 1 {
		top := stack[len(stack)-1]
		code := tuneErrorUnterminatedLoop
		if top.kind == '[' {
			code = tuneErrorUnterminatedChord
		}
		return fail(code, top.position)
	}
	return tokens, nil
}

// expandTuneTokens receives validated, bounded notation. Keeping tokens intact
// prevents a modifier after a loop from becoming attached to its last note.
func expandTuneTokens(tokens []tuneToken) []tuneToken {
	var out []tuneToken
	for i := 0; i < len(tokens); i++ {
		if tokens[i].text[0] != '(' {
			out = append(out, tokens[i])
			continue
		}
		start, depth := i+1, 1
		i++
		for ; depth > 0; i++ {
			switch tokens[i].text[0] {
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		i--
		count := 1
		if len(tokens[i].text) > 1 {
			count = int(tokens[i].text[1] - '0')
		}
		body := tokens[start:i]
		mainEnd, segmentStart, label := len(body), 0, -1
		endings := map[int][]tuneToken{}
		depth = 0
		for j, token := range body {
			c := token.text[0]
			switch c {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth != 0 || c != '|' && c != '!' {
				continue
			}
			if label == -1 {
				mainEnd = j
			} else {
				endings[label] = body[segmentStart:j]
			}
			label = 0
			if c == '|' {
				label = int(token.text[1] - '0')
			}
			segmentStart = j + 1
		}
		if label != -1 {
			endings[label] = body[segmentStart:]
		}
		main := expandTuneTokens(body[:mainEnd])
		for iteration := 1; iteration <= count; iteration++ {
			out = append(out, main...)
			if iteration == 1 {
				// Classic discovers every ending before playing the selected one.
				// During discovery it checks note modifiers and chord sizes, but
				// ignores timeline, octave, volume, and tempo changes. Nested loops
				// need only one such pass because they cannot change that state.
				for _, token := range body[mainEnd:] {
					if strings.ContainsRune("()|!", rune(token.text[0])) {
						continue
					}
					token.silent = true
					out = append(out, token)
				}
			}
			ending, found := endings[iteration]
			if !found {
				ending = endings[0]
			}
			out = append(out, expandTuneTokens(ending)...)
		}
	}
	return out
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
