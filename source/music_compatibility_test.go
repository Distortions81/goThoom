package main

import (
	"encoding/binary"
	"math"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	meltysynth "github.com/Distortions81/go-meltysynth/meltysynth"
)

func captureMusicCommands(t *testing.T) (*Session, *[][]tuneJob) {
	t.Helper()
	session := mustNewSession(primarySessionID)
	oldCapture, oldBlock := movieMusicIndexCapture, blockMusic
	blockMusic = false
	var captured [][]tuneJob
	movieMusicIndexCapture = func(jobs []tuneJob) {
		captured = append(captured, append([]tuneJob(nil), jobs...))
	}
	t.Cleanup(func() { movieMusicIndexCapture, blockMusic = oldCapture, oldBlock })
	return session, &captured
}

func TestMusicRegressionMultipartPreservesClassicSettings(t *testing.T) {
	for _, volume := range []string{"V50", "V0"} {
		t.Run(volume, func(t *testing.T) {
			session, captured := captureMusicCommands(t)
			for _, command := range []string{
				"/music/W7/P/I7/T90/" + volume + "/M/Nc",
				"/music/W7/P/I7/M/Nd",
				"/music/W7/P/I7/T180/V25/Ne",
			} {
				if !parseSessionMusicCommand(session, command, nil) {
					t.Fatal(command)
				}
			}
			if len(*captured) != 1 || len((*captured)[0]) != 1 {
				t.Fatalf("captures = %+v", *captured)
			}
			notes := (*captured)[0][0].notes
			velocity := 50
			if volume == "V0" {
				velocity = 0
			}
			if len(notes) != 3 || absDuration(notes[1].Start-time.Second/3) > time.Nanosecond || absDuration(notes[2].Start-2*time.Second/3) > time.Nanosecond {
				t.Fatalf("multipart tempo changed: %+v", notes)
			}
			for _, note := range notes {
				if note.Velocity != velocity {
					t.Fatalf("multipart volume changed: %+v", notes)
				}
			}
		})
	}
}

func TestMusicRegressionWithDependenciesIncludeEveryPerformer(t *testing.T) {
	for _, commands := range [][]string{
		{"/music/W1/P/I7/H2/Nc", "/music/W2/P/I7/Ne"},
		{"/music/W2/P/I7/H1/Ne", "/music/W1/P/I7/Nc"},
		{"/music/W1/P/I7/H2/Nc", "/music/W2/P/I7/H3/Ne", "/music/W3/P/I7/Ng"},
		{"/music/W1/P/I7/H2/M/Nc", "/music/W2/P/I7/M/Ne", "/music/W1/P/I7/Nd", "/music/W2/P/I7/Nf"},
	} {
		t.Run(commands[0], func(t *testing.T) {
			session, captured := captureMusicCommands(t)
			for i, command := range commands {
				if !parseSessionMusicCommand(session, command, nil) {
					t.Fatal(command)
				}
				if i < len(commands)-1 && len(*captured) != 0 {
					t.Fatalf("ensemble started early: %+v", *captured)
				}
			}
			want := 2
			if len(commands) == 3 {
				want = 3
			}
			if len(*captured) != 1 || len((*captured)[0]) != want {
				t.Fatalf("ensemble captures = %+v", *captured)
			}
			for i, job := range (*captured)[0] {
				if job.who != i+1 {
					t.Fatalf("ensemble performers = %+v", (*captured)[0])
				}
			}
		})
	}
}

func TestMusicRegressionHeaderStopsBeforeNotation(t *testing.T) {
	session, captured := captureMusicCommands(t)
	if !parseSessionMusicCommand(session, "/music/W7/P/I7/Nc</M/W42/H99/T90/V50/I1/>d", nil) {
		t.Fatal("unhandled music command")
	}
	if len(*captured) != 1 || len((*captured)[0]) != 1 {
		t.Fatalf("comment changed command handling: %+v", *captured)
	}
	job := (*captured)[0][0]
	if job.who != 7 || job.program != 16 || len(job.notes) != 2 || job.notes[1].Start != 250*time.Millisecond || job.notes[0].Velocity != 100 {
		t.Fatalf("notation comment changed parameters: %+v", job)
	}
}

