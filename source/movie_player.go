package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"gothoom/eui"

	"github.com/hako/durafmt"
)

var (
	shortUnits, _        = durafmt.DefaultUnitsCoder.Decode("y:yrs,wk:wks,d:d,h:h,m:m,s:s,ms:ms,us:us")
	playingMovie         bool
	movieMode            bool
	movieWin             *eui.WindowData
	movieDropped         int
	moviePlaybackSession *Session
	moviePlaybackPlayer  *moviePlayer
	moviePreviousSession SessionID
)

func beginMoviePlaybackSession(label string) (*Session, bool) {
	if appSessions == nil || moviePlaybackSession != nil {
		return nil, false
	}
	previous := appSessions.selectedID()
	session, ok := appSessions.addSession()
	if !ok {
		return nil, false
	}
	session.setCharacterName(label)
	moviePlaybackSession = session
	moviePreviousSession = previous
	refreshViewportWorkspace()
	return session, true
}

func endMoviePlaybackSession(session *Session) {
	if session == nil || moviePlaybackSession != session || appSessions == nil {
		return
	}
	// Retain classic's renderer history when the temporary playback tab closes.
	// Keep live sessions independent; the primary session owns movie bootstrap.
	shadow := session.night.snapshot().shadow
	primarySession.night.mu.Lock()
	primarySession.night.shadow = shadow
	primarySession.night.generation++
	primarySession.night.mu.Unlock()
	wasSelected := appSessions.selectedID() == session.ID()
	previous := moviePreviousSession
	moviePlaybackSession = nil
	moviePreviousSession = 0
	appSessions.closeSession(session.ID())
	if wasSelected {
		if _, ok := appSessions.session(previous); ok {
			appSessions.selectSession(previous)
		}
	}
	refreshViewportWorkspace()
}

func setMovieControlIcon(button *eui.ItemData, name, fallback string) {
	setMaterialIconOnly(button, name, fallback)
}

// movieCheckpoint captures the draw state after processing a frame. idx
// matches the number of processed frames (the next frame to play).
type movieCheckpoint struct {
	idx   int
	state drawState
	night movieNightState
}

// movieMusicEvent is a completed bard start or stop indexed by the playback
// frame immediately after the message that caused it was processed.
type movieMusicEvent struct {
	frame   int
	jobs    []tuneJob
	stopWho int
	stop    bool
}

type activeMovieMusic struct {
	frame int
	jobs  []tuneJob
}

var movieMusicTimelineColors = []eui.Color{
	eui.NewColor(239, 83, 80, 210),
	eui.NewColor(66, 165, 245, 210),
	eui.NewColor(255, 238, 88, 210),
	eui.NewColor(126, 87, 194, 210),
	eui.NewColor(255, 167, 38, 210),
	eui.NewColor(38, 198, 218, 210),
	eui.NewColor(236, 64, 122, 210),
	eui.NewColor(102, 187, 106, 210),
}

type movieNightState struct {
	baseLevel       int
	azimuth         int
	cloudy          bool
	flags           uint
	level           int
	shadows         int
	oldAzimuth      int
	redshift        float64
	startOfTwilight int
	shadow          shadowCasterState
}

func captureMovieNightState() movieNightState {
	return captureMovieNightStateForSession(primarySession)
}

func captureMovieNightStateForSession(session *Session) movieNightState {
	if session == nil {
		return movieNightState{}
	}
	night := session.night
	night.mu.Lock()
	defer night.mu.Unlock()
	return movieNightState{
		baseLevel:       night.BaseLevel,
		azimuth:         night.Azimuth,
		cloudy:          night.Cloudy,
		flags:           night.Flags,
		level:           night.Level,
		shadows:         night.Shadows,
		oldAzimuth:      night.oldAzimuth,
		redshift:        night.redshift,
		startOfTwilight: night.startOfTwilight,
		shadow:          night.shadow,
	}
}

func restoreMovieNightState(n movieNightState) {
	restoreMovieNightStateForSession(primarySession, n)
}

func restoreMovieNightStateForSession(session *Session, n movieNightState) {
	if session == nil {
		return
	}
	night := session.night
	night.mu.Lock()
	night.BaseLevel = n.baseLevel
	night.Azimuth = n.azimuth
	night.Cloudy = n.cloudy
	night.Flags = n.flags
	night.Level = n.level
	night.Shadows = n.shadows
	night.oldAzimuth = n.oldAzimuth
	night.redshift = n.redshift
	night.startOfTwilight = n.startOfTwilight
	night.shadow = n.shadow
	night.generation++
	night.mu.Unlock()
}

// checkpointInterval determines how often checkpoints are recorded during
// playback. Larger intervals reduce memory usage at the cost of slower seek
// times.
const checkpointInterval = 300
const movieSeekFullRenderInterval = 500 * time.Millisecond
const movieControlButtonHeight = 38

// Clan Lord movies record one frame for each of the game's fixed 5 UPS.
// Playback UPS may change, but frame indexes and indexed music positions must
// always be interpreted on this recorded timeline.
const movieRecordedUPS = 5

