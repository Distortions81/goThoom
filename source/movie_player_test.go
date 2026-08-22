package main

import (
	"testing"
	"time"
)

func TestResetInterpolationClearsPositionHistory(t *testing.T) {
	stateMu.Lock()
	state = drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: map[uint8]frameMobile{1: {Index: 1, H: 10, V: 20}},
		prevDescs:   map[uint8]frameDescriptor{1: {Index: 1}},
		pictures: []framePicture{{
			PictID: 1,
			H:      100,
			V:      200,
			PrevH:  12,
			PrevV:  34,
			Moving: true,
		}},
		prevPictures: []framePicture{{PictID: 1, H: 12, V: 34}},
		picShiftX:    88,
		picShiftY:    -44,
		prevTime:     time.Unix(1, 0),
		curTime:      time.Unix(2, 0),
		hp:           75,
		hpMax:        100,
		sp:           40,
		spMax:        60,
		balance:      25,
		balanceMax:   30,
	}
	stateMu.Unlock()
	t.Cleanup(resetDrawState)

	resetInterpolation()

	stateMu.Lock()
	defer stateMu.Unlock()
	if len(state.prevMobiles) != 0 || len(state.prevDescs) != 0 || len(state.prevPictures) != 0 {
		t.Fatal("interpolation history was not cleared")
	}
	if state.picShiftX != 0 || state.picShiftY != 0 {
		t.Fatalf("picture shift = (%d, %d), want (0, 0)", state.picShiftX, state.picShiftY)
	}
	picture := state.pictures[0]
	if picture.PrevH != picture.H || picture.PrevV != picture.V || picture.Moving {
		t.Fatalf("picture interpolation was not reset: %+v", picture)
	}
	if !state.prevTime.Equal(state.curTime) {
		t.Fatalf("prevTime = %v, want curTime %v", state.prevTime, state.curTime)
	}
	if state.prevHP != state.hp || state.prevHPMax != state.hpMax ||
		state.prevSP != state.sp || state.prevSPMax != state.spMax ||
		state.prevBalance != state.balance || state.prevBalanceMax != state.balanceMax {
		t.Fatal("status interpolation history was not reset to current values")
	}
}
