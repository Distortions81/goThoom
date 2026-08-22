//go:build testheadless

package main

import (
    "sync"
)

// Minimal types and globals to let movie parsing compile without GUI deps.

type frameDescriptor struct {
    Index  uint8
    Type   uint8
    PictID uint16
    Name   string
    Colors []byte
}

type framePicture struct {
    PictID uint16
    H, V   int16
    Plane  int
}

type frameMobile struct {
    Index  uint8
    State  uint8
    H, V   int16
    Colors uint8
}

type drawState struct {
    descriptors map[uint8]frameDescriptor
    mobiles     map[uint8]frameMobile
    pictures    []framePicture
}

var (
    state      drawState
    initialState drawState
    stateMu    sync.Mutex
)

func resetDrawState() {
    stateMu.Lock()
    state = drawState{
        descriptors: make(map[uint8]frameDescriptor),
        mobiles:     make(map[uint8]frameMobile),
        pictures:    nil,
    }
    initialState = cloneDrawState(state)
    stateMu.Unlock()
}

func cloneDrawState(s drawState) drawState {
    out := drawState{
        descriptors: make(map[uint8]frameDescriptor, len(s.descriptors)),
        mobiles:     make(map[uint8]frameMobile, len(s.mobiles)),
        pictures:    append([]framePicture(nil), s.pictures...),
    }
    for k, v := range s.descriptors {
        out.descriptors[k] = v
    }
    for k, v := range s.mobiles {
        out.mobiles[k] = v
    }
    return out
}

// handleInfoText is a no-op for headless tests.
func handleInfoText(_ []byte) {}

// clImages plane provider used by movie parsing; keep nil in tests.
type planeProvider interface{ Plane(uint32) int }
var clImages planeProvider

// Players UI flags referenced by player.go; harmless in tests.
var (
    playersDirty        bool
    playersPersistDirty bool
)

// killNameTagCacheFor is a no-op in headless tests.
func killNameTagCacheFor(_ string) {}

// Minimal Character and saveCharacters for updatePlayerAppearance references.
type Character struct {
    Name   string
    PictID uint16
    Colors []byte
}

var characters []Character

func saveCharacters() {}

// consoleMessage is suppressed in headless tests.
func consoleMessage(_ string) {}