// moviePlayer manages clMov playback with basic controls.
type moviePlayer struct {
	session *Session
	frames  []movieFrame
	fps     int
	baseFPS int
	cur     int // number of frames processed
	playing bool
	repeat  bool
	ticker  *time.Ticker
	cancel  context.CancelFunc
	looped  chan struct{}
	// resetOnNextDraw removes interpolation history after the first complete
	// draw at movie start or after looping back to frame zero.
	resetOnNextDraw bool
	// musicNeedsRestore is set when a paused seek or UPS change invalidates the
	// paused audio stream. Ordinary pause/play can resume the existing player.
	musicNeedsRestore bool
	// musicRestoreGeneration invalidates SoundFont streams that an older seek
	// is still pre-rendering. Without it, rapid scrubbing can let every earlier
	// seek begin playing several seconds after the final seek.
	musicRestoreGeneration atomic.Uint64

	// Slider motion can generate many values while an expensive seek is still
	// rebuilding state. Keep only the most recent value and seek to it next.
	seekMu      sync.Mutex
	seekTarget  int
	seekPending bool
	seekEpoch   uint64
	seekStopped bool

	checkpoints []movieCheckpoint
	music       []movieMusicEvent

	slider     *eui.ItemData
	curLabel   *eui.ItemData
	totalLabel *eui.ItemData
	fpsLabel   *eui.ItemData
	playButton *eui.ItemData
}

func (p *moviePlayer) playbackSession() *Session {
	if p != nil && p.session != nil {
		return p.session
	}
	return primarySession
}

// seedMoviePlaybackSession transfers the header state produced by parseMovie
// into a temporary movie session. The movie parser still uses the legacy
// primary draw state while reading picture and mobile tables; subsequent
// playback frames must start from those tables in the session they update.
func seedMoviePlaybackSession(session *Session) {
	if session == nil || session == primarySession {
		return
	}
	primarySession.draw.mu.Lock()
	initial := cloneDrawState(primarySession.draw.initial)
	primarySession.draw.mu.Unlock()

	session.draw.mu.Lock()
	session.draw.current = cloneDrawState(initial)
	session.draw.initial = initial
	session.draw.frame = 0
	prepareSessionRenderCacheLocked(session)
	session.draw.mu.Unlock()
	// Header lighting belongs to the movie just as its picture/mobile tables do.
	// Preserve it in the playback session and its frame-zero checkpoint.
	restoreMovieNightStateForSession(session, captureMovieNightState())
}

func markMovieMusicSourceInactive(id SessionID) {
	p := moviePlaybackPlayer
	if p == nil || p.playbackSession().ID() != id {
		return
	}
	p.musicRestoreGeneration.Add(1)
	p.musicNeedsRestore = true
}

func restoreMovieMusicSource(session *Session) bool {
	p := moviePlaybackPlayer
	if p == nil || session == nil || p.playbackSession() != session {
		return false
	}
	if !p.playing {
		p.musicNeedsRestore = true
		return true
	}
	p.musicNeedsRestore = false
	p.restoreIndexedMusic(p.cur, true)
	return true
}

func (p *moviePlayer) closePlaybackTabAtEnd() {
	session := p.playbackSession()
	if session != moviePlaybackSession {
		return
	}
	dispatchMainThread(func() {
		if session == moviePlaybackSession && movieWin != nil {
			movieWin.Close()
		}
	})
}

var movieMusicTempoMu sync.RWMutex
var movieMusicTempoRate = 1.0

func setMovieMusicTempoRate(rate float64) {
	if rate <= 0 {
		rate = 1
	}
	movieMusicTempoMu.Lock()
	movieMusicTempoRate = rate
	movieMusicTempoMu.Unlock()
}

func currentMovieMusicTempoRate() float64 {
	movieMusicTempoMu.RLock()
	defer movieMusicTempoMu.RUnlock()
	return movieMusicTempoRate
}

// indexMovieMusic parses only the music payloads once at movie load time.
// The tune assembler emits complete jobs, so a seek need not replay messages
// from the beginning just to recover /part and /with groups.
func indexMovieMusic(session *Session, frames []movieFrame) []movieMusicEvent {
	if session == nil {
		session = primarySession
	}
	previousCapture, previousStop := movieMusicIndexCapture, movieMusicIndexStop
	previousBlockMusic := blockMusic
	previousMusicCommandNow := musicCommandNow
	session.music.mu.Lock()
	previousPending := session.music.pendingByID
	session.music.pendingByID = make(map[int]*pendingSong)
	session.music.mu.Unlock()
	defer func() {
		movieMusicIndexCapture, movieMusicIndexStop = previousCapture, previousStop
		blockMusic = previousBlockMusic
		musicCommandNow = previousMusicCommandNow
		session.music.mu.Lock()
		session.music.pendingByID = previousPending
		session.music.mu.Unlock()
	}()

	blockMusic = false
	var events []movieMusicEvent
	currentFrame := 0
	movieMusicIndexStart := time.Unix(0, 0)
	musicCommandNow = func() time.Time {
		return movieMusicIndexStart.Add(time.Duration(currentFrame) * time.Second / movieRecordedUPS)
	}
	movieMusicIndexCapture = func(jobs []tuneJob) {
		copyJobs := append([]tuneJob(nil), jobs...)
		events = append(events, movieMusicEvent{frame: currentFrame + 1, jobs: copyJobs})
	}
	movieMusicIndexStop = func(who int) {
		events = append(events, movieMusicEvent{frame: currentFrame + 1, stop: true, stopWho: who})
	}
	for frame, movieFrame := range frames {
		currentFrame = frame
		payload := movieFrame.data
		for start := 0; start < len(payload); {
			index := bytes.Index(payload[start:], []byte("/music/"))
			if index < 0 {
				break
			}
			index += start
			_ = parseSessionMusicCommand(session, "", payload[index:])
			start = index + len("/music/")
		}
	}
	return events
}

// checkpointAtOrBefore returns the closest cached state that does not pass
// idx. checkpoints is kept sorted by addCheckpoint, so seek work is bounded by
// the distance from that checkpoint rather than by the order of earlier seeks.
func (p *moviePlayer) checkpointAtOrBefore(idx int) movieCheckpoint {
	i := sort.Search(len(p.checkpoints), func(i int) bool {
		return p.checkpoints[i].idx > idx
	})
	if i == 0 {
		return p.checkpoints[0]
	}
	return p.checkpoints[i-1]
}

