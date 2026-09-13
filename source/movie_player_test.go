package main

import (
	"net"
	"slices"
	"testing"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

func TestResetInterpolationClearsPositionHistory(t *testing.T) {
	primarySession.draw.mu.Lock()
	primarySession.draw.current = drawState{
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
	primarySession.draw.mu.Unlock()
	t.Cleanup(resetDrawState)

	resetInterpolation()

	primarySession.draw.mu.Lock()
	defer primarySession.draw.mu.Unlock()
	if len(primarySession.draw.current.prevMobiles) != 0 || len(primarySession.draw.current.prevDescs) != 0 || len(primarySession.draw.current.prevPictures) != 0 {
		t.Fatal("interpolation history was not cleared")
	}
	if primarySession.draw.current.picShiftX != 0 || primarySession.draw.current.picShiftY != 0 {
		t.Fatalf("picture shift = (%d, %d), want (0, 0)", primarySession.draw.current.picShiftX, primarySession.draw.current.picShiftY)
	}
	picture := primarySession.draw.current.pictures[0]
	if picture.PrevH != picture.H || picture.PrevV != picture.V || picture.Moving {
		t.Fatalf("picture interpolation was not reset: %+v", picture)
	}
	if !primarySession.draw.current.prevTime.Equal(primarySession.draw.current.curTime) {
		t.Fatalf("prevTime = %v, want curTime %v", primarySession.draw.current.prevTime, primarySession.draw.current.curTime)
	}
	if primarySession.draw.current.prevHP != primarySession.draw.current.hp || primarySession.draw.current.prevHPMax != primarySession.draw.current.hpMax ||
		primarySession.draw.current.prevSP != primarySession.draw.current.sp || primarySession.draw.current.prevSPMax != primarySession.draw.current.spMax ||
		primarySession.draw.current.prevBalance != primarySession.draw.current.balance || primarySession.draw.current.prevBalanceMax != primarySession.draw.current.balanceMax {
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

func TestMovieTimelineAlwaysUsesFiveUPS(t *testing.T) {
	originalRate := currentMovieMusicTempoRate()
	originalPlayingMovie, originalMovieMode := playingMovie, movieMode
	originalPaused := movieMusicPaused.Load()
	t.Cleanup(func() {
		setMovieMusicTempoRate(originalRate)
		playingMovie, movieMode = originalPlayingMovie, originalMovieMode
		movieMusicPaused.Store(originalPaused)
	})

	p := newMoviePlayer(nil, 10, nil)
	t.Cleanup(p.ticker.Stop)
	if p.baseFPS != movieRecordedUPS {
		t.Fatalf("movie timeline UPS = %d, want %d", p.baseFPS, movieRecordedUPS)
	}
	if got := currentMovieMusicTempoRate(); got != 2 {
		t.Fatalf("music tempo rate = %v, want 2 at 10 playback UPS", got)
	}
}

func TestMovieJumpUsesPendingScrubTargetOnFiveUPSTimeline(t *testing.T) {
	p := &moviePlayer{cur: 100, fps: 30, baseFPS: movieRecordedUPS}
	if got := p.skipTarget(5 * 1000); got != 125 {
		t.Fatalf("five-second jump target = %d, want 125", got)
	}

	p.seekTarget = 200
	p.seekPending = true
	if got := p.skipTarget(-5 * 1000); got != 175 {
		t.Fatalf("jump from pending scrub target = %d, want 175", got)
	}
}

func TestMoviePauseAndPlayControlMusicPauseState(t *testing.T) {
	originalPaused := movieMusicPaused.Load()
	musicPlayersMu.Lock()
	originalPlayers := musicPlayers
	musicPlayers = make(map[*audio.Player]musicTrack)
	musicPlayersMu.Unlock()
	t.Cleanup(func() {
		movieMusicPaused.Store(originalPaused)
		musicPlayersMu.Lock()
		musicPlayers = originalPlayers
		musicPlayersMu.Unlock()
	})

	p := &moviePlayer{playing: true, baseFPS: 5, fps: 5}
	p.pause()
	if p.playing || !movieMusicPaused.Load() {
		t.Fatal("movie pause did not pause movie music")
	}
	p.play()
	if !p.playing || movieMusicPaused.Load() {
		t.Fatal("movie play did not resume movie music")
	}
}

func TestMovieStopPausesWhenNoSeekIsActive(t *testing.T) {
	originalPaused := movieMusicPaused.Load()
	musicPlayersMu.Lock()
	originalPlayers := musicPlayers
	musicPlayers = make(map[*audio.Player]musicTrack)
	musicPlayersMu.Unlock()
	t.Cleanup(func() {
		movieMusicPaused.Store(originalPaused)
		musicPlayersMu.Lock()
		musicPlayers = originalPlayers
		musicPlayersMu.Unlock()
	})

	p := &moviePlayer{playing: true}
	p.stopSeek()
	if p.playing || !movieMusicPaused.Load() {
		t.Fatal("Stop did not pause playback when no seek was active")
	}
}

func TestMovieStopOnlyCancelsActiveSeek(t *testing.T) {
	p := &moviePlayer{playing: true, seekPending: true}
	p.stopSeek()
	if !p.seekStopped || p.seekEpoch != 1 {
		t.Fatal("Stop did not cancel the active seek")
	}
	if !p.playing {
		t.Fatal("stopping an active seek also paused playback")
	}
}

func TestMovieSeekFullRenderInterval(t *testing.T) {
	now := time.Unix(100, 0)
	if !movieSeekFullRenderDue(time.Time{}, now) {
		t.Fatal("initial seek render was not due")
	}
	if movieSeekFullRenderDue(now.Add(-movieSeekFullRenderInterval+time.Millisecond), now) {
		t.Fatal("seek render was due before half a second elapsed")
	}
	if !movieSeekFullRenderDue(now.Add(-movieSeekFullRenderInterval), now) {
		t.Fatal("seek render was not due after half a second")
	}
	if !movieSeekFullRenderDue(now.Add(time.Millisecond), now) {
		t.Fatal("seek render did not recover from a future timestamp")
	}
}

func TestMovieMusicIndexCapturesCompletedStarts(t *testing.T) {
	frames, err := parseMovie(movieFixturePath(t, "lore1.clMov"), baseVersion)
	if err != nil {
		t.Fatal(err)
	}
	events := indexMovieMusic(frames)
	for _, event := range events {
		if !event.stop && len(event.jobs) > 0 {
			if event.frame <= 0 || event.frame > len(frames) {
				t.Fatalf("music event frame = %d, want within movie", event.frame)
			}
			return
		}
	}
	t.Fatal("movie music index did not capture a completed music start")
}

func TestMovieMusicJobsDurationUsesLatestNoteEnd(t *testing.T) {
	jobs := []tuneJob{
		{notes: []Note{{Start: time.Second, Duration: 2 * time.Second}}},
		{notes: []Note{{Start: 4 * time.Second, Duration: time.Second}}},
	}
	if got := movieMusicJobsDuration(jobs); got != 5*time.Second {
		t.Fatalf("movie music duration = %v, want 5s", got)
	}
	if !movieMusicJobsActiveAt(jobs, 5*time.Second-time.Nanosecond) {
		t.Fatal("movie music was inactive before its natural end")
	}
	if movieMusicJobsActiveAt(jobs, 5*time.Second) {
		t.Fatal("movie music remained active at its natural end")
	}
}

func TestMovieMusicTimelineRangesFollowStartsStopsAndNaturalEnds(t *testing.T) {
	long := []Note{{Duration: 10 * time.Second}}
	events := []movieMusicEvent{
		{frame: 10, jobs: []tuneJob{{who: 1, notes: long}}},
		{frame: 20, stop: true, stopWho: 99},
		{frame: 40, jobs: []tuneJob{{who: 2, notes: long}}},
		{frame: 45, stop: true, stopWho: 2},
		{frame: 60, jobs: []tuneJob{{who: 3, notes: []Note{{Duration: time.Second}}}}},
	}
	ranges := movieMusicTimelineRanges(events, 100, movieRecordedUPS)
	want := [][2]float32{{10, 40}, {40, 45}, {60, 65}}
	if len(ranges) != len(want) {
		t.Fatalf("music timeline ranges = %#v, want %d ranges", ranges, len(want))
	}
	for i, bounds := range want {
		if ranges[i].Start != bounds[0] || ranges[i].End != bounds[1] {
			t.Errorf("music timeline range %d = [%v,%v], want [%v,%v]", i, ranges[i].Start, ranges[i].End, bounds[0], bounds[1])
		}
	}
}

func TestMovieMusicTimelineColorsRepeat(t *testing.T) {
	events := make([]movieMusicEvent, len(movieMusicTimelineColors)+1)
	for i := range events {
		events[i] = movieMusicEvent{
			frame: i * 2,
			jobs:  []tuneJob{{notes: []Note{{Duration: time.Second}}}},
		}
	}
	ranges := movieMusicTimelineRanges(events, 100, movieRecordedUPS)
	if len(ranges) != len(events) {
		t.Fatalf("music timeline ranges = %d, want %d", len(ranges), len(events))
	}
	if ranges[0].Color != ranges[len(movieMusicTimelineColors)].Color {
		t.Fatal("music timeline palette did not repeat")
	}
}

func TestMovieMusicTimelineAdjacentColorsAreDistinct(t *testing.T) {
	for i, current := range movieMusicTimelineColors {
		next := movieMusicTimelineColors[(i+1)%len(movieMusicTimelineColors)]
		currentHue, _, _, _ := current.HSVA()
		nextHue, _, _, _ := next.HSVA()
		distance := currentHue - nextHue
		if distance < 0 {
			distance = -distance
		}
		if distance > 180 {
			distance = 360 - distance
		}
		if distance < 90 {
			t.Errorf("timeline colors %d and %d are only %.1f degrees apart", i, (i+1)%len(movieMusicTimelineColors), distance)
		}
	}
}

func TestMovieMusicSeekUsesLatestStartKeyframe(t *testing.T) {
	longNote := []Note{{Duration: time.Hour}}
	events := []movieMusicEvent{
		{frame: 10, jobs: []tuneJob{{who: 1, notes: longNote}}},
		{frame: 20, jobs: []tuneJob{{who: 2, notes: longNote}, {who: 3, notes: longNote}}},
	}
	active := activeMovieMusicAt(events, 25, 1)
	if len(active) != 1 || active[0].frame != 20 || len(active[0].jobs) != 2 {
		t.Fatalf("active movie music = %#v, want only the synchronized event at frame 20", active)
	}
}

func TestConcertSeekAtFortySixFiftyThreeUsesOneMusicKeyframe(t *testing.T) {
	frames, err := parseMovie(movieFixturePath(t, "concert1.clMov"), baseVersion)
	if err != nil {
		t.Fatal(err)
	}
	const target = (46*60 + 53) * 5
	active := activeMovieMusicAt(indexMovieMusic(frames), target, 5)
	if len(active) != 1 {
		t.Fatalf("active concert music at 46:53 = %d tracks, want 1", len(active))
	}
	if active[0].frame != 13936 {
		t.Fatalf("active concert music frame = %d, want 13936", active[0].frame)
	}
}

func TestConcertSeekAtTwentyNineFortySevenUsesOneMusicKeyframe(t *testing.T) {
	frames, err := parseMovie(movieFixturePath(t, "concert1.clMov"), baseVersion)
	if err != nil {
		t.Fatal(err)
	}
	const target = (29*60 + 47) * movieRecordedUPS
	events := indexMovieMusic(frames)
	active := activeMovieMusicAt(events, target, movieRecordedUPS)
	if len(active) != 1 {
		t.Fatalf("active concert music at 29:47 = %d tracks, want 1", len(active))
	}
	if active[0].frame != 8668 {
		t.Fatalf("active concert music frame = %d, want 8668", active[0].frame)
	}
	if len(active[0].jobs) != 2 {
		t.Fatalf("active concert group has %d jobs, want synchronized duo", len(active[0].jobs))
	}
	wantPrograms := []int{instruments[5].program, instruments[1].program}
	for i, job := range active[0].jobs {
		if job.program != wantPrograms[i] {
			t.Errorf("duo job %d program = %d, want %d", i, job.program, wantPrograms[i])
		}
	}
	if duration := movieMusicJobsDuration(active[0].jobs); duration < 2*time.Minute || duration > 3*time.Minute {
		t.Fatalf("duo duration = %v, want about two minutes", duration)
	}
}

func TestMovieMusicRestoreGenerationInvalidatesOlderPreparation(t *testing.T) {
	p := &moviePlayer{}
	select {
	case <-p.restoreIndexedMusic(0, true):
	case <-time.After(time.Second):
		t.Fatal("empty music restore did not report ready")
	}
	first := p.musicRestoreGeneration.Load()
	p.restoreIndexedMusic(0, true)
	if got := p.musicRestoreGeneration.Load(); got != first+1 {
		t.Fatalf("restore generation = %d, want %d", got, first+1)
	}
}

func TestScaleMusicPartsFollowsPlaybackUPSWithoutChangingPitchData(t *testing.T) {
	original := []musicPart{{program: 46, notes: []Note{{Key: 60, Velocity: 100, Start: 2 * time.Second, Duration: time.Second}}}}
	scaled := scaleMusicParts(original, 2)
	if got := scaled[0].notes[0].Start; got != time.Second {
		t.Fatalf("scaled start = %v, want 1s", got)
	}
	if got := scaled[0].notes[0].Duration; got != 500*time.Millisecond {
		t.Fatalf("scaled duration = %v, want 500ms", got)
	}
	if scaled[0].notes[0].Key != original[0].notes[0].Key || scaled[0].notes[0].Velocity != original[0].notes[0].Velocity {
		t.Fatal("tempo scaling changed pitch or velocity data")
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
	originalPlayingMovie := playingMovie
	t.Cleanup(func() {
		tcpConn = originalConn
		clmov = originalCLMov
		playingMovie = originalPlayingMovie
	})

	tcpConn = client
	clmov = ""
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

func TestMovieSeekSuppressesTransientMessageCreation(t *testing.T) {
	originalSeeking := seekingMov
	originalSettings := gs
	originalGameWin := gameWin
	originalNotifications := notifications
	originalThinkMessages := thinkMessages
	t.Cleanup(func() {
		seekingMov = originalSeeking
		gs = originalSettings
		gameWin = originalGameWin
		notifications = originalNotifications
		thinkMessages = originalThinkMessages
	})

	seekingMov = true
	gs.Notifications = true
	gameWin = eui.NewWindow()
	notifications = nil
	thinkMessages = nil
	showNotification("replayed movie event")
	showThinkMessage("Hardia thinks, replayed movie event")

	if len(notifications) != 0 || len(thinkMessages) != 0 || len(gameWin.Contents) != 0 {
		t.Fatal("movie seek created an in-game notification or think message")
	}
}
