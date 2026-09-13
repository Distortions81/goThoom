package main

import (
	"testing"
	"time"
)

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
	originalSource, originalGeneration := appMusicSource.source, appMusicSource.generation
	appMusicSource.source = 1
	appMusicSource.mu.Unlock()
	stops := 0
	stopMusicSourcePlayback = func() { stops++ }
	t.Cleanup(func() {
		stopMusicSourcePlayback = originalStop
		appMusicSource.mu.Lock()
		appMusicSource.source, appMusicSource.generation = originalSource, originalGeneration
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

func TestSessionMusicTracksAdvanceByWallTime(t *testing.T) {
	state := newSessionMusicState()
	started := time.Unix(100, 0)
	jobs := []tuneJob{{who: 7, notes: []Note{{Start: 0, Duration: 10 * time.Second}}}}
	state.startTracks(jobs, started)

	tracks := state.activeTracks(started.Add(2500 * time.Millisecond))
	if len(tracks) != 1 || !tracks[0].started.Equal(started) {
		t.Fatalf("active tracks = %+v", tracks)
	}
	wantFrame := int(2.5 * sampleRate)
	gotFrame := sessionMusicStartFrame(tracks[0].started, started.Add(2500*time.Millisecond))
	if gotFrame != wantFrame {
		t.Fatalf("resume frame = %d, want %d", gotFrame, wantFrame)
	}
	if tracks := state.activeTracks(started.Add(10 * time.Second)); len(tracks) != 0 {
		t.Fatalf("expired track remained active: %+v", tracks)
	}
}

func TestSessionMusicTrackStopsAreSessionLocal(t *testing.T) {
	first := newSessionMusicState()
	second := newSessionMusicState()
	started := time.Unix(100, 0)
	job := tuneJob{who: 7, notes: []Note{{Duration: 10 * time.Second}}}
	first.startTracks([]tuneJob{job}, started)
	second.startTracks([]tuneJob{job}, started)
	first.stopTracks(7)
	if tracks := first.activeTracks(started.Add(time.Second)); len(tracks) != 0 {
		t.Fatalf("stopped session retained tracks: %+v", tracks)
	}
	if tracks := second.activeTracks(started.Add(time.Second)); len(tracks) != 1 {
		t.Fatalf("other session lost its track: %+v", tracks)
	}
}

func TestSoundPlaybackRequestRejectsInactiveSession(t *testing.T) {
	oldSessions := appSessions
	manager := newSessionManager(mustNewSession(primarySessionID))
	open := [maxSessions]bool{}
	open[0], open[1] = true, true
	manager.restoreTabs(open, primarySessionID)
	appSessions = manager
	t.Cleanup(func() { appSessions = oldSessions })

	soundMu.Lock()
	request := soundPlaybackRequest{
		context:          audioContext,
		generation:       soundPlaybackGeneration,
		sourceGeneration: soundCacheGeneration,
		sourceSession:    primarySessionID,
	}
	soundMu.Unlock()
	if !soundPlaybackRequestCurrent(request) {
		t.Fatal("selected session sound request was rejected")
	}
	manager.mu.Lock()
	manager.selected = 2
	manager.mu.Unlock()
	if soundPlaybackRequestCurrent(request) {
		t.Fatal("inactive session sound request remained current")
	}
	request.sourceSession = 0
	if !soundPlaybackRequestCurrent(request) {
		t.Fatal("app-owned audio preview was tied to a session tab")
	}
}