// addCheckpoint inserts or replaces a checkpoint while preserving frame
// order. Seeking backward must not leave a low-frame checkpoint at the end of
// the slice, since subsequent forward seeks need the nearest cached state.
func (p *moviePlayer) addCheckpoint(cp movieCheckpoint) {
	i := sort.Search(len(p.checkpoints), func(i int) bool {
		return p.checkpoints[i].idx >= cp.idx
	})
	if i < len(p.checkpoints) && p.checkpoints[i].idx == cp.idx {
		p.checkpoints[i] = cp
		return
	}
	p.checkpoints = append(p.checkpoints, movieCheckpoint{})
	copy(p.checkpoints[i+1:], p.checkpoints[i:])
	p.checkpoints[i] = cp
}

func newMoviePlayer(session *Session, frames []movieFrame, fps int, cancel context.CancelFunc) *moviePlayer {
	if session == nil {
		session = primarySession
	}
	seedMoviePlaybackSession(session)
	setSessionInterpFPS(session, fps)
	session.timing.setInterval(time.Second / time.Duration(fps))
	playingMovie = true
	movieMode = true
	movieMusicPaused.Store(false)
	setMovieMusicTempoRate(float64(fps) / movieRecordedUPS)
	// Do not interpolate the very first frame of playback.
	// Ensure prevTime == curTime and clear prior history so sprites
	// don't lerp from zeroed positions on start.
	resetSessionInterpolation(session)
	suppressInterpOnce = true
	p := &moviePlayer{
		session:         session,
		frames:          frames,
		fps:             fps,
		baseFPS:         movieRecordedUPS,
		playing:         true,
		resetOnNextDraw: true,
		ticker:          time.NewTicker(time.Second / time.Duration(fps)),
		cancel:          cancel,
		looped:          make(chan struct{}, 1),
		checkpoints:     []movieCheckpoint{{idx: 0, state: cloneDrawState(session.draw.initial), night: captureMovieNightStateForSession(session)}},
		music:           indexMovieMusic(session, frames),
	}
	if session == moviePlaybackSession {
		moviePlaybackPlayer = p
	}
	return p
}

var seekLock sync.Mutex
var seekingMov bool
var movieSeekRenderGeneration atomic.Uint64
var movieSeekRenderedGeneration atomic.Uint64
var movieSeekRenderAcknowledged = make(chan struct{}, 1)
var movieMusicPaused atomic.Bool

func movieSeekFullRenderDue(lastRender, now time.Time) bool {
	return lastRender.IsZero() || now.Before(lastRender) || now.Sub(lastRender) >= movieSeekFullRenderInterval
}

func publishMovieSeekRender() uint64 {
	return movieSeekRenderGeneration.Add(1)
}

func acknowledgeMovieSeekRender(generation uint64) {
	movieSeekRenderedGeneration.Store(generation)
	select {
	case movieSeekRenderAcknowledged <- struct{}{}:
	default:
	}
}

