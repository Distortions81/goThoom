package main

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/Distortions81/EUI/eui"
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
}

func newMoviePlayer(frames [][]byte, fps int, cancel context.CancelFunc) *moviePlayer {
	return &moviePlayer{
		frames:  frames,
		fps:     fps,
		playing: true,
		ticker:  time.NewTicker(time.Second / time.Duration(fps)),
		cancel:  cancel,
	}
}

// initUI creates the playback control window.
func (p *moviePlayer) initUI() {
	win := eui.NewWindow(&eui.WindowData{
		Title:     "Controls",
		Open:      true,
		Closable:  false,
		Resizable: false,
		AutoSize:  true,
		PinTo:     eui.PIN_TOP_CENTER,
	})
	win.Closable = false

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	// Time slider flow
	tFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	p.curLabel, _ = eui.NewText(&eui.ItemData{Text: "0s", Size: eui.Point{X: 60, Y: 24}, FontSize: 10})
	tFlow.AddItem(p.curLabel)

	max := float32(len(p.frames))
	var events *eui.EventHandler
	p.slider, events = eui.NewSlider(&eui.ItemData{MinValue: 0, MaxValue: max, Size: eui.Point{X: 450, Y: 24}, IntOnly: true})
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			p.seek(int(ev.Value))
		}
	}
	tFlow.AddItem(p.slider)

	totalDur := time.Duration(len(p.frames)) * time.Second / time.Duration(p.fps)
	totalDur = totalDur.Round(time.Second)
	p.totalLabel, _ = eui.NewText(&eui.ItemData{Text: durafmt.Parse(totalDur).LimitFirstN(2).Format(shortUnits), Size: eui.Point{X: 60, Y: 24}, FontSize: 10})
	tFlow.AddItem(p.totalLabel)

	flow.AddItem(tFlow)

	// Button flow
	bFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	back, backEv := eui.NewButton(&eui.ItemData{Text: "<<", Size: eui.Point{X: 40, Y: 24}})
	backEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBack()
		}
	}
	bFlow.AddItem(back)

	pause, pauseEv := eui.NewButton(&eui.ItemData{Text: "Pause", Size: eui.Point{X: 50, Y: 24}})
	pauseEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.pause()
		}
	}
	bFlow.AddItem(pause)

	play, playEv := eui.NewButton(&eui.ItemData{Text: "Play", Size: eui.Point{X: 50, Y: 24}})
	playEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.play()
		}
	}
	bFlow.AddItem(play)

	forward, fwdEv := eui.NewButton(&eui.ItemData{Text: ">>", Size: eui.Point{X: 40, Y: 24}})
	fwdEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipForward()
		}
	}
	bFlow.AddItem(forward)

	flow.AddItem(bFlow)

	// Speed control flow
	sFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	dbl, dblEv := eui.NewButton(&eui.ItemData{Text: "++", Size: eui.Point{X: 40, Y: 24}})
	dblEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps * 2)
		}
	}
	sFlow.AddItem(dbl)

	inc, incEv := eui.NewButton(&eui.ItemData{Text: "+", Size: eui.Point{X: 40, Y: 24}})
	incEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps + 1)
		}
	}
	sFlow.AddItem(inc)

	dec, decEv := eui.NewButton(&eui.ItemData{Text: "-", Size: eui.Point{X: 40, Y: 24}})
	decEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps - 1)
		}
	}
	sFlow.AddItem(dec)

	half, halfEv := eui.NewButton(&eui.ItemData{Text: "--", Size: eui.Point{X: 40, Y: 24}})
	halfEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps / 2)
		}
	}
	sFlow.AddItem(half)

	flow.AddItem(sFlow)

	win.AddItem(flow)
	win.AddWindow(false)
	p.updateUI()
}

func (p *moviePlayer) run(ctx context.Context) {
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
		p.cancel()
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
		p.cancel()
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
}

func (p *moviePlayer) setFPS(fps int) {
	if fps < 1 {
		fps = 1
	}
	p.fps = fps
	p.ticker.Reset(time.Second / time.Duration(p.fps))
	p.updateUI()
}

func (p *moviePlayer) play() { p.playing = true }

func (p *moviePlayer) pause() {
	p.playing = false
}

func (p *moviePlayer) skipBack() { p.seek(p.cur - 5*p.fps) }

func (p *moviePlayer) skipForward() { p.seek(p.cur + 5*p.fps) }

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

	//resetDrawState()
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
	p.updateUI()
	p.playing = wasPlaying
}

func resetDrawState() {
	stateMu.Lock()
	state = drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
	stateMu.Unlock()
}

func resetInterpolation() {
	stateMu.Lock()
	state.prevMobiles = make(map[uint8]frameMobile)
	state.prevDescs = make(map[uint8]frameDescriptor)
	state.prevTime = state.curTime
	stateMu.Unlock()
}
