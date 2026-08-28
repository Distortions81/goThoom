package main

import (
	"net"
	"slices"
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

func TestMovieUPSValueIsFourDigits(t *testing.T) {
	for _, test := range []struct {
		ups  int
		want string
	}{
		{ups: 1, want: "0001"},
		{ups: 30, want: "0030"},
		{ups: 9999, want: "9999"},
		{ups: 12000, want: "9999"},
	} {
		if got := movieUPSValue(test.ups); got != test.want {
			t.Errorf("movieUPSValue(%d) = %q, want %q", test.ups, got, test.want)
		}
	}
}

func TestMovieCheckpointsStaySortedAcrossBackwardSeek(t *testing.T) {
	p := &moviePlayer{checkpoints: []movieCheckpoint{{idx: 0}, {idx: 300}, {idx: 600}, {idx: 900}}}

	p.addCheckpoint(movieCheckpoint{idx: 450})
	p.addCheckpoint(movieCheckpoint{idx: 300, night: movieNightState{level: 7}})

	got := make([]int, len(p.checkpoints))
	for i, cp := range p.checkpoints {
		got[i] = cp.idx
	}
	if want := []int{0, 300, 450, 600, 900}; !slices.Equal(got, want) {
		t.Fatalf("checkpoint indexes = %v, want %v", got, want)
	}
	if p.checkpoints[1].night.level != 7 {
		t.Fatal("checkpoint at an existing frame was not replaced")
	}
	if got := p.checkpointAtOrBefore(850).idx; got != 600 {
		t.Fatalf("checkpointAtOrBefore(850) = %d, want 600", got)
	}
}

func TestReserveMoviePlaybackRejectsServerConnection(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	originalConn := tcpConn
	originalCLMov := clmov
	originalLoginInProgress := loginInProgress
	originalPlayingMovie := playingMovie
	t.Cleanup(func() {
		tcpConn = originalConn
		clmov = originalCLMov
		loginInProgress = originalLoginInProgress
		playingMovie = originalPlayingMovie
	})

	tcpConn = client
	clmov = ""
	loginInProgress = false
	playingMovie = false
	if reserveMoviePlayback("connected.clMov") {
		t.Fatal("movie playback was reserved while connected to the server")
	}
	if clmov != "" {
		t.Fatalf("movie path changed while connected: %q", clmov)
	}

	tcpConn = nil
	if !reserveMoviePlayback("offline.clMov") {
		t.Fatal("movie playback was rejected while disconnected")
	}
	if clmov != "offline.clMov" {
		t.Fatalf("reserved movie path = %q, want offline.clMov", clmov)
	}
}