// waitForMovieSeekRender prevents the seek worker from advancing the shared
// state while Draw is copying and rendering a published cache. Headless work
// has no renderer to wait for.
func waitForMovieSeekRender(generation uint64) {
	if !uiReady || gameWin == nil {
		return
	}
	for movieSeekRenderedGeneration.Load() < generation {
		select {
		case <-movieSeekRenderAcknowledged:
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// makePlaybackWindow creates the playback control window.
func (p *moviePlayer) makePlaybackWindow() {
	win := eui.NewWindow()
	movieWin = win
	win.Title = "Movie Controls"
	win.BackgroundTransparency = 0.4
	win.ShowDragbar = true
	win.Theme.Window.DragbarColor = eui.Color{R: 96, G: 96, B: 96}
	win.DragbarSpacing = 5
	win.Closable = true
	win.Resizable = false
	win.AutoSize = true
	win.NoScroll = true
	win.SetZone(eui.HZoneCenter, eui.VZoneBottom)
	win.SetZoneOffset(eui.Point{Y: -164})

	flow := eui.NewColumn()

	// Time slider flow
	tFlow := eui.NewRow()

	p.curLabel, _ = eui.NewText()
	p.curLabel.Text = "0s"
	p.curLabel.Size = eui.Point{X: 55, Y: 24}
	p.curLabel.FontSize = 10
	tFlow.AddItem(p.curLabel)

	max := float32(len(p.frames))
	var events *eui.EventHandler
	p.slider, events = eui.NewSlider()
	p.slider.MinValue = 0
	p.slider.MaxValue = max
	p.slider.Size = eui.Point{X: 600, Y: 24}
	p.slider.IntOnly = true
	p.slider.SliderKnobColor = eui.NewColor(255, 255, 255, 255)
	p.slider.Ranges = movieMusicTimelineRanges(p.music, len(p.frames), p.baseFPS)
	p.slider.SetTooltip("Seek through the movie. Colored bands show indexed bard music.")
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			p.requestSeek(int(ev.Value))
		}
	}
	tFlow.AddItem(p.slider)

	totalDur := time.Duration(len(p.frames)) * time.Second / time.Duration(p.fps)
	totalDur = totalDur.Round(time.Second)
	p.totalLabel, _ = eui.NewText()
	p.totalLabel.Text = durafmt.Parse(totalDur).LimitFirstN(2).Format(shortUnits)
	p.totalLabel.Size = eui.Point{X: 55, Y: 24}
	p.totalLabel.FontSize = 10
	tFlow.AddItem(p.totalLabel)

	flow.AddItem(tFlow)

	// Button flow
	bFlow := eui.NewRow()

	backb, backbEv := eui.NewButton()
	setMovieControlIcon(backb, "replay_30", "<<<")
	backb.Size = eui.Point{X: 40, Y: movieControlButtonHeight}
	backb.SetTooltip("Skip back 30s")
	backbEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBackMilli(30 * 1000)
		}
	}
	bFlow.AddItem(backb)

	back, backEv := eui.NewButton()
	setMovieControlIcon(back, "replay_5", "<<")
	back.Size = eui.Point{X: 40, Y: movieControlButtonHeight}
	back.SetTooltip("Skip back 5s")
	backEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBackMilli(5 * 1000)
		}
	}
	bFlow.AddItem(back)

	play, playEv := eui.NewButton()
	setMovieControlIcon(play, "pause", "Pause")
	play.SetTooltip("Toggle playback")
	play.Size = eui.Point{X: 80, Y: movieControlButtonHeight}
	p.playButton = play
	changePlayButton(p, p.playButton)
	playEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if p.playing {
				p.pause()
			} else {
				p.play()
			}
			changePlayButton(p, p.playButton)
		}
	}
	bFlow.AddItem(play)

	stopSeek, stopSeekEv := eui.NewButton()
	setMovieControlIcon(stopSeek, "stop", "Stop Seek")
	stopSeek.SetTooltip("Stop seeking, or pause playback when no seek is active")
	stopSeek.Size = eui.Point{X: 80, Y: movieControlButtonHeight}
	stopSeekEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.stopSeek()
		}
	}
	bFlow.AddItem(stopSeek)

	forwardb, fwdbEv := eui.NewButton()
	setMovieControlIcon(forwardb, "forward_5", ">>")
	forwardb.Size = eui.Point{X: 40, Y: movieControlButtonHeight}
	forwardb.SetTooltip("Skip forward 5s")
	fwdbEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipForwardMilli(5 * 1000)
		}
	}
	bFlow.AddItem(forwardb)

	forward, fwdEv := eui.NewButton()
	setMovieControlIcon(forward, "forward_30", ">>>")
	forward.Size = eui.Point{X: 40, Y: movieControlButtonHeight}
	forward.SetTooltip("Skip forward 30s")
	fwdEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipForwardMilli(30 * 1000)
		}
	}
	bFlow.AddItem(forward)

	spacer, _ := eui.NewText()
	spacer.Text = ""
	spacer.Size = eui.Point{X: 20, Y: movieControlButtonHeight}
	bFlow.AddItem(spacer)

	half, halfEv := eui.NewButton()
	setMovieControlIcon(half, "fast_rewind_3", "--")
	half.Size = eui.Point{X: 40, Y: movieControlButtonHeight}
	half.SetTooltip("Half speed")
	halfEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps / 2)
		}
	}
	bFlow.AddItem(half)

	dec, decEv := eui.NewButton()
	setMovieControlIcon(dec, "fast_rewind", "-")
	dec.Size = eui.Point{X: 40, Y: movieControlButtonHeight}
	dec.SetTooltip("Slow down")
	decEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps - 1)
		}
	}
	bFlow.AddItem(dec)

	reset, resetEv := eui.NewButton()
	setMovieControlIcon(reset, "restart_alt", "RESET")
	reset.SetTooltip("Reset playback speed")
	reset.Size = eui.Point{X: 80, Y: movieControlButtonHeight}
	resetEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.baseFPS)
		}
	}
	bFlow.AddItem(reset)

	inc, incEv := eui.NewButton()
	setMovieControlIcon(inc, "fast_forward", "+")
	inc.Size = eui.Point{X: 40, Y: movieControlButtonHeight}
	inc.SetTooltip("Speed up")
	incEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps + 1)
		}
	}
	bFlow.AddItem(inc)

	dbl, dblEv := eui.NewButton()
	setMovieControlIcon(dbl, "fast_forward_3", "++")
	dbl.Size = eui.Point{X: 40, Y: movieControlButtonHeight}
	dbl.SetTooltip("Double speed")
	dblEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps * 2)
		}
	}
	bFlow.AddItem(dbl)

	exitBtn, exitEv := eui.NewButton()
	setMovieControlIcon(exitBtn, "exit_to_app", "Exit")
	exitBtn.Size = eui.Point{X: 80, Y: movieControlButtonHeight}
	exitEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			eui.ShowPopup(
				"Exit Movie",
				"Stop playback and close the movie session tab?",
				[]eui.PopupButton{{Text: "Cancel"}, {Text: "Exit", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
					if movieWin != nil {
						movieWin.Close()
					}
				}}},
			)
		}
	}
	bFlow.AddItem(exitBtn)

	fpsInfo, _ := eui.NewText()
	fpsInfo.Text = movieUPSValue(p.fps)
	fpsInfo.SetTooltip("Playback updates per second")
	fpsInfo.Size = eui.Point{X: 50, Y: movieControlButtonHeight}
	fpsInfo.FontSize = 15
	fpsInfo.Alignment = eui.ALIGN_CENTER
	p.fpsLabel = fpsInfo
	bFlow.AddItem(fpsInfo)

	flow.AddItem(bFlow)

	win.AddItem(flow)

	// Recompute window dimensions now that all controls are present
	win.Refresh()
	// Add and open the fully populated window
	// Playback controls must be open regardless of their persisted state. In
	// particular, startup window restoration must not see a stale closed state
	// and immediately invoke this window's cancellation callback.
	gs.MovieWindow.Open = true
	win.AddWindow(false)
	applyWindowState(win, &gs.MovieWindow)
	win.MarkOpen()

	// Closing the controls ends playback and removes its temporary session tab.
	win.OnClose = func() {
		log.Printf("movie playback stopped: movie controls closed")
		// Pause and stop ticker
		p.pause()
		p.musicRestoreGeneration.Add(1)
		if p.ticker != nil {
			p.ticker.Stop()
		}
		// Stop any active sounds
		stopAllSounds()
		stopAllTTS()
		stopAllMusic()
		movieMusicPaused.Store(false)
		if moviePlaybackPlayer == p {
			moviePlaybackPlayer = nil
		}
		// Cancel playback loop
		if p.cancel != nil {
			p.cancel()
		}
		playingMovie = false
		movieMode = false
		updateRecordButton()
		// Clear any players loaded during playback so GT_Players.json
		// is unaffected.
		playersMu.Lock()
		players = make(map[string]*Player)
		playersMu.Unlock()
		loadPlayersPersist()
		updatePlayersWindow()
		playersPersistDirty = false
		playersDirty = false
		p.playbackSession().night.reset()
		// Clear the selected movie path and remove its temporary session tab.
		clmov = ""
		pcapPath = ""
	}

	p.updateUI()
	updateRecordButton()
}