func TestMusicRegressionIncomingVelocityAndTempo(t *testing.T) {
	for _, test := range []struct {
		command  string
		velocity int
		start    time.Duration
	}{
		{"/music/W7/P/I7/Ncd", 100, 250 * time.Millisecond},
		{"/music/W7/P/I7/V0/Ncd", 0, 250 * time.Millisecond},
		{"/music/W7/P/I7/V50/Ncd", 50, 250 * time.Millisecond},
		{"/music/W7/P/I7/V100/Ncd", 100, 250 * time.Millisecond},
		{"/music/W7/P/I7/V101/Ncd", 100, 250 * time.Millisecond},
		{"/music/W7/P/I7/T1/Ncd", 100, 250 * time.Millisecond},
		{"/music/W7/P/I7/T59/Ncd", 100, 250 * time.Millisecond},
		{"/music/W7/P/I7/T181/Ncd", 100, 250 * time.Millisecond},
		{"/music/W7/P/I7/T999/Ncd", 100, 250 * time.Millisecond},
		{"/music/W7/P/I7/T60/Ncd", 100, 500 * time.Millisecond},
		{"/music/W7/P/I7/T180/Ncd", 100, time.Duration(100) * time.Second / 600},
	} {
		t.Run(test.command, func(t *testing.T) {
			session, captured := captureMusicCommands(t)
			if !parseSessionMusicCommand(session, test.command, nil) {
				t.Fatal("unhandled music command")
			}
			if len(*captured) != 1 || len((*captured)[0][0].notes) != 2 {
				t.Fatalf("captures = %+v", *captured)
			}
			notes := (*captured)[0][0].notes
			if notes[0].Velocity != test.velocity || absDuration(notes[1].Start-test.start) > time.Nanosecond {
				t.Fatalf("notes = %+v", notes)
			}
			if test.velocity == 0 {
				if events, end := buildSongEvents(16, notes); len(events) != 0 || end < int((notes[1].Start+notes[1].Duration).Seconds()*sampleRate) {
					t.Fatalf("silent timeline changed: events=%+v end=%d", events, end)
				}
			}
		})
	}
}

func TestMusicRegressionSettingsChangeBeforePlaybackGoroutine(t *testing.T) {
	oldGS, oldFocus := gs, focusMuted
	gs = gsdef
	gs.Mute, gs.Music, focusMuted = false, true, false
	gs.MasterVolume, gs.MusicVolume = 1, 1
	t.Cleanup(func() { gs, focusMuted = oldGS, oldFocus })
	installMusicRegressionSynth(t, func() synthesizer {
		t.Error("muted preparation created a synthesizer")
		return &bufferedStreamSynth{}
	})
	parts := []musicPart{{program: 16, notes: []Note{{Key: 48, Velocity: 100, Duration: time.Second}}}}
	settings := currentMusicPlaybackSettings()
	reservation := reserveMusicPlayback([]int{71})
	gs.Mute = true
	restartMusicWithCurrentSettings()
	if err := playReservedMusicGroup(audioContext, parts, []int{71}, nil, nil, settings, 0, nil, reservation); err != nil {
		t.Fatal(err)
	}
}

func TestMusicRegressionPCMSaturatesWithoutPolarityWrap(t *testing.T) {
	left := []float32{-4, -1, -.5, 0, .5, 1, 4, float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())}
	pcm := mixPCMChunk(left, append([]float32(nil), left...), false)
	want := []int16{-32767, -32767, -16383, 0, 16383, 32767, 32767, 32767, -32767, 0}
	for i, expected := range want {
		for channel := range 2 {
			if got := int16(binary.LittleEndian.Uint16(pcm[4*i+2*channel:])); got != expected {
				t.Errorf("sample %d channel %d = %d, want %d", i, channel, got, expected)
			}
		}
	}
}

// Playback tests keep the real audio context at TestMain's zero output volume,
// but inject a synthesizer to control preparation and observe MIDI events.
func installMusicRegressionSynth(t *testing.T, factory func() synthesizer) {
	t.Helper()
	oldSynth, oldMeasure := newSynthesizer, measureProgramGainForCache
	oldFont, oldSettings := sfntCached, synthSettings
	programGainMu.Lock()
	oldGain := programGainCache
	programGainCache = make(map[programGainKey]*programGainCacheEntry)
	programGainMu.Unlock()
	setupSynthOnce = sync.Once{}
	setupSynthOnce.Do(func() {})
	sfntCached, synthSettings = &meltysynth.SoundFont{}, newSynthSettings()
	measureProgramGainForCache = func(*meltysynth.SoundFont, int) float32 { return 1 }
	newSynthesizer = func(*meltysynth.SoundFont, *meltysynth.SynthesizerSettings) (synthesizer, error) {
		return factory(), nil
	}
	t.Cleanup(func() {
		newSynthesizer, measureProgramGainForCache = oldSynth, oldMeasure
		setupSynthOnce = sync.Once{}
		sfntCached, synthSettings = oldFont, oldSettings
		programGainMu.Lock()
		programGainCache = oldGain
		programGainMu.Unlock()
	})
}

