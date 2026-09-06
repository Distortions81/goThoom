package main

import (
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
	shortUnits, _ = durafmt.DefaultUnitsCoder.Decode("y:yrs,wk:wks,d:d,h:h,m:m,s:s,ms:ms,us:us")
	playingMovie  bool
	movieMode     bool
	movieWin      *eui.WindowData
	movieDropped  int
)

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
}

func captureMovieNightState() movieNightState {
	gNight.mu.Lock()
	defer gNight.mu.Unlock()
	return movieNightState{
		baseLevel:       gNight.BaseLevel,
		azimuth:         gNight.Azimuth,
		cloudy:          gNight.Cloudy,
		flags:           gNight.Flags,
		level:           gNight.Level,
		shadows:         gNight.Shadows,
		oldAzimuth:      gNight.oldAzimuth,
		redshift:        gNight.redshift,
		startOfTwilight: gNight.startOfTwilight,
	}
}

func restoreMovieNightState(n movieNightState) {
	gNight.mu.Lock()
	gNight.BaseLevel = n.baseLevel
	gNight.Azimuth = n.azimuth
	gNight.Cloudy = n.cloudy
	gNight.Flags = n.flags
	gNight.Level = n.level
	gNight.Shadows = n.shadows
	gNight.oldAzimuth = n.oldAzimuth
	gNight.redshift = n.redshift
	gNight.startOfTwilight = n.startOfTwilight
	gNight.mu.Unlock()
}

// checkpointInterval determines how often checkpoints are recorded during
// playback. Larger intervals reduce memory usage at the cost of slower seek
// times.
const checkpointInterval = 300
const movieSeekFullRenderInterval = 500 * time.Millisecond
const movieControlButtonHeight = 38

// moviePlayer manages clMov playback with basic controls.
type moviePlayer struct {
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

	// Slider motion can generate many values while an expensive seek is still
	// rebuilding state. Keep only the most recent value and seek to it next.
	seekMu      sync.Mutex
	seekTarget  int
	seekPending bool
	seekEpoch   uint64
	seekStopped bool

	checkpoints []movieCheckpoint

	slider     *eui.ItemData
	curLabel   *eui.ItemData
	totalLabel *eui.ItemData
	fpsLabel   *eui.ItemData
	playButton *eui.ItemData
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

func newMoviePlayer(frames []movieFrame, fps int, cancel context.CancelFunc) *moviePlayer {
	setInterpFPS(fps)
	frameInterval = time.Second / time.Duration(fps)
	playingMovie = true
	movieMode = true
	// Do not interpolate the very first frame of playback.
	// Ensure prevTime == curTime and clear prior history so sprites
	// don't lerp from zeroed positions on start.
	resetInterpolation()
	suppressInterpOnce = true
	return &moviePlayer{
		frames:          frames,
		fps:             fps,
		baseFPS:         fps,
		playing:         true,
		resetOnNextDraw: true,
		ticker:          time.NewTicker(time.Second / time.Duration(fps)),
		cancel:          cancel,
		looped:          make(chan struct{}, 1),
		checkpoints:     []movieCheckpoint{{idx: 0, state: cloneDrawState(initialState), night: captureMovieNightState()}},
	}
}

var seekLock sync.Mutex
var seekingMov bool
var movieSeekRenderGeneration atomic.Uint64
var movieSeekRenderedGeneration atomic.Uint64
var movieSeekRenderAcknowledged = make(chan struct{}, 1)

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
	stopSeek.SetTooltip("Stop seeking")
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
	setMovieControlIcon(reset, "arrow_right", "RESET")
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
				"Stop playback and return to login?",
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

	// When the movie controls window is closed, stop playback and return to
	// the login window so a new movie can be selected.
	win.OnClose = func() {
		log.Printf("movie playback stopped: movie controls closed")
		// Pause and stop ticker
		p.pause()
		if p.ticker != nil {
			p.ticker.Stop()
		}
		// Stop any active sounds
		stopAllSounds()
		stopAllTTS()
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
		resetNightState()
		// Clear the selected movie path and reopen the login window.
		clmov = ""
		pcapPath = ""
		if loginWin != nil {
			loginWin.MarkOpen()
		}
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
			return
		}
	}
	m := p.frames[p.cur]
	movieDropped = updateFrameCounters(m.index)
	if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
		handleDrawState(m.data, true)
		if p.resetOnNextDraw {
			resetInterpolation()
			suppressInterpOnce = true
			p.resetOnNextDraw = false
		}
	} else {
		// Advance the logical frame counter even when this movie frame
		// does not contain a draw-state update so time-based effects
		// (e.g., bubble expiration) progress correctly during playback.
		frameCounter++
	}
	maybeDecodeMessage(m.data)
	p.cur++
	if p.cur%checkpointInterval == 0 {
		night := captureMovieNightState()
		stateMu.Lock()
		cp := movieCheckpoint{idx: p.cur, state: cloneDrawState(state), night: night}
		stateMu.Unlock()
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
	if p.seekPending {
		p.seekStopped = true
		p.seekEpoch++
	}
	p.seekMu.Unlock()
}

func (p *moviePlayer) setFPS(fps int) {
	if fps < 1 {
		fps = 1
	} else if fps > 9999 {
		fps = 9999
	}
	p.fps = fps
	p.ticker.Reset(time.Second / time.Duration(p.fps))
	frameInterval = time.Second / time.Duration(p.fps)
	setInterpFPS(p.fps)
	p.updateUI()
}