func changePlayButton(p *moviePlayer, play *eui.ItemData) {
	if p.playing {
		setMovieControlIcon(play, "pause", "Pause")
	} else {
		setMovieControlIcon(play, "play_arrow", "Play")
	}
	play.Dirty = true
}

func (p *moviePlayer) run(ctx context.Context) {
	<-gameStarted
	for {
		select {
		case <-ctx.Done():
			p.ticker.Stop()
			playingMovie = false
			movieMode = false
			return
		case <-p.ticker.C:
			if p.playing {
				p.step()
			}
		}
	}
}

func (p *moviePlayer) step() {
	if len(p.frames) == 0 {
		p.playing = false
		playingMovie = false
		updateRecordButton()
		p.updateUI()
		p.closePlaybackTabAtEnd()
		return
	}

	if p.cur >= len(p.frames) {
		if p.repeat {
			p.seek(0)
			p.signalLooped()
			if p.cur >= len(p.frames) {
				p.playing = false
				playingMovie = false
				updateRecordButton()
				p.updateUI()
				return
			}
		} else {
			p.playing = false
			playingMovie = false
			updateRecordButton()
			p.updateUI()
			p.closePlaybackTabAtEnd()
			return
		}
	}
	m := p.frames[p.cur]
	movieDropped = p.playbackSession().frames.updateCounters(m.index)
	if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
		handleSessionDrawState(p.playbackSession(), m.data, true)
		if p.resetOnNextDraw {
			resetSessionInterpolation(p.playbackSession())
			suppressInterpOnce = true
			p.resetOnNextDraw = false
		}
	} else {
		// Advance the logical frame counter even when this movie frame
		// does not contain a draw-state update so time-based effects
		// (e.g., bubble expiration) progress correctly during playback.
		p.playbackSession().draw.frame++
	}
	maybeDecodeSessionMessage(p.playbackSession(), m.data)
	p.cur++
	if p.cur%checkpointInterval == 0 {
		night := captureMovieNightStateForSession(p.playbackSession())
		p.playbackSession().draw.mu.Lock()
		cp := movieCheckpoint{idx: p.cur, state: cloneDrawState(p.playbackSession().draw.current), night: night}
		p.playbackSession().draw.mu.Unlock()
		p.addCheckpoint(cp)
	}
	if p.cur >= len(p.frames) {
		if p.repeat {
			p.seek(0)
			p.signalLooped()
		} else {
			p.playing = false
			playingMovie = false
			updateRecordButton()
			p.closePlaybackTabAtEnd()
		}
	}
	p.updateUI()
}

func (p *moviePlayer) signalLooped() {
	select {
	case p.looped <- struct{}{}:
	default:
	}
}

func (p *moviePlayer) updateUI() {
	pendingTarget, seekPending := p.pendingSeekTarget()
	if p.slider != nil && !seekPending {
		p.slider.Value = float32(p.cur)
		p.slider.Dirty = true
	}
	if p.curLabel != nil {
		if seekPending {
			p.setCurrentTimeLabel(pendingTarget)
		} else {
			p.setCurrentTimeLabel(p.cur)
		}
	}
	if p.totalLabel != nil {
		totalDur := time.Duration(len(p.frames)) * time.Second / time.Duration(p.baseFPS)
		totalDur = totalDur.Round(time.Second)
		p.totalLabel.Text = durafmt.Parse(totalDur).LimitFirstN(2).Format(shortUnits)
		p.totalLabel.Dirty = true
	}

	if p.fpsLabel != nil {
		p.fpsLabel.Text = movieUPSValue(p.fps)
		p.fpsLabel.Dirty = true
	}

	if p.playButton != nil {
		changePlayButton(p, p.playButton)
	}
}

func movieUPSValue(ups int) string {
	if ups < 0 {
		ups = 0
	} else if ups > 9999 {
		ups = 9999
	}
	return fmt.Sprintf("%04d", ups)
}

func (p *moviePlayer) setCurrentTimeLabel(idx int) {
	if p.curLabel == nil {
		return
	}
	if idx < 0 {
		idx = 0
	} else if idx > len(p.frames) {
		idx = len(p.frames)
	}
	fps := p.baseFPS
	if fps < 1 {
		fps = p.fps
	}
	d := time.Duration(idx) * time.Second / time.Duration(fps)
	p.curLabel.Text = durafmt.Parse(d.Round(time.Second)).LimitFirstN(2).Format(shortUnits)
	p.curLabel.Dirty = true
}