type musicRegressionEvent struct {
	channel, key int32
	on           bool
	frame        int
}
type musicRegressionSynth struct {
	frame    int
	events   []musicRegressionEvent
	programs []recordedMIDIMessage
}

func (s *musicRegressionSynth) ProcessMidiMessage(ch, cmd, d1, d2 int32) {
	s.programs = append(s.programs, recordedMIDIMessage{ch, cmd, d1, d2})
}
func (s *musicRegressionSynth) NoteOn(ch, key, _ int32) {
	s.events = append(s.events, musicRegressionEvent{ch, key, true, s.frame})
}
func (s *musicRegressionSynth) NoteOff(ch, key int32) {
	s.events = append(s.events, musicRegressionEvent{ch, key, false, s.frame})
}
func (s *musicRegressionSynth) Render(left, _ []float32) { s.frame += len(left) }

func TestMusicRegressionRepeatedPitchKeepsBothNotesAndReleases(t *testing.T) {
	for _, notation := range []string{"[c]8p2c2p8", "[c]8p2[c]2p8", "[cc]8p8"} {
		t.Run(notation, func(t *testing.T) {
			syn := &musicRegressionSynth{}
			installMusicRegressionSynth(t, func() synthesizer { return syn })
			notes, parseErr := parseClassicTune(notation, instruments[7], 120, 100)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			renderer, err := newSongRenderer(16, notes)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := renderer.render(renderer.totalSamples); err != nil {
				t.Fatal(err)
			}
			var ons, offs []musicRegressionEvent
			for _, event := range syn.events {
				if event.on {
					ons = append(ons, event)
				} else {
					offs = append(offs, event)
				}
			}
			if len(ons) != 2 || len(offs) != 2 || ons[0].channel == ons[1].channel {
				t.Fatalf("overlapping notes did not get independent voices: %+v", syn.events)
			}
			for i, note := range notes {
				start := int(note.Start.Nanoseconds() * sampleRate / int64(time.Second))
				end := int((note.Start + note.Duration).Nanoseconds() * sampleRate / int64(time.Second))
				if ons[i].frame > start || start-ons[i].frame >= block {
					t.Fatalf("late/missing note-on: %+v", ons[i])
				}
				found := false
				for _, off := range offs {
					if off.channel == ons[i].channel {
						found = true
						if off.frame < end || off.frame-end > block {
							t.Fatalf("note released at %d, expected around %d", off.frame, end)
						}
					}
				}
				if !found {
					t.Fatalf("missing release for %+v", ons[i])
				}
			}
			if len(syn.programs) != 2 || syn.programs[1].channel != 1 || syn.programs[1].data1 != 16 {
				t.Fatalf("overlap channel has wrong instrument: %+v", syn.programs)
			}
		})
	}
}

func TestMusicRegressionRepeatedPitchNeverUsesPercussionChannel(t *testing.T) {
	syn := &musicRegressionSynth{}
	installMusicRegressionSynth(t, func() synthesizer { return syn })
	notes, parseErr := parseClassicTune(strings.Repeat("[c]8", 20)+"p8", instruments[7], 120, 100)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	renderer, err := newSongRenderer(16, notes)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := renderer.render(renderer.totalSamples); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, event := range syn.events {
		if event.channel == 9 || event.channel < 0 || event.channel > 15 {
			t.Fatalf("invalid melodic channel: %+v", event)
		}
		if event.on {
			count++
		}
	}
	if count != 20 {
		t.Fatalf("note-ons = %d, want every finite note", count)
	}
}

