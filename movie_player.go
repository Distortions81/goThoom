package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
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
	states  []drawSnapshot

	slider     *eui.ItemData
	curLabel   *eui.ItemData
	totalLabel *eui.ItemData
	fpsLabel   *eui.ItemData

	mu sync.Mutex
}

func newMoviePlayer(frames [][]byte, fps int, cancel context.CancelFunc) *moviePlayer {
	return &moviePlayer{
		frames:  frames,
		fps:     fps,
		playing: false,
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

	var events *eui.EventHandler
	p.slider, events = eui.NewSlider(&eui.ItemData{MinValue: 0, MaxValue: 0, Size: eui.Point{X: 450, Y: 24}, IntOnly: true})
	events.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			p.seek(int(ev.Value))
		}
	}
	tFlow.AddItem(p.slider)

	p.totalLabel, _ = eui.NewText(&eui.ItemData{Text: "0s", Size: eui.Point{X: 60, Y: 24}, FontSize: 10})
	tFlow.AddItem(p.totalLabel)

	flow.AddItem(tFlow)

	// Button flow
	bFlow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL}

	backb, backbEv := eui.NewButton(&eui.ItemData{Text: "<<<", Size: eui.Point{X: 40, Y: 24}})
	backbEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBackMilli(30 * 1000)
		}
	}
	bFlow.AddItem(backb)

	back, backEv := eui.NewButton(&eui.ItemData{Text: "<<", Size: eui.Point{X: 40, Y: 24}})
	backEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipBackMilli(5 * 1000)
		}
	}
	bFlow.AddItem(back)

	play, playEv := eui.NewButton(&eui.ItemData{Text: ">  ||", Size: eui.Point{X: 50, Y: 24}})
	playEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			if p.playing {
				p.pause()
			} else {
				p.play()
			}
		}
	}
	bFlow.AddItem(play)

	forwardb, fwdbEv := eui.NewButton(&eui.ItemData{Text: ">>", Size: eui.Point{X: 40, Y: 24}})
	fwdbEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipForwardMilli(5 * 1000)
		}
	}
	bFlow.AddItem(forwardb)

	forward, fwdEv := eui.NewButton(&eui.ItemData{Text: ">>>", Size: eui.Point{X: 40, Y: 24}})
	fwdEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.skipForwardMilli(30 * 1000)
		}
	}
	bFlow.AddItem(forward)

	spacer, _ := eui.NewText(&eui.ItemData{Text: "", Size: eui.Point{X: 40, Y: 24}})
	bFlow.AddItem(spacer)

	half, halfEv := eui.NewButton(&eui.ItemData{Text: "--", Size: eui.Point{X: 40, Y: 24}})
	halfEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps / 2)
		}
	}
	bFlow.AddItem(half)

	dec, decEv := eui.NewButton(&eui.ItemData{Text: "-", Size: eui.Point{X: 40, Y: 24}})
	decEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps - 1)
		}
	}
	bFlow.AddItem(dec)

	buf := fmt.Sprintf("%v fps", p.fps)
	fpsInfo, _ := eui.NewText(&eui.ItemData{Text: buf, Size: eui.Point{X: 100, Y: 24}, FontSize: 15, Alignment: eui.ALIGN_CENTER})
	p.fpsLabel = fpsInfo
	bFlow.AddItem(fpsInfo)

	inc, incEv := eui.NewButton(&eui.ItemData{Text: "+", Size: eui.Point{X: 40, Y: 24}})
	incEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps + 1)
		}
	}
	bFlow.AddItem(inc)

	dbl, dblEv := eui.NewButton(&eui.ItemData{Text: "++", Size: eui.Point{X: 40, Y: 24}})
	dblEv.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			p.setFPS(p.fps * 2)
		}
	}
	bFlow.AddItem(dbl)

	flow.AddItem(bFlow)
	win.AddItem(flow)
	win.AddWindow(false)
	p.updateUI()
}

