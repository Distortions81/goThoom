package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstrumentAuditionCoversEveryClassicMapping(t *testing.T) {
	sections := instrumentAuditionSections()
	if len(sections) != classicInstrumentCount || len(classicInstrumentNames) != classicInstrumentCount {
		t.Fatalf("audition sections = %d, names = %d, want %d", len(sections), len(classicInstrumentNames), classicInstrumentCount)
	}
	for index, section := range sections {
		inst := instruments[index]
		if section.index != index || section.name == "" || section.program != inst.classicProgram-1 || section.program != inst.program {
			t.Errorf("section %d does not match instrument mapping: section=%+v instrument=%+v", index, section, inst)
		}
		if len(section.notes) != 5 {
			t.Errorf("section %d notes = %d, want 5", index, len(section.notes))
		}
		for _, note := range section.notes {
			if !allowedNoteForInst(inst, note.Key) {
				t.Errorf("section %d contains restricted note %d", index, note.Key)
			}
		}
	}
}

func TestInstrumentAuditionMIDIUsesEveryClassicProgram(t *testing.T) {
	data := instrumentAuditionMIDI(instrumentAuditionSections())
	if len(data) < 22 || string(data[:4]) != "MThd" || string(data[14:18]) != "MTrk" {
		t.Fatalf("invalid MIDI header: %x", data[:min(len(data), 22)])
	}
	if got := binary.BigEndian.Uint16(data[12:14]); got != instrumentAuditionTPQ {
		t.Fatalf("MIDI ticks per quarter = %d, want %d", got, instrumentAuditionTPQ)
	}

	trackLen := int(binary.BigEndian.Uint32(data[18:22]))
	if trackLen != len(data)-22 {
		t.Fatalf("MIDI track length = %d, data contains %d", trackLen, len(data)-22)
	}
	programs, noteOns, markers, err := parseInstrumentAuditionMIDITrack(data[22:])
	if err != nil {
		t.Fatal(err)
	}
	if len(programs) != classicInstrumentCount || noteOns != classicInstrumentCount*5 || markers != classicInstrumentCount {
		t.Fatalf("MIDI programs=%d note-ons=%d markers=%d", len(programs), noteOns, markers)
	}
	for index, program := range programs {
		if program != instruments[index].program {
			t.Errorf("MIDI program %d = %d, want %d", index, program, instruments[index].program)
		}
	}
}

func parseInstrumentAuditionMIDITrack(track []byte) (programs []int, noteOns, markers int, err error) {
	for position := 0; position < len(track); {
		_, next, ok := readMIDIVariableLength(track, position)
		if !ok || next >= len(track) {
			return nil, 0, 0, os.ErrInvalid
		}
		position = next
		status := track[position]
		position++
		switch {
		case status == 0xff:
			if position >= len(track) {
				return nil, 0, 0, os.ErrInvalid
			}
			kind := track[position]
			position++
			length, next, ok := readMIDIVariableLength(track, position)
			if !ok || length < 0 || next+length > len(track) {
				return nil, 0, 0, os.ErrInvalid
			}
			if kind == 0x06 {
				markers++
			}
			position = next + length
		case status&0xf0 == 0xc0:
			if position >= len(track) {
				return nil, 0, 0, os.ErrInvalid
			}
			programs = append(programs, int(track[position]))
			position++
		case status&0xf0 == 0x80 || status&0xf0 == 0x90:
			if position+2 > len(track) {
				return nil, 0, 0, os.ErrInvalid
			}
			if status&0xf0 == 0x90 && track[position+1] != 0 {
				noteOns++
			}
			position += 2
		default:
			return nil, 0, 0, os.ErrInvalid
		}
	}
	return programs, noteOns, markers, nil
}

func readMIDIVariableLength(data []byte, position int) (value, next int, ok bool) {
	for count := 0; count < 4 && position < len(data); count++ {
		b := data[position]
		position++
		value = value<<7 | int(b&0x7f)
		if b&0x80 == 0 {
			return value, position, true
		}
	}
	return 0, position, false
}

func TestWritePCMAsWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audition.wav")
	pcm := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	if err := writePCMAsWAV(path, pcm); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 44+len(pcm) || !bytes.Equal(data[:4], []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		t.Fatalf("invalid WAV header: %x", data[:min(len(data), 44)])
	}
	if got := binary.LittleEndian.Uint32(data[40:44]); got != uint32(len(pcm)) {
		t.Fatalf("WAV data length = %d, want %d", got, len(pcm))
	}
}

func TestInstrumentAuditionOutputBaseUsesLaunchDirectory(t *testing.T) {
	original := processLaunchDirectory
	processLaunchDirectory = filepath.Join(string(filepath.Separator), "launch")
	t.Cleanup(func() { processLaunchDirectory = original })
	if got, want := instrumentAuditionOutputBase("audition"), filepath.Join(string(filepath.Separator), "launch", "audition"); got != want {
		t.Fatalf("relative output = %q, want %q", got, want)
	}
	absolute := filepath.Join(string(filepath.Separator), "tmp", "audition")
	if got := instrumentAuditionOutputBase(absolute); got != absolute {
		t.Fatalf("absolute output = %q, want %q", got, absolute)
	}
}

func TestCheckedInInstrumentAuditionFiles(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join(processLaunchDirectory, ".."))
	midi, err := os.ReadFile(filepath.Join(repositoryRoot, "bard-instrument-audition.mid"))
	if err != nil {
		t.Fatal(err)
	}
	if want := instrumentAuditionMIDI(instrumentAuditionSections()); !bytes.Equal(midi, want) {
		t.Fatal("checked-in instrument audition MIDI is stale")
	}

	wantPCMBytes := uint32(classicInstrumentCount * int(instrumentAuditionSegment/time.Second) * sampleRate * 4)
	for _, name := range []string{"bard-instrument-audition.wav", "bard-instrument-audition-quicktime.wav"} {
		if err := checkInstrumentAuditionWAV(filepath.Join(repositoryRoot, name), wantPCMBytes); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func checkInstrumentAuditionWAV(path string, wantPCMBytes uint32) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return err
	}
	if string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" {
		return fmt.Errorf("invalid RIFF/WAVE header")
	}

	foundFormat := false
	for {
		var chunk [8]byte
		if _, err := io.ReadFull(f, chunk[:]); err != nil {
			return err
		}
		size := binary.LittleEndian.Uint32(chunk[4:])
		switch string(chunk[:4]) {
		case "fmt ":
			if size < 16 {
				return fmt.Errorf("short format chunk")
			}
			var format [16]byte
			if _, err := io.ReadFull(f, format[:]); err != nil {
				return err
			}
			if binary.LittleEndian.Uint16(format[:2]) != 1 || binary.LittleEndian.Uint16(format[2:4]) != 2 || binary.LittleEndian.Uint32(format[4:8]) != sampleRate || binary.LittleEndian.Uint16(format[14:16]) != 16 {
				return fmt.Errorf("want 44.1 kHz, stereo, 16-bit PCM")
			}
			foundFormat = true
			size -= 16
		case "data":
			if !foundFormat {
				return fmt.Errorf("data precedes format chunk")
			}
			if size != wantPCMBytes {
				return fmt.Errorf("PCM bytes = %d, want %d", size, wantPCMBytes)
			}
			return nil
		}
		if _, err := f.Seek(int64(size+(size&1)), io.SeekCurrent); err != nil {
			return err
		}
	}
}