func TestMusicRegressionInstalledSoundFontEnsembleOutput(t *testing.T) {
	path := soundFontForTest(t)
	font, err := loadSoundFont(path)
	if err != nil {
		t.Fatal(err)
	}
	measure := measureProgramGainForCache
	installMusicRegressionSynth(t, nil)
	sfntCached = font
	measureProgramGainForCache = measure
	newSynthesizer = func(font *meltysynth.SoundFont, settings *meltysynth.SynthesizerSettings) (synthesizer, error) {
		return meltysynth.NewSynthesizer(font, settings)
	}
	job := makeTuneJob(1, 2, 120, 100, "[\\ceg=ceg+ceg]8p8", false)
	if job.parseErr != nil {
		t.Fatal(job.parseErr)
	}
	part := musicPart{program: job.program, notes: job.notes}
	renderer, err := newMusicGroupRenderer([]musicPart{part, part, part})
	if err != nil {
		t.Fatal(err)
	}
	left, right, err := renderer.render(sampleRate)
	if err != nil {
		t.Fatal(err)
	}
	pcm := mixPCMChunk(left, right, false)
	overloaded := 0
	for i := range left {
		for channel, value := range []float32{left[i], right[i]} {
			got := int16(binary.LittleEndian.Uint16(pcm[4*i+channel*2:]))
			if value > 1 {
				overloaded++
				if got != 32767 {
					t.Fatalf("positive ensemble overload wrapped to %d", got)
				}
			}
			if value < -1 {
				overloaded++
				if got != -32767 {
					t.Fatalf("negative ensemble overload wrapped to %d", got)
				}
			}
		}
	}
	t.Logf("saturated %d overloaded channel samples from installed SoundFont", overloaded)
}

func waitMusicRegression(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("music condition did not become ready")
}

func TestMusicRegressionStopAndTabSwitchCancelPreparingPlayback(t *testing.T) {
	for _, action := range []string{"scoped stop", "global stop", "switch tab", "stop before goroutine"} {
		t.Run(action, func(t *testing.T) {
			usePrimaryMusicSourceForTest(t)
			oldSessions := appSessions
			appSessions = nil
			t.Cleanup(func() { appSessions = oldSessions })
			blocked := &blockingStreamSynth{started: make(chan struct{}), release: make(chan struct{})}
			installMusicRegressionSynth(t, func() synthesizer { return blocked })
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(blocked.release) }) }
			t.Cleanup(release)
			parts := []musicPart{{program: 16, notes: []Note{{Key: 48, Velocity: 100, Duration: time.Second}}}}
			reservation := reserveMusicPlayback([]int{71})
			settings := musicPlaybackSettings{enabled: true, bufferSeconds: 1}
			done := make(chan error, 1)
			registered := false
			prepared := func() {
				musicPlayersMu.Lock()
				defer musicPlayersMu.Unlock()
				for _, track := range musicPlayers {
					if _, ok := track.whos[71]; ok {
						registered = true
					}
				}
			}
			if action == "stop before goroutine" {
				stopMusicFor(71)
			}
			go func() {
				done <- playReservedMusicGroup(audioContext, parts, []int{71}, prepared, nil, settings, 0, nil, reservation)
			}()
			if action != "stop before goroutine" {
				select {
				case <-blocked.started:
				case <-time.After(3 * time.Second):
					t.Fatal("prebuffer did not begin")
				}
				switch action {
				case "scoped stop":
					stopMusicFor(71)
				case "global stop":
					stopAllMusic()
				case "switch tab":
					selectMusicSource(2)
				}
			}
			release()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				stopAllMusic()
				t.Fatal("cancelled preparation did not return")
			}
			if registered {
				t.Fatal("cancelled song was registered for playback")
			}
		})
	}
}

func TestMusicRegressionScopedStopDuringEnsemblePreparationKeepsPartner(t *testing.T) {
	blocked := &blockingStreamSynth{started: make(chan struct{}), release: make(chan struct{})}
	var created int
	installMusicRegressionSynth(t, func() synthesizer {
		created++
		if created == 1 {
			return blocked
		}
		return &bufferedStreamSynth{}
	})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(blocked.release) }) }
	t.Cleanup(release)
	parts := []musicPart{{program: 16, notes: []Note{{Key: 48, Velocity: 100, Duration: time.Second}}}, {program: 73, notes: []Note{{Key: 60, Velocity: 100, Duration: time.Second}}}}
	reservation := reserveMusicPlayback([]int{71, 72})
	ready := make(chan struct{})
	done := make(chan error, 1)
	start := make(chan struct{})
	go func() {
		done <- playReservedMusicGroup(audioContext, parts, []int{71, 72}, func() { close(ready) }, start, musicPlaybackSettings{enabled: true, bufferSeconds: 1}, 0, nil, reservation)
	}()
	t.Cleanup(func() {
		release()
		stopAllMusic()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("playback did not clean up")
		}
	})
	select {
	case <-blocked.started:
	case <-time.After(3 * time.Second):
		t.Fatal("prebuffer did not begin")
	}
	stopMusicFor(71)
	release()
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("survivor did not prepare")
	}
	musicPlayersMu.Lock()
	defer musicPlayersMu.Unlock()
	for _, track := range musicPlayers {
		if _, ok := track.whos[72]; ok {
			if _, stopped := track.whos[71]; stopped || len(track.parts) != 1 || track.parts[0].program != 73 {
				t.Fatalf("remaining track=%+v", track)
			}
			return
		}
	}
	t.Fatal("scoped stop discarded partner during preparation")
}