func (p *moviePlayer) play() { p.playing = true }

func (p *moviePlayer) pause() {
	p.playing = false
}

func (p *moviePlayer) skipBackMilli(milli int) {
	if seekingMov {
		return
	}
	seekLock.Lock()
	go func() {
		skip := int(float64(milli) * (float64(p.baseFPS) / 1000.0))
		p.seek(p.cur - skip)
		seekLock.Unlock()
	}()

}

func (p *moviePlayer) skipForwardMilli(milli int) {
	if seekingMov {
		return
	}
	seekLock.Lock()
	go func() {
		skip := int(float64(milli) * (float64(p.baseFPS) / 1000.0))
		p.seek(p.cur + skip)
		seekLock.Unlock()
	}()

}

func (p *moviePlayer) seek(idx int) {
	p.seekWithCancel(idx, nil)
}

// seekWithCancel rebuilds movie state up to idx. A drag update or Stop Seek
// can end the rebuild early, retaining the latest fully processed frame.
func (p *moviePlayer) seekWithCancel(idx int, cancelled func() bool) {
	seekingMov = true
	defer func() { seekingMov = false }()

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

	stateMu.Lock()
	state = cloneDrawState(cp.state)
	// Ensure render caches reflect the restored checkpoint state. The cache
	// will be rebuilt again if additional frames are parsed.
	prepareRenderCacheLocked()
	stateMu.Unlock()
	restoreMovieNightState(cp.night)
	firstRender := publishMovieSeekRender()
	waitForMovieSeekRender(firstRender)
	frameCounter = cp.idx
	lastFullRender := time.Now()

	for i := cp.idx; i < idx; i++ {
		if cancelled != nil && cancelled() {
			idx = i
			break
		}
		m := p.frames[i]
		movieDropped = updateFrameCounters(m.index)
		if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
			now := time.Now()
			buildFullRender := i == idx-1 || movieSeekFullRenderDue(lastFullRender, now)
			if handleDrawState(m.data, buildFullRender) && buildFullRender {
				lastFullRender = now
				generation := publishMovieSeekRender()
				waitForMovieSeekRender(generation)
			}
		} else {
			// Keep timeline consistent during scrubbing when frames
			// without draw-state are encountered.
			frameCounter++
		}
		maybeDecodeMessage(m.data)
		if frameCounter%checkpointInterval == 0 {
			night := captureMovieNightState()
			stateMu.Lock()
			snap := movieCheckpoint{idx: frameCounter, state: cloneDrawState(state), night: night}
			stateMu.Unlock()
			p.addCheckpoint(snap)
		}
	}
	night := captureMovieNightState()
	stateMu.Lock()
	// Cancellation or a run of non-draw frames may end between periodic
	// publishes. Always leave a complete final cache for the first post-seek
	// frame rather than exposing the partially rebuilt state.
	prepareRenderCacheLocked()
	snap := movieCheckpoint{idx: idx, state: cloneDrawState(state), night: night}
	stateMu.Unlock()
	publishMovieSeekRender()
	p.addCheckpoint(snap)
	p.cur = idx
	setInterpFPS(p.fps)
	resetInterpolation()
	p.resetOnNextDraw = idx == 0
	// Avoid interpolation artifacts on the first frame after a seek.
	suppressInterpOnce = true
	p.updateUI()
	p.playing = wasPlaying
}

// maybeDecodeMessage applies a simple heuristic to determine whether a frame
// could contain a textual message. Frames shorter than the 16-byte prefix or
// tagged as draw-state (tag 2) are skipped to avoid needless decoding.
// This heuristic may be refined as additional frame types are understood.
func maybeDecodeMessage(m []byte) {
	if len(m) <= 16 {
		return
	}
	if len(m) >= 2 && binary.BigEndian.Uint16(m[:2]) == 2 {
		return
	}
	// decodeMessage mutates the message body; use a copy to keep the stored
	// frame unchanged.
	if txt := decodeMessage(append([]byte(nil), m...)); txt != "" {
		_ = txt
	}
}

func resetInterpolation() {
	stateMu.Lock()
	state.prevMobiles = make(map[uint8]frameMobile)
	state.prevDescs = make(map[uint8]frameDescriptor)
	state.prevPictures = nil
	state.picShiftX = 0
	state.picShiftY = 0
	for i := range state.pictures {
		state.pictures[i].PrevH = state.pictures[i].H
		state.pictures[i].PrevV = state.pictures[i].V
		state.pictures[i].Moving = false
	}
	state.prevTime = state.curTime
	state.prevHP = state.hp
	state.prevHPMax = state.hpMax
	state.prevSP = state.sp
	state.prevSPMax = state.spMax
	state.prevBalance = state.balance
	state.prevBalanceMax = state.balanceMax
	prepareRenderCacheLocked()
	stateMu.Unlock()
}

func setInterpFPS(fps int) {
	if fps < 1 {
		fps = 1
	}
	d := time.Second / time.Duration(fps)
	stateMu.Lock()
	if state.prevTime.IsZero() {
		state.prevTime = time.Now()
	}
	state.curTime = state.prevTime.Add(d)
	stateMu.Unlock()
}
