package main

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/Distortions81/EUI/eui"
	"github.com/hako/durafmt"
)

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
		Movable:   true,
		PinTo:     eui.PIN_BOTTOM_CENTER,
	})

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	// Time slider flow
	tFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	p.curLabel, _ = eui.NewText(&eui.ItemData{Text: "0s", Size: eui.Point{X: 60, Y: 24}})
	tFlow.AddItem(p.curLabel)

	max := float32(len(p.frames))
	var events *eui.EventHandler
	p.slider, events = eui.NewSlider(&eui.ItemData{MinValue: 0, MaxValue: max, Size: eui.Point{X: 200, Y: 24}})
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			p.seek(int(ev.Value))
		}
	}
	tFlow.AddItem(p.slider)

	totalDur := time.Duration(len(p.frames)) * time.Second / time.Duration(p.fps)
	p.totalLabel, _ = eui.NewText(&eui.ItemData{Text: durafmt.ParseShort(totalDur).String(), Size: eui.Point{X: 60, Y: 24}})
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

	stop, stopEv := eui.NewButton(&eui.ItemData{Text: "Stop", Size: eui.Point{X: 50, Y: 24}})
	stopEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.stop()
		}
	}
	bFlow.AddItem(stop)

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
		p.curLabel.Text = durafmt.ParseShort(d).String()
		p.curLabel.Dirty = true
	}
}

func (p *moviePlayer) play() { p.playing = true }

func (p *moviePlayer) stop() {
	p.playing = false
	p.seek(0)
}

func (p *moviePlayer) skipBack() { p.seek(p.cur - 5*p.fps) }

func (p *moviePlayer) skipForward() { p.seek(p.cur + 5*p.fps) }

func (p *moviePlayer) seek(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx > len(p.frames) {
		idx = len(p.frames)
	}
	wasPlaying := p.playing
	p.playing = false

	resetDrawState()
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