func TestMusicRegressionScopedStopRebuildsPlayingPartnerAtPosition(t *testing.T) {
	installMusicRegressionSynth(t, func() synthesizer { return &bufferedStreamSynth{} })
	oldMovie, oldPaused := movieMode, movieMusicPaused.Load()
	movieMode = true
	movieMusicPaused.Store(true)
	t.Cleanup(func() { movieMode = oldMovie; movieMusicPaused.Store(oldPaused) })
	parts := []musicPart{{program: 16, notes: []Note{{Key: 48, Velocity: 100, Duration: 5 * time.Second}}}, {program: 73, notes: []Note{{Key: 60, Velocity: 100, Duration: 5 * time.Second}}}}
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- playMusicGroupWithSettingsAtFrame(audioContext, parts, []int{71, 72}, func() { close(ready) }, nil, musicPlaybackSettings{enabled: true, bufferSeconds: 1}, sampleRate)
	}()
	t.Cleanup(func() {
		stopAllMusic()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("original playback did not clean up")
		}
		waitMusicRegression(t, func() bool {
			musicPlayersMu.Lock()
			defer musicPlayersMu.Unlock()
			for _, track := range musicPlayers {
				if _, ok := track.whos[72]; ok {
					return false
				}
			}
			return true
		})
	})
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("ensemble did not prepare")
	}
	stopMusicFor(71)
	waitMusicRegression(t, func() bool {
		musicPlayersMu.Lock()
		defer musicPlayersMu.Unlock()
		for _, track := range musicPlayers {
			if _, ok := track.whos[72]; ok && !track.stream.isClosed() {
				return len(track.whos) == 1 && len(track.parts) == 1 && track.parts[0].program == 73 && track.startFrame == sampleRate
			}
		}
		return false
	})
}

func TestMusicRegressionScopedStopPreservesLiveAndMoviePartners(t *testing.T) {
	jobs := []tuneJob{{who: 71, notes: []Note{{Duration: 10 * time.Second}}}, {who: 72, notes: []Note{{Duration: 8 * time.Second}}}}
	now := time.Now()
	state := newSessionMusicState()
	state.startTracks(jobs, now)
	state.stopTracks(71)
	tracks := state.activeTracks(now.Add(time.Second))
	if len(tracks) != 1 || len(tracks[0].jobs) != 1 || tracks[0].jobs[0].who != 72 || !tracks[0].started.Equal(now) {
		t.Fatalf("remaining live tracks=%+v", tracks)
	}
	events := []movieMusicEvent{{frame: 1, jobs: jobs}, {frame: 3, stop: true, stopWho: 71}}
	active := activeMovieMusicAt(events, 4, 1)
	if len(active) != 1 || len(active[0].jobs) != 1 || active[0].jobs[0].who != 72 || active[0].frame != 1 {
		t.Fatalf("remaining movie tracks=%+v", active)
	}
	if len(events[0].jobs) != 2 {
		t.Fatal("movie index was mutated by scoped stop")
	}
	ranges := movieMusicTimelineRanges(events, 20, 1)
	if len(ranges) != 1 || ranges[0].Start != 1 || ranges[0].End != 9 {
		t.Fatalf("remaining movie timeline=%+v", ranges)
	}
	if !slices.Equal([]int{jobs[0].who, jobs[1].who}, []int{71, 72}) {
		t.Fatal("live source jobs were mutated")
	}
}