func (p *moviePlayer) hasPendingSeek() bool {
	_, pending := p.pendingSeekTarget()
	return pending
}

func (p *moviePlayer) pendingSeekTarget() (int, bool) {
	p.seekMu.Lock()
	defer p.seekMu.Unlock()
	return p.seekTarget, p.seekPending
}

// requestSeek coalesces drag events. The final slider position is never lost
// merely because an earlier seek is still rebuilding the movie state.
func (p *moviePlayer) requestSeek(idx int) {
	p.seekMu.Lock()
	p.seekTarget = idx
	p.seekEpoch++
	p.seekStopped = false
	p.setCurrentTimeLabel(idx)
	if p.slider != nil {
		p.slider.Value = float32(idx)
		p.slider.Dirty = true
	}
	if p.seekPending {
		p.seekMu.Unlock()
		return
	}
	p.seekPending = true
	p.seekMu.Unlock()

	go func() {
		for {
			p.seekMu.Lock()
			target := p.seekTarget
			epoch := p.seekEpoch
			p.seekMu.Unlock()

			seekLock.Lock()
			p.seekWithCancel(target, func() bool {
				p.seekMu.Lock()
				defer p.seekMu.Unlock()
				return p.seekStopped || p.seekEpoch != epoch
			})
			seekLock.Unlock()

			p.seekMu.Lock()
			if p.seekStopped {
				p.seekPending = false
				p.seekMu.Unlock()
				p.updateUI()
				return
			}
			if p.seekTarget == target {
				p.seekPending = false
				p.seekMu.Unlock()
				p.updateUI()
				return
			}
			p.seekMu.Unlock()
		}
	}()
}

func (p *moviePlayer) stopSeek() {
	p.seekMu.Lock()
	stopped := p.seekPending
	if p.seekPending {
		p.seekStopped = true
		p.seekEpoch++
	}
	p.seekMu.Unlock()
	if stopped {
		return
	}
	p.pause()
	if p.playButton != nil {
		changePlayButton(p, p.playButton)
	}
}

func (p *moviePlayer) setFPS(fps int) {
	if fps < 1 {
		fps = 1
	} else if fps > 9999 {
		fps = 9999
	}
	p.fps = fps
	p.ticker.Reset(time.Second / time.Duration(p.fps))
	p.playbackSession().timing.setInterval(time.Second / time.Duration(p.fps))
	setSessionInterpFPS(p.playbackSession(), p.fps)
	setMovieMusicTempoRate(float64(p.fps) / float64(p.baseFPS))
	if p.playing {
		p.restoreIndexedMusic(p.cur, true)
		p.musicNeedsRestore = false
	} else {
		// A paused stream was rendered for the previous UPS. Rebuild it from
		// the indexed movie position when Play is pressed.
		p.musicRestoreGeneration.Add(1)
		stopAllMusic()
		p.musicNeedsRestore = true
	}
	p.updateUI()
}

// restoreIndexedMusic rebuilds each active movie bard track at idx and returns
// a channel that closes once every replacement stream is ready. The seek frame
// is measured in the movie's recorded UPS; note timing is then scaled to the
// current playback UPS so video and music stay synchronized at every speed.
func (p *moviePlayer) restoreIndexedMusic(idx int, play bool) <-chan struct{} {
	ready := make(chan struct{})
	generation := p.musicRestoreGeneration.Add(1)
	if !play || len(p.music) == 0 || p.baseFPS < 1 {
		close(ready)
		return ready
	}
	if idx < 0 {
		idx = 0
	}
	active := activeMovieMusicAt(p.music, idx, p.baseFPS)
	stopAllMusic()
	soundMu.Lock()
	context := audioContext
	soundMu.Unlock()
	rate := currentMovieMusicTempoRate()
	settings := currentMusicPlaybackSettings()
	if context == nil || !settings.enabled || len(active) == 0 {
		close(ready)
		return ready
	}
	var prepared sync.WaitGroup
	prepared.Add(len(active))
	go func() {
		prepared.Wait()
		close(ready)
	}()
	for _, track := range active {
		elapsed := time.Duration(idx-track.frame) * time.Second / time.Duration(p.baseFPS)
		scaledElapsed := time.Duration(float64(elapsed) / rate)
		startFrame := int(scaledElapsed.Seconds() * sampleRate)
		parts := make([]musicPart, 0, len(track.jobs))
		whos := make([]int, 0, len(track.jobs))
		for _, job := range track.jobs {
			parts = append(parts, musicPart{program: job.program, notes: job.notes})
			whos = append(whos, job.who)
		}
		parts = scaleMusicParts(parts, rate)
		go func(parts []musicPart, whos []int, startFrame int) {
			var preparedOnce sync.Once
			markPrepared := func() { preparedOnce.Do(prepared.Done) }
			defer markPrepared()
			valid := func() bool {
				return p.musicRestoreGeneration.Load() == generation && sessionIsMusicSource(p.playbackSession())
			}
			if err := playMusicGroupWithSettingsAtFrameIf(context, parts, whos, markPrepared, nil, settings, startFrame, valid); err != nil {
				log.Printf("resume movie music: %v", err)
			}
		}(parts, whos, startFrame)
	}
	return ready
}

