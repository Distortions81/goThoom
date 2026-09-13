package main

import "testing"

func TestSessionMusicMultipartStateDoesNotCrossSessions(t *testing.T) {
	first := mustNewSession(1)
	second := mustNewSession(2)
	originalCapture := movieMusicIndexCapture
	var captured [][]tuneJob
	movieMusicIndexCapture = func(jobs []tuneJob) {
		captured = append(captured, append([]tuneJob(nil), jobs...))
	}
	t.Cleanup(func() { movieMusicIndexCapture = originalCapture })

	handleSessionMusicParams(first, MusicParams{Who: 7, Inst: 5, Notes: "c", Part: true})
	handleSessionMusicParams(second, MusicParams{Who: 7, Inst: 5, Notes: "e", Part: true})
	handleSessionMusicParams(first, MusicParams{Who: 7, Inst: 5, Notes: "d"})
	handleSessionMusicParams(second, MusicParams{Who: 7, Inst: 5, Notes: "f"})

	if len(captured) != 2 || len(captured[0]) != 1 || len(captured[1]) != 1 {
		t.Fatalf("captured groups = %#v, want two independent songs", captured)
	}
	firstNotes, secondNotes := captured[0][0].notes, captured[1][0].notes
	if len(firstNotes) != 2 || len(secondNotes) != 2 {
		t.Fatalf("assembled notes = %d/%d, want two notes per session", len(firstNotes), len(secondNotes))
	}
	if firstNotes[0].Key == secondNotes[0].Key {
		t.Fatalf("first notes share key %d; multipart songs crossed sessions", firstNotes[0].Key)
	}
}

func TestMusicSourceRoutesOnlySelectedSessionAndStopsOnChange(t *testing.T) {
	originalStop := stopMusicSourcePlayback
	appMusicSource.mu.Lock()
	originalSource := appMusicSource.source
	appMusicSource.source = 1
	appMusicSource.mu.Unlock()
	stops := 0
	stopMusicSourcePlayback = func() { stops++ }
	t.Cleanup(func() {
		stopMusicSourcePlayback = originalStop
		appMusicSource.mu.Lock()
		appMusicSource.source = originalSource
		appMusicSource.mu.Unlock()
	})

	first := mustNewSession(1)
	second := mustNewSession(2)
	firstPlays, secondPlays := 0, 0
	if !routeSessionMusic(first, func() { firstPlays++ }) {
		t.Fatal("selected session one was not routed")
	}
	if routeSessionMusic(second, func() { secondPlays++ }) {
		t.Fatal("unselected session two was routed")
	}
	if !selectMusicSource(2) || stops != 1 {
		t.Fatalf("source change stops = %d, want 1", stops)
	}
	if firstPlays != 1 || secondPlays != 0 {
		t.Fatalf("source change replayed music: first=%d second=%d", firstPlays, secondPlays)
	}
	if routeSessionMusic(first, func() { firstPlays++ }) {
		t.Fatal("old source still routed")
	}
	if !routeSessionMusic(second, func() { secondPlays++ }) || secondPlays != 1 {
		t.Fatal("new source did not route its next tune")
	}
	if !selectMusicSource(2) || stops != 1 {
		t.Fatalf("reselecting source stops = %d, want 1", stops)
	}
}
