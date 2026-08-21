package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
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

// movieCheckpoint captures the draw state after processing a frame. idx
// matches the number of processed frames (the next frame to play).
type movieCheckpoint struct {
	idx   int
	state drawState
}

// checkpointInterval determines how often checkpoints are recorded during
// playback. Larger intervals reduce memory usage at the cost of slower seek
// times.
const checkpointInterval = 300

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
	// resetOnNextDraw removes interpolation history after the first complete
	// draw at movie start or after looping back to frame zero.
	resetOnNextDraw bool

	checkpoints []movieCheckpoint

	slider       *eui.ItemData
	curLabel     *eui.ItemData
	totalLabel   *eui.ItemData
	fpsLabel     *eui.ItemData
	playButton   *eui.ItemData
	repeatButton *eui.ItemData
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
		checkpoints:     []movieCheckpoint{{idx: 0, state: cloneDrawState(initialState)}},
	}
}

var seekLock sync.Mutex
var seekingMov bool

// makePlaybackWindow creates the playback control window.
func (p *moviePlayer) makePlaybackWindow() {
	win := eui.NewWindow()
	movieWin = win
	win.Title = "Movie Controls"
	win.ShowDragbar = true
	win.Theme.Window.DragbarColor = eui.Color{R: 96, G: 96, B: 96}
	win.DragbarSpacing = 5
	win.Closable = true
	win.Resizable = false
	win.AutoSize = true
	win.SetZone(eui.HZoneCenter, eui.VZoneBottomMiddle)

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	// Time slider flow
	tFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	p.curLabel, _ = eui.NewText()
	p.curLabel.Text = "0s"
	p.curLabel.Size = eui.Point{X: 60, Y: 24}
	p.curLabel.FontSize = 10
	tFlow.AddItem(p.curLabel)

	max := float32(len(p.frames))
	var events *eui.EventHandler
	p.slider, events = eui.NewSlider()
	p.slider.MinValue = 0
	p.slider.MaxValue = max
	p.slider.Size = eui.Point{X: 650, Y: 24}
	p.slider.IntOnly = true
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged && !seekingMov {
			seekLock.Lock()
			go func() {
				p.seek(int(ev.Value))
				seekLock.Unlock()
			}()
		}
	}
	tFlow.AddItem(p.slider)

	totalDur := time.Duration(len(p.frames)) * time.Second / time.Duration(p.fps)
	totalDur = totalDur.Round(time.Second)
	p.totalLabel, _ = eui.NewText()
	p.totalLabel.Text = durafmt.Parse(totalDur).LimitFirstN(2).Format(shortUnits)
	p.totalLabel.Size = eui.Point{X: 60, Y: 24}
	p.totalLabel.FontSize = 10
	tFlow.AddItem(p.totalLabel)

	flow.AddItem(tFlow)

	// Button flow
	bFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	backb, backbEv := eui.NewButton()
	backb.Text = "<<<"
	backb.Size = eui.Point{X: 40, Y: 24}
	backb.SetTooltip("Skip back 30s")
	backbEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBackMilli(30 * 1000)
		}
	}
	bFlow.AddItem(backb)

	back, backEv := eui.NewButton()
	back.Text = "<<"
	back.Size = eui.Point{X: 40, Y: 24}
	back.SetTooltip("Skip back 5s")
	backEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBackMilli(5 * 1000)
		}
	}
	bFlow.AddItem(back)

	play, playEv := eui.NewButton()
	play.Text = "Play/Pause"
	play.SetTooltip("Toggle playback")
	play.Size = eui.Point{X: 140, Y: 24}
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

	repeatBtn, repeatEv := eui.NewButton()
	repeatBtn.Text = "repeat"
	repeatBtn.SetTooltip("Loop playback when the movie ends")
	repeatBtn.Size = eui.Point{X: 80, Y: 24}
	p.repeatButton = repeatBtn
	changeRepeatButton(p, p.repeatButton)
	repeatEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.repeat = !p.repeat
			changeRepeatButton(p, p.repeatButton)
		}
	}
	bFlow.AddItem(repeatBtn)

	forwardb, fwdbEv := eui.NewButton()
	forwardb.Text = ">>"
	forwardb.Size = eui.Point{X: 40, Y: 24}
	forwardb.SetTooltip("Skip forward 5s")
	fwdbEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipForwardMilli(5 * 1000)
		}
	}
	bFlow.AddItem(forwardb)

	forward, fwdEv := eui.NewButton()
	forward.Text = ">>>"
	forward.Size = eui.Point{X: 40, Y: 24}
	forward.SetTooltip("Skip forward 30s")
	fwdEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipForwardMilli(30 * 1000)
		}
	}
	bFlow.AddItem(forward)

	spacer, _ := eui.NewText()
	spacer.Text = ""
	spacer.Size = eui.Point{X: 40, Y: 24}
	bFlow.AddItem(spacer)

	half, halfEv := eui.NewButton()
	half.Text = "--"
	half.Size = eui.Point{X: 40, Y: 24}
	half.SetTooltip("Half speed")
	halfEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps / 2)
		}
	}
	bFlow.AddItem(half)

	dec, decEv := eui.NewButton()
	dec.Text = "-"
	dec.Size = eui.Point{X: 40, Y: 24}
	dec.SetTooltip("Slow down")
	decEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps - 1)
		}
	}
	bFlow.AddItem(dec)

	reset, resetEv := eui.NewButton()
	reset.Text = "RESET"
	reset.SetTooltip("Reset playback speed")
	reset.Size = eui.Point{X: 140, Y: 24}
	resetEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.baseFPS)
		}
	}
	bFlow.AddItem(reset)

	inc, incEv := eui.NewButton()
	inc.Text = "+"
	inc.Size = eui.Point{X: 40, Y: 24}
	inc.SetTooltip("Speed up")
	incEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps + 1)
		}
	}
	bFlow.AddItem(inc)

	dbl, dblEv := eui.NewButton()
	dbl.Text = "++"
	dbl.Size = eui.Point{X: 40, Y: 24}
	dbl.SetTooltip("Double speed")
	dblEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps * 2)
		}
	}
	bFlow.AddItem(dbl)

	exitBtn, exitEv := eui.NewButton()
	exitBtn.Text = "Exit"
	exitBtn.Size = eui.Point{X: 80, Y: 24}
	exitEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			showPopup(
				"Exit Movie",
				"Stop playback and return to login?",
				[]popupButton{{Text: "Cancel"}, {Text: "Exit", Color: &eui.ColorDarkRed, HoverColor: &eui.ColorRed, Action: func() {
					if movieWin != nil {
						movieWin.Close()
					}
				}}},
			)
		}
	}
	bFlow.AddItem(exitBtn)

	buf := fmt.Sprintf("%v fps", p.fps)
	fpsInfo, _ := eui.NewText()
	fpsInfo.Text = buf
	fpsInfo.Size = eui.Point{X: 100, Y: 24}
	fpsInfo.FontSize = 15
	fpsInfo.Alignment = eui.ALIGN_CENTER
	p.fpsLabel = fpsInfo
	bFlow.AddItem(fpsInfo)

	flow.AddItem(bFlow)
	win.AddItem(flow)

	// Recompute window dimensions now that all controls are present
	win.Refresh()

	// Add and open the fully populated window
	win.AddWindow(false)
	win.MarkOpen()

	// When the movie controls window is closed, stop playback and return to
	// the login window so a new movie can be selected.
	win.OnClose = func() {
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
		play.Text = "Pause"
	} else {
		play.Text = "Play"
	}
}