// activeMovieMusicAt treats each completed start as a movie music keyframe.
// Concurrent bards in a synchronized performance are already represented as
// jobs within that event. This avoids carrying an older song across a later
// start when its decoded tune duration is longer than the recorded performance.
func activeMovieMusicAt(events []movieMusicEvent, idx, baseFPS int) []activeMovieMusic {
	if idx < 0 || baseFPS < 1 {
		return nil
	}
	active := make([]activeMovieMusic, 0, 1)
	for _, event := range events {
		if event.frame > idx {
			break
		}
		if event.stop {
			if event.stopWho == 0 {
				active = active[:0]
				continue
			}
			kept := active[:0]
			for _, track := range active {
				matches := false
				for _, job := range track.jobs {
					if job.who == event.stopWho {
						matches = true
						break
					}
				}
				if !matches {
					kept = append(kept, track)
				}
			}
			active = kept
			continue
		}
		active = append(active[:0], activeMovieMusic{frame: event.frame, jobs: event.jobs})
	}
	// Stop events are not required for songs that have simply reached their
	// natural end. Drop those here instead of creating a zero-length player for
	// every song that ever appeared before the seek target.
	kept := active[:0]
	for _, track := range active {
		elapsed := time.Duration(idx-track.frame) * time.Second / time.Duration(baseFPS)
		if movieMusicJobsActiveAt(track.jobs, elapsed) {
			kept = append(kept, track)
		}
	}
	return kept
}

func movieMusicJobsDuration(jobs []tuneJob) time.Duration {
	var duration time.Duration
	for _, job := range jobs {
		for _, note := range job.notes {
			if end := note.Start + note.Duration; end > duration {
				duration = end
			}
		}
	}
	return duration
}

func movieMusicJobsActiveAt(jobs []tuneJob, elapsed time.Duration) bool {
	return elapsed >= 0 && elapsed < movieMusicJobsDuration(jobs)
}

func movieMusicTimelineRanges(events []movieMusicEvent, totalFrames, ups int) []eui.SliderRange {
	if totalFrames <= 0 || ups <= 0 || len(movieMusicTimelineColors) == 0 {
		return nil
	}
	type musicRange struct {
		start int
		end   int
		jobs  []tuneJob
	}
	ranges := make([]musicRange, 0)
	active := -1
	for _, event := range events {
		frame := min(max(event.frame, 0), totalFrames)
		if !event.stop {
			if active >= 0 {
				ranges[active].end = min(ranges[active].end, frame)
			}
			duration := movieMusicJobsDuration(event.jobs)
			durationFrames := int((duration.Nanoseconds()*int64(ups) + int64(time.Second) - 1) / int64(time.Second))
			ranges = append(ranges, musicRange{
				start: frame,
				end:   min(frame+durationFrames, totalFrames),
				jobs:  event.jobs,
			})
			active = len(ranges) - 1
			continue
		}
		if active < 0 || frame >= ranges[active].end {
			continue
		}
		stopsActive := event.stopWho == 0
		for _, job := range ranges[active].jobs {
			stopsActive = stopsActive || job.who == event.stopWho
		}
		if stopsActive {
			ranges[active].end = frame
			active = -1
		}
	}

	highlights := make([]eui.SliderRange, 0, len(ranges))
	for _, music := range ranges {
		if music.end <= music.start {
			continue
		}
		highlights = append(highlights, eui.SliderRange{
			Start: float32(music.start),
			End:   float32(music.end),
			Color: movieMusicTimelineColors[len(highlights)%len(movieMusicTimelineColors)],
		})
	}
	return highlights
}

func (p *moviePlayer) play() {
	if p.playing {
		return
	}
	p.playing = true
	movieMusicPaused.Store(false)
	if p.musicNeedsRestore {
		stopAllMusic()
		p.musicNeedsRestore = false
		p.restoreIndexedMusic(p.cur, true)
		return
	}
	resumeAllMusic()
}

func (p *moviePlayer) pause() {
	p.playing = false
	movieMusicPaused.Store(true)
	pauseAllMusic()
}

func (p *moviePlayer) skipBackMilli(milli int) {
	p.requestSeek(p.skipTarget(-milli))
}

func (p *moviePlayer) skipForwardMilli(milli int) {
	p.requestSeek(p.skipTarget(milli))
}

// skipTarget keeps jump buttons on the same queued timeline as slider seeks.
// If a scrub is still rebuilding, the visible pending position is the user's
// current position and therefore the correct base for the jump.
func (p *moviePlayer) skipTarget(milli int) int {
	target, pending := p.pendingSeekTarget()
	if !pending {
		target = p.cur
	}
	return target + milli*p.baseFPS/1000
}

func (p *moviePlayer) seek(idx int) {
	p.seekWithCancel(idx, nil)
}

