package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Import notation into a self-contained native copy. Filename extensions do
// not establish compatibility with a tool's project format or command export.
func importBardTune(path string) (bardTune, error) {
	value, err := readBardTune(path)
	if err != nil {
		return bardTune{}, err
	}
	fallback := bardLegacyInstrument(path)
	score, err := parseBardScore(value, fallback)
	if err == nil {
		for _, part := range score.Parts {
			if _, err = bardNoteTokens(part.Text); err != nil {
				break
			}
		}
	}
	if err != nil {
		return bardTune{}, fmt.Errorf("Cannot import this file as tune notation: %w Export or copy plain Clan Lord notation from the original tool first.", err)
	}
	// Explicit metadata takes precedence over old sidecar preferences. Record
	// resolved defaults for parts that would otherwise depend on that sidecar.
	defaults := make(map[int]string)
	for _, part := range score.Parts {
		if part.instrumentLine < 0 {
			defaults[part.startLine] = bardMetadata("instrument", classicInstrumentNames[part.Instrument])
		}
	}
	if len(defaults) > 0 {
		// Insert in one pass; large arrangements must not be reparsed once
		// for every part just to carry their resolved instruments along.
		var lines []string
		if instrument, ok := defaults[-1]; ok {
			lines = append(lines, instrument)
		}
		for i, line := range strings.Split(value, "\n") {
			lines = append(lines, line)
			if instrument, ok := defaults[i]; ok {
				lines = append(lines, instrument)
			}
		}
		value = strings.Join(lines, "\n")
	}
	return createBardTune(bardNativeFilename(filepath.Base(path)), value)
}