func changeRepeatButton(p *moviePlayer, repeat *eui.ItemData) {
	if repeat == nil {
		return
	}
	if p.repeat {
		repeat.Text = "no repeat"
	} else {
		repeat.Text = "repeat"
	}
	repeat.Dirty = true
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
		stateMu.Lock()
		cp := movieCheckpoint{idx: p.cur, state: cloneDrawState(state)}
		stateMu.Unlock()
		p.checkpoints = append(p.checkpoints, cp)
	}
	if p.cur >= len(p.frames) {
		if p.repeat {
			p.seek(0)
		} else {
			p.playing = false
			playingMovie = false
			updateRecordButton()
		}
	}
	p.updateUI()
}

func (p *moviePlayer) updateUI() {
	if p.slider != nil {
		p.slider.Value = float32(p.cur)
		p.slider.Dirty = true
	}
	if p.curLabel != nil {
		d := time.Duration(p.cur) * time.Second / time.Duration(p.fps)
		d = d.Round(time.Second)
		p.curLabel.Text = durafmt.Parse(d).LimitFirstN(2).Format(shortUnits)
		p.curLabel.Dirty = true
	}
	if p.totalLabel != nil {
		totalDur := time.Duration(len(p.frames)) * time.Second / time.Duration(p.fps)
		totalDur = totalDur.Round(time.Second)
		p.totalLabel.Text = durafmt.Parse(totalDur).LimitFirstN(2).Format(shortUnits)
		p.totalLabel.Dirty = true
	}

	if p.fpsLabel != nil {
		p.fpsLabel.Text = fmt.Sprintf("UPS: %v", p.fps)
		p.fpsLabel.Dirty = true
	}

	if p.playButton != nil {
		changePlayButton(p, p.playButton)
	}

	if p.repeatButton != nil {
		changeRepeatButton(p, p.repeatButton)
	}
}

func (p *moviePlayer) setFPS(fps int) {
	if fps < 1 {
		fps = 1
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

	cp := p.checkpoints[0]
	for i := len(p.checkpoints) - 1; i >= 0; i-- {
		if p.checkpoints[i].idx <= idx {
			cp = p.checkpoints[i]
			break
		}
	}

	stateMu.Lock()
	state = cloneDrawState(cp.state)
	// Ensure render caches reflect the restored checkpoint state. The cache
	// will be rebuilt again if additional frames are parsed.
	prepareRenderCacheLocked()
	stateMu.Unlock()
	frameCounter = cp.idx

	for i := cp.idx; i < idx; i++ {
		m := p.frames[i]
		movieDropped = updateFrameCounters(m.index)
		if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
			// Skip render cache preparation for intermediate frames.
			handleDrawState(m.data, i == idx-1)
		} else {
			// Keep timeline consistent during scrubbing when frames
			// without draw-state are encountered.
			frameCounter++
		}
		maybeDecodeMessage(m.data)
		if frameCounter%checkpointInterval == 0 {
			last := p.checkpoints[len(p.checkpoints)-1]
			if last.idx != frameCounter {
				stateMu.Lock()
				snap := movieCheckpoint{idx: frameCounter, state: cloneDrawState(state)}
				stateMu.Unlock()
				p.checkpoints = append(p.checkpoints, snap)
			}
		}
	}
	last := p.checkpoints[len(p.checkpoints)-1]
	if last.idx != idx {
		stateMu.Lock()
		snap := movieCheckpoint{idx: idx, state: cloneDrawState(state)}
		stateMu.Unlock()
		p.checkpoints = append(p.checkpoints, snap)
	}
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
