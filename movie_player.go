package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"go_client/eui"

	"github.com/hako/durafmt"
)

var shortUnits, _ = durafmt.DefaultUnitsCoder.Decode("y:yrs,wk:wks,d:d,h:h,m:m,s:s,ms:ms,us:us")

// moviePlayer manages clMov playback with basic controls.
type moviePlayer struct {
	frames  [][]byte
	fps     int
	cur     int // number of frames processed
	playing bool
	ticker  *time.Ticker
	cancel  context.CancelFunc

	slider     *eui.ItemData
	curLabel   *eui.ItemData
	totalLabel *eui.ItemData
	fpsLabel   *eui.ItemData
}

func newMoviePlayer(frames [][]byte, fps int, cancel context.CancelFunc) *moviePlayer {
	setInterpFPS(fps)
	serverFPS = float64(fps)
	return &moviePlayer{
		frames:  frames,
		fps:     fps,
		playing: true,
		ticker:  time.NewTicker(time.Second / time.Duration(fps)),
		cancel:  cancel,
	}
}

// makePlaybackWindow creates the playback control window.
func (p *moviePlayer) makePlaybackWindow() {
	win := eui.NewWindow()
	win.Title = "Movie Controls"
	win.Closable = false
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
	p.slider.Label = "Position"
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			p.seek(int(ev.Value))
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
	backb.Tooltip = "Skip back 30s"
	backbEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBackMilli(30 * 1000)
		}
	}
	bFlow.AddItem(backb)

	back, backEv := eui.NewButton()
	back.Text = "<<"
	back.Size = eui.Point{X: 40, Y: 24}
	back.Tooltip = "Skip back 5s"
	backEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBackMilli(5 * 1000)
		}
	}
	bFlow.AddItem(back)

	play, playEv := eui.NewButton()
	play.Text = "Play/Pause"
	play.Size = eui.Point{X: 140, Y: 24}
	changePlayButton(p, play)
	playEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if p.playing {
				p.pause()
				changePlayButton(p, play)
			} else {
				p.play()
				changePlayButton(p, play)
			}
		}
	}
	bFlow.AddItem(play)

	forwardb, fwdbEv := eui.NewButton()
	forwardb.Text = ">>"
	forwardb.Size = eui.Point{X: 40, Y: 24}
	forwardb.Tooltip = "Skip forward 5s"
	fwdbEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipForwardMilli(5 * 1000)
		}
	}
	bFlow.AddItem(forwardb)

	forward, fwdEv := eui.NewButton()
	forward.Text = ">>>"
	forward.Size = eui.Point{X: 40, Y: 24}
	forward.Tooltip = "Skip forward 30s"
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
	half.Tooltip = "Half speed"
	halfEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps / 2)
		}
	}
	bFlow.AddItem(half)

	dec, decEv := eui.NewButton()
	dec.Text = "-"
	dec.Size = eui.Point{X: 40, Y: 24}
	dec.Tooltip = "Slow down"
	decEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps - 1)
		}
	}
	bFlow.AddItem(dec)

	reset, resetEv := eui.NewButton()
	reset.Text = "RESET"
	reset.Size = eui.Point{X: 140, Y: 24}
	resetEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(clMovFPS)
		}
	}
	bFlow.AddItem(reset)

	inc, incEv := eui.NewButton()
	inc.Text = "+"
	inc.Size = eui.Point{X: 40, Y: 24}
	inc.Tooltip = "Speed up"
	incEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps + 1)
		}
	}
	bFlow.AddItem(inc)

	dbl, dblEv := eui.NewButton()
	dbl.Text = "++"
	dbl.Size = eui.Point{X: 40, Y: 24}
	dbl.Tooltip = "Double speed"
	dblEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps * 2)
		}
	}
	bFlow.AddItem(dbl)

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

	p.updateUI()
}

func changePlayButton(p *moviePlayer, play *eui.ItemData) {
	if p.playing {
		play.Text = "Pause"
	} else {
		play.Text = "Play"
	}
}

func (p *moviePlayer) run(ctx context.Context) {
	<-gameStarted
	for {
		select {
		case <-ctx.Done():
			p.ticker.Stop()
			return
		case <-p.ticker.C:
			if p.playing {
				p.step()
			}
		}
	}
}

func (p *moviePlayer) step() {
	if p.cur >= len(p.frames) {
		p.playing = false
		//p.cancel()
		return
	}
	m := p.frames[p.cur]
	if len(m) >= 2 && binary.BigEndian.Uint16(m[:2]) == 2 {
		handleDrawState(m)
	}
	if txt := decodeMessage(m); txt != "" {
		_ = txt
	}
	p.cur++
	p.updateUI()
	if p.cur >= len(p.frames) {
		p.playing = false
		//p.cancel()
	}
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
}

func (p *moviePlayer) setFPS(fps int) {
	if fps < 1 {
		fps = 1
	}
	p.fps = fps
	p.ticker.Reset(time.Second / time.Duration(p.fps))
	setInterpFPS(p.fps)
	serverFPS = float64(p.fps)
	p.updateUI()
}

func (p *moviePlayer) play() { p.playing = true }

func (p *moviePlayer) pause() {
	p.playing = false
}

func (p *moviePlayer) skipBackMilli(milli int) {
	p.seek(p.cur - int(float64(milli)*(float64(p.fps)/1000.0)))
}

func (p *moviePlayer) skipForwardMilli(milli int) {
	p.seek(p.cur + int(float64(milli)*(float64(p.fps)/1000.0)))
}

func (p *moviePlayer) seek(idx int) {
	blockSound = true
	blockBubbles = true
	defer func() {
		blockSound = false
		blockBubbles = false
	}()

	if idx < 0 {
		idx = 0
	}
	if idx > len(p.frames) {
		idx = len(p.frames)
	}
	wasPlaying := p.playing
	p.playing = false
	resetDrawState()
	frameCounter = 0

	for i := 0; i < idx; i++ {
		m := p.frames[i]
		if len(m) >= 2 && binary.BigEndian.Uint16(m[:2]) == 2 {
			handleDrawState(m)
		}
		if txt := decodeMessage(m); txt != "" {
			_ = txt
		}
	}
	p.cur = idx
	resetInterpolation()
	setInterpFPS(p.fps)
	p.updateUI()
	p.playing = wasPlaying
}

func resetDrawState() {
	stateMu.Lock()
	state = cloneDrawState(initialState)
	stateMu.Unlock()
}

func resetInterpolation() {
	stateMu.Lock()
	state.prevMobiles = make(map[uint8]frameMobile)
	state.prevDescs = make(map[uint8]frameDescriptor)
	state.prevTime = state.curTime
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