// cacheFrames simulates all frames and stores draw state snapshots.
func (p *moviePlayer) cacheFrames() {
	addMessage("Processing clMov frames...")

	prevSound := blockSound
	blockSound = true
	defer func() { blockSound = prevSound }()

	p.mu.Lock()
	if len(p.states) == 0 {
		p.states = make([]drawSnapshot, 0, len(p.frames)+1)
		p.states = append(p.states, captureDrawSnapshot())
	}
	p.mu.Unlock()
	p.updateUI()

	prevRender := blockRender
	for i := len(p.states) - 1; i < len(p.frames); i++ {
		p.mu.Lock()
		curSnap := p.states[p.cur]
		lastSnap := p.states[len(p.states)-1]
		blockRender = true
		applyDrawSnapshot(lastSnap, p.fps)
		m := p.frames[i]
		if len(m) >= 2 && binary.BigEndian.Uint16(m[:2]) == 2 {
			handleDrawState(m)
		}
		if txt := decodeMessage(m); txt != "" {
			_ = txt
		}
		snap := captureDrawSnapshot()
		p.states = append(p.states, snap)
		applyDrawSnapshot(curSnap, p.fps)
		blockRender = prevRender
		p.mu.Unlock()

		p.updateUI()

		if i == 0 {
			p.play()
		}
	}

	addMessage("Complete, starting playback!")
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
	p.mu.Lock()
	if p.cur >= len(p.states)-1 {
		p.playing = false
		p.mu.Unlock()
		return
	}
	p.cur++
	applyDrawSnapshot(p.states[p.cur], p.fps)
	if p.cur >= len(p.states)-1 {
		p.playing = false
	}
	p.mu.Unlock()
	p.updateUI()
}

func (p *moviePlayer) updateUI() {
	p.mu.Lock()
	cur := p.cur
	total := len(p.states) - 1
	fps := p.fps
	p.mu.Unlock()

	if total < 0 {
		total = 0
	}
	if cur > total {
		cur = total
	}

	if p.slider != nil {
		p.slider.MaxValue = float32(total)
		p.slider.Value = float32(cur)
		p.slider.Dirty = true
	}
	if p.curLabel != nil {
		d := time.Duration(cur) * time.Second / time.Duration(fps)
		d = d.Round(time.Second)
		p.curLabel.Text = durafmt.Parse(d).LimitFirstN(2).Format(shortUnits)
		p.curLabel.Dirty = true
	}
	if p.totalLabel != nil {
		totalDur := time.Duration(total) * time.Second / time.Duration(fps)
		totalDur = totalDur.Round(time.Second)
		p.totalLabel.Text = durafmt.Parse(totalDur).LimitFirstN(2).Format(shortUnits)
		p.totalLabel.Dirty = true
	}

	if p.fpsLabel != nil {
		p.fpsLabel.Text = fmt.Sprintf("UPS: %v", fps)
		p.fpsLabel.Dirty = true
	}
}

func (p *moviePlayer) setFPS(fps int) {
	if fps < 1 {
		fps = 1
	}
	p.mu.Lock()
	p.fps = fps
	p.ticker.Reset(time.Second / time.Duration(p.fps))
	p.mu.Unlock()
	clMovFPS = fps
	resetInterpolation()
	p.updateUI()
}

func (p *moviePlayer) play() {
	p.mu.Lock()
	p.playing = true
	p.mu.Unlock()
}

func (p *moviePlayer) pause() {
	p.mu.Lock()
	p.playing = false
	p.mu.Unlock()
}

func (p *moviePlayer) skipBackMilli(milli int) {
	p.mu.Lock()
	cur := p.cur
	fps := p.fps
	p.mu.Unlock()
	p.seek(cur - int(float64(milli)*(float64(fps)/1000.0)))
}

func (p *moviePlayer) skipForwardMilli(milli int) {
	p.mu.Lock()
	cur := p.cur
	fps := p.fps
	p.mu.Unlock()
	p.seek(cur + int(float64(milli)*(float64(fps)/1000.0)))
}

func (p *moviePlayer) seek(idx int) {
	p.mu.Lock()
	if idx < 0 {
		idx = 0
	}
	if idx > len(p.states)-1 {
		idx = len(p.states) - 1
	}
	wasPlaying := p.playing
	p.playing = false
	applyDrawSnapshot(p.states[idx], p.fps)
	p.cur = idx
	p.mu.Unlock()
	p.updateUI()
	if wasPlaying {
		p.play()
	}
}

func resetInterpolation() {
	stateMu.Lock()
	state.prevMobiles = make(map[uint8]frameMobile)
	state.prevDescs = make(map[uint8]frameDescriptor)
	state.prevTime = state.curTime
	stateMu.Unlock()
}
