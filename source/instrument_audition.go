package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	instrumentAuditionNoteStep = 400 * time.Millisecond
	instrumentAuditionNoteHold = 300 * time.Millisecond
	instrumentAuditionSegment  = 4 * time.Second
	instrumentAuditionTPQ      = 480
)

var classicInstrumentNames = [...]string{
	"Lucky Lyra",
	"Bone Flute",
	"Starbuck Harp",
	"Torjo",
	"Xylo",
	"Gitor",
	"Reed Flute",
	"Temple Organ",
	"Conch",
	"Ocarina",
	"Centaur Organ",
	"Vibra",
	"Tuborn",
	"Bagpipe",
	"Orga Drum",
	"Casserole",
	"Violene",
	"Pine Flute",
	"Groanbox",
	"Gho-To",
	"Mammoth Violene",
	"Gutbucket Bass",
	"Glass Jug",
}

var processLaunchDirectory = func() string {
	directory, err := os.Getwd()
	if err != nil {
		return "."
	}
	return directory
}()

type instrumentAuditionSection struct {
	index   int
	name    string
	program int
	notes   []Note
}

func instrumentAuditionSections() []instrumentAuditionSection {
	// G and B across three octaves exercise the useful range of every classic
	// instrument and also satisfy Orga Drum's restricted pitch classes.
	baseKeys := [...]int{55, 59, 67, 71, 79}
	sections := make([]instrumentAuditionSection, len(instruments))
	for index, inst := range instruments {
		notes := make([]Note, len(baseKeys))
		for noteIndex, key := range baseKeys {
			notes[noteIndex] = Note{
				Key:      key + inst.octave*12,
				Velocity: 100,
				Start:    time.Duration(noteIndex) * instrumentAuditionNoteStep,
				Duration: instrumentAuditionNoteHold,
			}
		}
		sections[index] = instrumentAuditionSection{
			index:   index,
			name:    classicInstrumentNames[index],
			program: inst.program,
			notes:   notes,
		}
	}
	return sections
}

func exportInstrumentAudition(outputBase, soundFontName string) error {
	if outputBase == "" {
		return fmt.Errorf("instrument audition output path is empty")
	}
	outputBase = instrumentAuditionOutputBase(outputBase)
	if soundFontName != "" {
		if filepath.Base(soundFontName) != soundFontName || !strings.EqualFold(filepath.Ext(soundFontName), ".sf2") {
			return fmt.Errorf("invalid soundfont filename %q", soundFontName)
		}
		gs.SoundFontFile = soundFontName
	}

	sections := instrumentAuditionSections()
	midiPath := outputBase + ".mid"
	wavPath := outputBase + ".wav"
	if err := os.WriteFile(midiPath, instrumentAuditionMIDI(sections), 0o644); err != nil {
		return fmt.Errorf("write instrument audition MIDI: %w", err)
	}
	if err := renderInstrumentAuditionWAV(wavPath, sections); err != nil {
		return err
	}
	return nil
}

func instrumentAuditionOutputBase(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(processLaunchDirectory, path)
}

func renderInstrumentAuditionWAV(path string, sections []instrumentAuditionSection) error {
	segmentFrames := int(instrumentAuditionSegment * sampleRate / time.Second)
	pcm := make([]byte, 0, len(sections)*segmentFrames*4)
	for _, section := range sections {
		left, right, err := renderSong(section.program, section.notes)
		if err != nil {
			return fmt.Errorf("render instrument %d %s: %w", section.index, section.name, err)
		}
		if len(left) > segmentFrames {
			left = left[:segmentFrames]
			right = right[:segmentFrames]
		} else if len(left) < segmentFrames {
			left = append(left, make([]float32, segmentFrames-len(left))...)
			right = append(right, make([]float32, segmentFrames-len(right))...)
		}
		pcm = append(pcm, mixPCMChunk(left, right, true)...)
	}
	if err := writePCMAsWAV(path, pcm); err != nil {
		return fmt.Errorf("write instrument audition WAV: %w", err)
	}
	return nil
}

func instrumentAuditionMIDI(sections []instrumentAuditionSection) []byte {
	var track bytes.Buffer
	writeMIDIMeta(&track, 0, 0x03, "goThoom classic instrument audition")
	// 120 BPM: 500,000 microseconds per quarter note.
	writeMIDIRaw(&track, 0, 0xff, 0x51, 0x03, 0x07, 0xa1, 0x20)

	cursor := 0
	segmentTicks := durationToMIDITicks(instrumentAuditionSegment)
	for sectionIndex, section := range sections {
		sectionStart := sectionIndex * segmentTicks
		label := fmt.Sprintf("%02d %s - classic GM %d, synth program %d", section.index, section.name, instruments[section.index].classicProgram, section.program)
		writeMIDIMeta(&track, sectionStart-cursor, 0x06, label)
		cursor = sectionStart
		writeMIDIRaw(&track, 0, 0xc0, byte(section.program))
		for _, note := range section.notes {
			start := sectionStart + durationToMIDITicks(note.Start)
			writeMIDIRaw(&track, start-cursor, 0x90, byte(note.Key), byte(note.Velocity))
			cursor = start
			end := start + durationToMIDITicks(note.Duration)
			writeMIDIRaw(&track, end-cursor, 0x80, byte(note.Key), 0)
			cursor = end
		}
	}
	writeMIDIRaw(&track, len(sections)*segmentTicks-cursor, 0xff, 0x2f, 0x00)

	var midi bytes.Buffer
	midi.WriteString("MThd")
	_ = binary.Write(&midi, binary.BigEndian, uint32(6))
	_ = binary.Write(&midi, binary.BigEndian, uint16(0))
	_ = binary.Write(&midi, binary.BigEndian, uint16(1))
	_ = binary.Write(&midi, binary.BigEndian, uint16(instrumentAuditionTPQ))
	midi.WriteString("MTrk")
	_ = binary.Write(&midi, binary.BigEndian, uint32(track.Len()))
	midi.Write(track.Bytes())
	return midi.Bytes()
}

func durationToMIDITicks(duration time.Duration) int {
	// At 120 BPM there are two quarter notes per second.
	return int((duration.Nanoseconds()*2*instrumentAuditionTPQ + int64(time.Second)/2) / int64(time.Second))
}

func writeMIDIMeta(track *bytes.Buffer, delta int, kind byte, value string) {
	writeMIDIVariableLength(track, delta)
	track.Write([]byte{0xff, kind})
	writeMIDIVariableLength(track, len(value))
	track.WriteString(value)
}

func writeMIDIRaw(track *bytes.Buffer, delta int, event ...byte) {
	writeMIDIVariableLength(track, delta)
	track.Write(event)
}

func writeMIDIVariableLength(dst *bytes.Buffer, value int) {
	var encoded [4]byte
	index := len(encoded) - 1
	encoded[index] = byte(value & 0x7f)
	for value >>= 7; value > 0; value >>= 7 {
		index--
		encoded[index] = byte(value&0x7f) | 0x80
	}
	dst.Write(encoded[index:])
}