func TestMusicRegressionScopedStopDuringTabRestorationKeepsPartner(t *testing.T) {
	usePrimaryMusicSourceForTest(t)
	oldGS, oldFocus, oldBlock := gs, focusMuted, blockMusic
	oldMovie, oldPaused := movieMode, movieMusicPaused.Load()
	gs = gsdef
	gs.Mute, gs.Music, focusMuted, blockMusic = false, true, false, false
	gs.MasterVolume, gs.MusicVolume = 1, 1
	movieMode = true
	movieMusicPaused.Store(true)
	t.Cleanup(func() {
		gs, focusMuted, blockMusic = oldGS, oldFocus, oldBlock
		movieMode = oldMovie
		movieMusicPaused.Store(oldPaused)
	})
	blocked := &blockingStreamSynth{started: make(chan struct{}), release: make(chan struct{})}
	created := 0
	installMusicRegressionSynth(t, func() synthesizer {
		created++
		if created == 1 {
			return blocked
		}
		return &bufferedStreamSynth{}
	})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(blocked.release) }) }
	t.Cleanup(func() {
		release()
		stopAllMusic()
		waitMusicRegression(t, func() bool {
			musicPlayersMu.Lock()
			defer musicPlayersMu.Unlock()
			return len(musicPlayers) == 0
		})
	})
	session := mustNewSession(primarySessionID)
	now := time.Now()
	session.music.startTracks([]tuneJob{
		{who: 71, program: 16, notes: []Note{{Key: 48, Velocity: 100, Duration: 5 * time.Second}}},
		{who: 72, program: 73, notes: []Note{{Key: 60, Velocity: 100, Duration: 5 * time.Second}}},
	}, now.Add(-time.Second))
	appMusicSource.mu.RLock()
	generation := appMusicSource.generation
	appMusicSource.mu.RUnlock()
	restoreSessionMusic(session, generation, now)
	select {
	case <-blocked.started:
	case <-time.After(3 * time.Second):
		t.Fatal("restoration did not begin")
	}
	handleSessionMusicParams(session, MusicParams{Stop: true, Who: 71})
	release()
	waitMusicRegression(t, func() bool {
		musicPlayersMu.Lock()
		defer musicPlayersMu.Unlock()
		for _, track := range musicPlayers {
			if _, ok := track.whos[72]; ok && !track.stream.isClosed() {
				return len(track.whos) == 1 && track.startFrame == sampleRate
			}
		}
		return false
	})
}

func TestMusicRegressionSettingsChangeDuringPreparation(t *testing.T) {
	for _, mute := range []bool{false, true} {
		t.Run(map[bool]string{false: "restart", true: "mute"}[mute], func(t *testing.T) {
			oldGS, oldFocus := gs, focusMuted
			gs = gsdef
			gs.Mute, gs.Music, focusMuted = false, true, false
			gs.MasterVolume, gs.MusicVolume = 1, 1
			t.Cleanup(func() { gs, focusMuted = oldGS, oldFocus })
			blocked := &blockingStreamSynth{started: make(chan struct{}), release: make(chan struct{})}
			created := 0
			installMusicRegressionSynth(t, func() synthesizer {
				created++
				if created == 1 {
					return blocked
				}
				return &bufferedStreamSynth{}
			})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(blocked.release) }) }
			ready, start, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			parts := []musicPart{{program: 16, notes: []Note{{Key: 48, Velocity: 100, Duration: time.Second}}}}
			settings := currentMusicPlaybackSettings()
			reservation := reserveMusicPlayback([]int{71})
			go func() {
				done <- playReservedMusicGroup(audioContext, parts, []int{71}, func() { close(ready) }, start, settings, 0, nil, reservation)
			}()
			t.Cleanup(func() {
				release()
				stopAllMusic()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("playback did not clean up")
				}
			})
			select {
			case <-blocked.started:
			case <-time.After(3 * time.Second):
				t.Fatal("preparation did not begin")
			}
			gs.Mute = mute
			restartMusicWithCurrentSettings()
			release()
			select {
			case <-ready:
			case <-time.After(3 * time.Second):
				t.Fatal("updated preparation did not finish")
			}
			musicPlayersMu.Lock()
			defer musicPlayersMu.Unlock()
			registered := false
			for _, track := range musicPlayers {
				if _, ok := track.whos[71]; ok && !track.stream.isClosed() {
					registered = true
				}
			}
			if registered == mute {
				t.Fatalf("registered=%t after mute=%t", registered, mute)
			}
		})
	}
}
