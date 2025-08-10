package main

import (
	"math"
	"strconv"
	"strings"

	"maze.io/x/math32"
)

// playTuneSimple parses a space-separated list of note names and plays them as a
// sequence of sine waves. Each note is assumed to be a quarter note at a
// fixed tempo and octave if not specified (default octave 4).
func playTuneSimple(tune string) {
	if audioContext == nil {
		return
	}
	notes := strings.Fields(tune)
	if len(notes) == 0 {
		return
	}
	rate := audioContext.SampleRate()
	const durMS = 200
	buf := make([]byte, 0, len(notes)*rate*durMS/1000*2)
	for _, n := range notes {
		freq := parseNote(n)
		if freq == 0 {
			continue
		}
		samples := synthSine(freq, rate, durMS)
		for _, v := range samples {
			buf = append(buf, byte(v), byte(v>>8))
		}
	}
	p := audioContext.NewPlayerFromBytes(buf)
	p.SetVolume(0.2)
	p.Play()
}

// parseNote converts a note string like "C4" into a frequency in Hz.
// If the octave is omitted, octave 4 is assumed. Only natural notes A-G
// are recognised.
func parseNote(s string) float32 {
	if s == "" {
		return 0
	}
	noteOffsets := map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}
	up := strings.ToUpper(s)
	base, ok := noteOffsets[up[0]]
	if !ok {
		return 0
	}
	octave := 4
	if len(up) > 1 {
		if o, err := strconv.Atoi(up[1:]); err == nil {
			octave = o
		}
	}
	midi := base + (octave+1)*12
	return 440 * math32.Pow(2, float32(midi-69)/12)
}

// synthSine generates a sine wave for the given frequency and duration.
func synthSine(freq float32, rate, durMS int) []int16 {
	n := rate * durMS / 1000
	samples := make([]int16, n)
	for i := 0; i < n; i++ {
		v := math32.Sin(2 * math32.Pi * freq * float32(i) / float32(rate))
		samples[i] = int16(v * math.MaxInt16)
	}
	return samples
}