// seekWithCancel rebuilds movie state up to idx. A drag update or Stop Seek
// can end the rebuild early, retaining the latest fully processed frame.
func (p *moviePlayer) seekWithCancel(idx int, cancelled func() bool) {
	seekingMov = true
	defer func() { seekingMov = false }()
	p.musicRestoreGeneration.Add(1)

	// Stop any currently playing sounds so scrubbing is silent.
	stopAllSounds()
	stopAllTTS()
	stopAllMusic()
	previousBlockSound := blockSound
	previousBlockBubbles := blockBubbles
	previousBlockTTS := blockTTS
	previousBlockMusic := blockMusic
	blockSound = true
	blockBubbles = true
	blockTTS = true
	blockMusic = true
	defer func() {
		blockSound = previousBlockSound
		blockBubbles = previousBlockBubbles
		blockTTS = previousBlockTTS
		blockMusic = previousBlockMusic
	}()

	if idx < 0 {
		idx = 0
	}
	if idx > len(p.frames) {
		idx = len(p.frames)
	}
	wasPlaying := p.playing
	p.playing = false

	cp := p.checkpointAtOrBefore(idx)

	p.playbackSession().draw.mu.Lock()
	p.playbackSession().draw.current = cloneDrawState(cp.state)
	// Ensure render caches reflect the restored checkpoint state. The cache
	// will be rebuilt again if additional frames are parsed.
	prepareSessionRenderCacheLocked(p.playbackSession())
	p.playbackSession().draw.mu.Unlock()
	restoreMovieNightStateForSession(p.playbackSession(), cp.night)
	firstRender := publishMovieSeekRender()
	waitForMovieSeekRender(firstRender)
	p.playbackSession().draw.frame = cp.idx
	lastFullRender := time.Now()

	for i := cp.idx; i < idx; i++ {
		if cancelled != nil && cancelled() {
			idx = i
			break
		}
		m := p.frames[i]
		movieDropped = p.playbackSession().frames.updateCounters(m.index)
		if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
			now := time.Now()
			buildFullRender := i == idx-1 || movieSeekFullRenderDue(lastFullRender, now)
			if handleSessionDrawState(p.playbackSession(), m.data, buildFullRender) && buildFullRender {
				lastFullRender = now
				generation := publishMovieSeekRender()
				waitForMovieSeekRender(generation)
			}
		} else {
			// Keep timeline consistent during scrubbing when frames
			// without draw-state are encountered.
			p.playbackSession().draw.frame++
		}
		maybeDecodeSessionMessage(p.playbackSession(), m.data)
		if p.playbackSession().draw.frame%checkpointInterval == 0 {
			night := captureMovieNightStateForSession(p.playbackSession())
			p.playbackSession().draw.mu.Lock()
			snap := movieCheckpoint{idx: p.playbackSession().draw.frame, state: cloneDrawState(p.playbackSession().draw.current), night: night}
			p.playbackSession().draw.mu.Unlock()
			p.addCheckpoint(snap)
		}
	}
	night := captureMovieNightStateForSession(p.playbackSession())
	p.playbackSession().draw.mu.Lock()
	// Cancellation or a run of non-draw frames may end between periodic
	// publishes. Always leave a complete final cache for the first post-seek
	// frame rather than exposing the partially rebuilt state.
	prepareSessionRenderCacheLocked(p.playbackSession())
	snap := movieCheckpoint{idx: idx, state: cloneDrawState(p.playbackSession().draw.current), night: night}
	p.playbackSession().draw.mu.Unlock()
	publishMovieSeekRender()
	p.addCheckpoint(snap)
	p.cur = idx
	setSessionInterpFPS(p.playbackSession(), p.fps)
	resetSessionInterpolation(p.playbackSession())
	p.resetOnNextDraw = idx == 0
	// Avoid interpolation artifacts on the first frame after a seek.
	suppressInterpOnce = true
	p.updateUI()
	movieMusicPaused.Store(!wasPlaying)
	// Preparing a restored SoundFont stream can take noticeable time. Keep the
	// movie clock stopped until its audio is ready so a seek cannot leave the
	// song a second or two behind the recorded reactions.
	<-p.restoreIndexedMusic(idx, true)
	p.musicNeedsRestore = false
	p.playing = wasPlaying
	p.updateUI()
}

// maybeDecodeMessage applies a simple heuristic to determine whether a frame
// could contain a textual message. Frames shorter than the 16-byte prefix or
// tagged as draw-state (tag 2) are skipped to avoid needless decoding.
// This heuristic may be refined as additional frame types are understood.
func maybeDecodeMessage(m []byte) {
	maybeDecodeSessionMessage(primarySession, m)
}

func maybeDecodeSessionMessage(session *Session, m []byte) {
	if session == nil {
		return
	}
	if len(m) <= 16 {
		return
	}
	if len(m) >= 2 && binary.BigEndian.Uint16(m[:2]) == 2 {
		return
	}
	// decodeMessage mutates the message body; use a copy to keep the stored
	// frame unchanged.
	if txt := decodeSessionMessage(session, append([]byte(nil), m...)); txt != "" {
		_ = txt
	}
}

func resetInterpolation() {
	resetSessionInterpolation(primarySession)
}

func resetSessionInterpolation(session *Session) {
	if session == nil {
		return
	}
	session.draw.mu.Lock()
	session.draw.current.prevMobiles = make(map[uint8]frameMobile)
	session.draw.current.prevDescs = make(map[uint8]frameDescriptor)
	session.draw.current.prevPictures = nil
	session.draw.current.picShiftX = 0
	session.draw.current.picShiftY = 0
	for i := range session.draw.current.pictures {
		session.draw.current.pictures[i].PrevH = session.draw.current.pictures[i].H
		session.draw.current.pictures[i].PrevV = session.draw.current.pictures[i].V
		session.draw.current.pictures[i].Moving = false
	}
	session.draw.current.prevTime = session.draw.current.curTime
	session.draw.current.prevHP = session.draw.current.hp
	session.draw.current.prevHPMax = session.draw.current.hpMax
	session.draw.current.prevSP = session.draw.current.sp
	session.draw.current.prevSPMax = session.draw.current.spMax
	session.draw.current.prevBalance = session.draw.current.balance
	session.draw.current.prevBalanceMax = session.draw.current.balanceMax
	prepareSessionRenderCacheLocked(session)
	session.draw.mu.Unlock()
}

func setInterpFPS(fps int) {
	setSessionInterpFPS(primarySession, fps)
}

func setSessionInterpFPS(session *Session, fps int) {
	if session == nil {
		return
	}
	if fps < 1 {
		fps = 1
	}
	d := time.Second / time.Duration(fps)
	session.draw.mu.Lock()
	if session.draw.current.prevTime.IsZero() {
		session.draw.current.prevTime = time.Now()
	}
	session.draw.current.curTime = session.draw.current.prevTime.Add(d)
	session.draw.mu.Unlock()
}
