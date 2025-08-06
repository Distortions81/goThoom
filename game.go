package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"image/color"
	"log"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Distortions81/EUI/eui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const gameAreaSizeX, gameAreaSizeY = 500, 500
const fieldCenterX, fieldCenterY = gameAreaSizeX / 2, gameAreaSizeY / 2
const epsilon = 0.005

var mouseX, mouseY int16
var mouseDown bool

var keyWalk bool
var keyX, keyY int16
var clickToToggle bool
var walkToggled bool
var walkTargetX, walkTargetY int16

var inputActive bool
var inputText []rune
var inputBg *ebiten.Image
var hudPixel *ebiten.Image

var settingsWin *eui.WindowData
var gameCtx context.Context
var scale int = 3
var interp bool
var smoothMoving bool
var onion bool
var fastAnimation = true
var blendPicts bool
var linear bool
var smoothDebug bool
var hideMoving bool
var drawFilter = ebiten.FilterNearest
var frameCounter int
var showPlanes bool
var showBubbles bool
var nightMode bool
var vsync = true
var hideMobiles bool

var (
	frameCh       = make(chan struct{}, 1)
	lastFrameTime time.Time
	frameInterval = 200 * time.Millisecond
	intervalHist  = map[int]int{}
	frameMu       sync.Mutex
	serverFPS     int
)

// drawState tracks information needed by the Ebiten renderer.
type drawState struct {
	descriptors map[uint8]frameDescriptor
	pictures    []framePicture
	picShiftX   int
	picShiftY   int
	mobiles     map[uint8]frameMobile
	prevMobiles map[uint8]frameMobile
	prevDescs   map[uint8]frameDescriptor
	prevTime    time.Time
	curTime     time.Time

	bubbles []bubble

	hp, hpMax                   int
	sp, spMax                   int
	balance, balanceMax         int
	prevHP, prevHPMax           int
	prevSP, prevSPMax           int
	prevBalance, prevBalanceMax int
}

var (
	state = drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
	stateMu sync.Mutex
)

// bubble stores temporary bubble debug information.
type bubble struct {
	Index   uint8
	H, V    int16
	Far     bool
	NoArrow bool
	Text    string
	Type    int
	Expire  time.Time
}

// drawSnapshot is a read-only copy of the current draw state.
type drawSnapshot struct {
	descriptors                 map[uint8]frameDescriptor
	pictures                    []framePicture
	picShiftX                   int
	picShiftY                   int
	mobiles                     []frameMobile
	prevMobiles                 map[uint8]frameMobile
	prevDescs                   map[uint8]frameDescriptor
	prevTime                    time.Time
	curTime                     time.Time
	bubbles                     []bubble
	hp, hpMax                   int
	sp, spMax                   int
	balance, balanceMax         int
	prevHP, prevHPMax           int
	prevSP, prevSPMax           int
	prevBalance, prevBalanceMax int
}

// captureDrawSnapshot copies the shared draw state under a mutex.
func captureDrawSnapshot() drawSnapshot {
	stateMu.Lock()
	defer stateMu.Unlock()

	snap := drawSnapshot{
		descriptors:    make(map[uint8]frameDescriptor, len(state.descriptors)),
		pictures:       append([]framePicture(nil), state.pictures...),
		picShiftX:      state.picShiftX,
		picShiftY:      state.picShiftY,
		mobiles:        make([]frameMobile, 0, len(state.mobiles)),
		prevTime:       state.prevTime,
		curTime:        state.curTime,
		hp:             state.hp,
		hpMax:          state.hpMax,
		sp:             state.sp,
		spMax:          state.spMax,
		balance:        state.balance,
		balanceMax:     state.balanceMax,
		prevHP:         state.prevHP,
		prevHPMax:      state.prevHPMax,
		prevSP:         state.prevSP,
		prevSPMax:      state.prevSPMax,
		prevBalance:    state.prevBalance,
		prevBalanceMax: state.prevBalanceMax,
	}

	for idx, d := range state.descriptors {
		snap.descriptors[idx] = d
	}
	for _, m := range state.mobiles {
		snap.mobiles = append(snap.mobiles, m)
	}
	if len(state.bubbles) > 0 {
		now := time.Now()
		kept := state.bubbles[:0]
		for _, b := range state.bubbles {
			if b.Expire.After(now) {
				if !b.Far {
					if m, ok := state.mobiles[b.Index]; ok {
						b.H, b.V = m.H, m.V
					}
				}
				kept = append(kept, b)
			}
		}
		last := make(map[uint8]int)
		for i, b := range kept {
			last[b.Index] = i
		}
		dedup := kept[:0]
		for i, b := range kept {
			if last[b.Index] == i {
				dedup = append(dedup, b)
			}
		}
		state.bubbles = dedup
		snap.bubbles = append([]bubble(nil), state.bubbles...)
	}
	if interp || onion || !fastAnimation {
		snap.prevMobiles = make(map[uint8]frameMobile, len(state.prevMobiles))
		for idx, m := range state.prevMobiles {
			snap.prevMobiles[idx] = m
		}
	}
	if onion {
		snap.prevDescs = make(map[uint8]frameDescriptor, len(state.prevDescs))
		for idx, d := range state.prevDescs {
			snap.prevDescs[idx] = d
		}
	}
	return snap
}

// computeInterpolation returns the blend factors for frame interpolation and onion skinning.
func computeInterpolation(prevTime, curTime time.Time, rate float64) (alpha float64, fade float32) {
	alpha = 1.0
	fade = 1.0
	if (interp || onion || blendPicts) && !curTime.IsZero() && curTime.After(prevTime) {
		elapsed := time.Since(prevTime)
		interval := curTime.Sub(prevTime)
		if interp {
			alpha = float64(elapsed) / float64(interval)
			if alpha < 0 {
				alpha = 0
			}
			if alpha > 1 {
				alpha = 1
			}
		}
		if onion || blendPicts {
			half := float64(interval) * rate
			if half > 0 {
				fade = float32(float64(elapsed) / float64(half))
			}
			if fade < 0 {
				fade = 0
			}
			if fade > 1 {
				fade = 1
			}
		}
	}
	return alpha, fade
}

type Game struct{}

func (g *Game) Update() error {
	eui.Update()

	if settingsDirty {
		saveSettings()
		settingsDirty = false
	}

	if inputActive {
		inputText = append(inputText, ebiten.AppendInputChars(nil)...)
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			if len(inputText) > 0 {
				inputText = inputText[:len(inputText)-1]
			}
		} else if d := inpututil.KeyPressDuration(ebiten.KeyBackspace); d > 30 && d%3 == 0 {
			if len(inputText) > 0 {
				inputText = inputText[:len(inputText)-1]
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			txt := strings.TrimSpace(string(inputText))
			if txt != "" {
				pendingCommand = txt
				//addMessage("> " + txt)
			}
			inputActive = false
			inputText = inputText[:0]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			inputActive = false
			inputText = inputText[:0]
		}
	} else {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			inputActive = true
			inputText = inputText[:0]
		}
	}

	if !inputActive {
		dx, dy := 0, 0
		if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
			dx--
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
			dx++
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
			dy--
		}
		if ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
			dy++
		}
		if dx != 0 || dy != 0 {
			keyWalk = true
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				keyX = int16(dx * fieldCenterX)
				keyY = int16(dy * fieldCenterY)
			} else {
				keyX = int16(dx * (fieldCenterX / 2))
				keyY = int16(dy * (fieldCenterX / 2))
			}
		} else {
			keyWalk = false
		}

		mx, my := ebiten.CursorPosition()
		overUI := pointInUI(mx, my)

		if clickToToggle {
			if !overUI && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if walkToggled {
					walkToggled = false
				} else {
					walkTargetX = int16(mx/scale - fieldCenterX)
					walkTargetY = int16(my/scale - fieldCenterY)
					walkToggled = true
				}
			}
			if walkToggled {
				w, h := eui.ScreenSize()
				if overUI || mx < 0 || my < 0 || mx >= w || my >= h {
					walkToggled = false
				} else {
					walkTargetX = int16(mx/scale - fieldCenterX)
					walkTargetY = int16(my/scale - fieldCenterY)
				}
			}
		} else {
			walkToggled = false
		}
	} else {
		keyWalk = false
		if walkToggled {
			walkToggled = false
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if clmov == "" && tcpConn == nil && !noSplash {
		drawSplash(screen)
		return
	}
	snap := captureDrawSnapshot()
	alpha, fade := computeInterpolation(snap.prevTime, snap.curTime, blendRate)
	//logDebug("Draw alpha=%.2f shift=(%d,%d) pics=%d", alpha, snap.picShiftX, snap.picShiftY, len(snap.pictures))
	drawScene(screen, snap, alpha, fade)
	if nightMode {
		drawNightOverlay(screen)
	}
	drawMessages(screen, getMessages())

	eui.Draw(screen)
	if inputActive {
		drawInputOverlay(screen, string(inputText))
	}
	drawStatusBars(screen, snap, alpha)
	drawServerFPS(screen, serverFPS)
}

// drawScene renders all world objects for the current frame.
func drawScene(screen *ebiten.Image, snap drawSnapshot, alpha float64, fade float32) {
	descMap := make(map[uint8]frameDescriptor, len(snap.descriptors))
	for idx, d := range snap.descriptors {
		descMap[idx] = d
	}

	sort.Slice(snap.pictures, func(i, j int) bool {
		pi := 0
		pj := 0
		if clImages != nil {
			pi = clImages.Plane(uint32(snap.pictures[i].PictID))
			pj = clImages.Plane(uint32(snap.pictures[j].PictID))
		}
		if pi != pj {
			return pi < pj
		}
		if snap.pictures[i].V == snap.pictures[j].V {
			return snap.pictures[i].H < snap.pictures[j].H
		}
		return snap.pictures[i].V < snap.pictures[j].V
	})

	dead := make([]frameMobile, 0, len(snap.mobiles))
	live := make([]frameMobile, 0, len(snap.mobiles))
	for _, m := range snap.mobiles {
		if m.State == poseDead {
			dead = append(dead, m)
		}
		live = append(live, m)
	}

	negPics := make([]framePicture, 0)
	zeroPics := make([]framePicture, 0)
	posPics := make([]framePicture, 0)
	for _, p := range snap.pictures {
		plane := 0
		if clImages != nil {
			plane = clImages.Plane(uint32(p.PictID))
		}
		switch {
		case plane < 0:
			negPics = append(negPics, p)
		case plane == 0:
			zeroPics = append(zeroPics, p)
		default:
			posPics = append(posPics, p)
		}
	}

	for _, p := range negPics {
		drawPicture(screen, p, alpha, fade, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY)
	}

	if hideMobiles {
		for _, p := range zeroPics {
			drawPicture(screen, p, alpha, fade, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY)
		}
	} else {
		sort.Slice(dead, func(i, j int) bool { return dead[i].V < dead[j].V })
		for _, m := range dead {
			drawMobile(screen, m, descMap, snap.prevMobiles, snap.prevDescs, snap.picShiftX, snap.picShiftY, alpha, fade)
		}

		sort.Slice(live, func(i, j int) bool { return live[i].V < live[j].V })
		i, j := 0, 0
		for i < len(live) || j < len(zeroPics) {
			var mV, pV int
			if i < len(live) {
				mV = int(live[i].V)
			} else {
				mV = int(^uint(0) >> 1)
			}
			if j < len(zeroPics) {
				pV = int(zeroPics[j].V)
			} else {
				pV = int(^uint(0) >> 1)
			}
			if mV < pV {
				if live[i].State != poseDead {
					drawMobile(screen, live[i], descMap, snap.prevMobiles, snap.prevDescs, snap.picShiftX, snap.picShiftY, alpha, fade)
				}
				i++
			} else {
				drawPicture(screen, zeroPics[j], alpha, fade, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY)
				j++
			}
		}
	}

	for _, p := range posPics {
		drawPicture(screen, p, alpha, fade, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY)
	}

	if showBubbles {
		for _, b := range snap.bubbles {
			hpos := float64(b.H)
			vpos := float64(b.V)
			if !b.Far {
				var m *frameMobile
				for i := range snap.mobiles {
					if snap.mobiles[i].Index == b.Index {
						m = &snap.mobiles[i]
						break
					}
				}
				if m != nil {
					hpos = float64(m.H)
					vpos = float64(m.V)
					if interp {
						if pm, ok := snap.prevMobiles[b.Index]; ok {
							dh := int(m.H) - int(pm.H) - snap.picShiftX
							dv := int(m.V) - int(pm.V) - snap.picShiftY
							if dh*dh+dv*dv <= maxMobileInterpPixels*maxMobileInterpPixels {
								hpos = float64(pm.H)*(1-alpha) + float64(m.H)*alpha
								vpos = float64(pm.V)*(1-alpha) + float64(m.V)*alpha
							}
						}
					}
				}
			}
			x := (int(math.Round(hpos)) + fieldCenterX) * scale
			y := (int(math.Round(vpos)) + fieldCenterY) * scale
			borderCol, bgCol, textCol := bubbleColors(b.Type)
			drawBubble(screen, b.Text, x, y, b.Type, b.Far, b.NoArrow, borderCol, bgCol, textCol)
		}
	}
}

// drawMobile renders a single mobile object with optional interpolation and onion skinning.
func drawMobile(screen *ebiten.Image, m frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, prevDescs map[uint8]frameDescriptor, shiftX, shiftY int, alpha float64, fade float32) {
	h := float64(m.H)
	v := float64(m.V)
	if interp {
		if pm, ok := prevMobiles[m.Index]; ok {
			dh := int(m.H) - int(pm.H) - shiftX
			dv := int(m.V) - int(pm.V) - shiftY
			if dh*dh+dv*dv <= maxMobileInterpPixels*maxMobileInterpPixels {
				h = float64(pm.H)*(1-alpha) + float64(m.H)*alpha
				v = float64(pm.V)*(1-alpha) + float64(m.V)*alpha
			}
		}
	}
	x := (int(math.Round(h)) + fieldCenterX) * scale
	y := (int(math.Round(v)) + fieldCenterY) * scale
	var img *ebiten.Image
	plane := 0
	if d, ok := descMap[m.Index]; ok {
		colors := d.Colors
		playersMu.RLock()
		if p, ok := players[d.Name]; ok && len(p.Colors) > 0 {
			colors = append([]byte(nil), p.Colors...)
		}
		playersMu.RUnlock()
		state := m.State
		if !fastAnimation {
			if pm, ok := prevMobiles[m.Index]; ok {
				state = pm.State
			}
		}
		img = loadMobileFrame(d.PictID, state, colors)
		if clImages != nil {
			plane = clImages.Plane(uint32(d.PictID))
		}
	}
	var prevImg *ebiten.Image
	if onion {
		if pm, ok := prevMobiles[m.Index]; ok {
			pd := descMap[m.Index]
			if d, ok := prevDescs[m.Index]; ok {
				pd = d
			}
			prevColors := pd.Colors
			playersMu.RLock()
			if p, ok := players[pd.Name]; ok && len(p.Colors) > 0 {
				prevColors = append([]byte(nil), p.Colors...)
			}
			playersMu.RUnlock()
			prevImg = loadMobileFrame(pd.PictID, pm.State, prevColors)
		}
	}
	if img != nil {
		size := img.Bounds().Dx()
		if onion && prevImg != nil {
			tmp := getTempImage(size)
			off := (tmp.Bounds().Dx() - size) / 2
			op1 := &ebiten.DrawImageOptions{}
			op1.ColorScale.ScaleAlpha(1 - fade)
			op1.Blend = ebiten.BlendCopy
			op1.GeoM.Translate(float64(off), float64(off))
			tmp.DrawImage(prevImg, op1)
			op2 := &ebiten.DrawImageOptions{}
			op2.ColorScale.ScaleAlpha(fade)
			op2.Blend = ebiten.BlendLighter
			op2.GeoM.Translate(float64(off), float64(off))
			tmp.DrawImage(img, op2)
			op := &ebiten.DrawImageOptions{}
			op.Filter = drawFilter
			op.GeoM.Scale(float64(scale), float64(scale))
			op.GeoM.Translate(float64(x-tmp.Bounds().Dx()*scale/2), float64(y-tmp.Bounds().Dy()*scale/2))
			screen.DrawImage(tmp, op)
			recycleTempImage(tmp)
		} else {
			op := &ebiten.DrawImageOptions{}
			op.Filter = drawFilter
			op.GeoM.Scale(float64(scale), float64(scale))
			op.GeoM.Translate(float64(x-size*scale/2), float64(y-size*scale/2))
			screen.DrawImage(img, op)
		}
		if d, ok := descMap[m.Index]; ok {
			if d.Name != "" {
				textClr, bgClr, frameClr := mobileNameColors(m.Colors)
				w, h := text.Measure(d.Name, mainFont, 0)
				iw := int(math.Ceil(w))
				ih := int(math.Ceil(h))
				top := y + ih + (4 * scale)
				left := x - iw/2
				ebitenutil.DrawRect(screen, float64(left), float64(top), float64(iw+5), float64(ih), bgClr)
				vector.StrokeRect(screen, float32(left), float32(top), float32(iw+5), float32(ih), 1, frameClr, false)
				op := &text.DrawOptions{}
				op.GeoM.Translate(float64(left+2), float64(top+2))
				op.ColorScale.ScaleWithColor(textClr)
				text.Draw(screen, d.Name, mainFont, op)
			} else {
				back := int((m.Colors >> 4) & 0x0f)
				if back != kColorCodeBackWhite && back != kColorCodeBackBlue && !(back == kColorCodeBackBlack && d.Type == kDescMonster) {
					if back >= len(nameBackColors) {
						back = 0
					}
					barClr := nameBackColors[back]
					top := y + size*scale/2 + 2*scale
					left := x - 6*scale
					ebitenutil.DrawRect(screen, float64(left), float64(top), float64(12*scale), float64(2*scale), barClr)
				}
			}
		}
		if showPlanes {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dm", plane)
			xPos := x - size*scale/2
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(xPos), float64(y-size*scale/2)-metrics.HAscent)
			op.ColorScale.ScaleWithColor(color.RGBA{0, 255, 255, 255})
			text.Draw(screen, lbl, mainFont, op)
		}
	} else {
		vector.DrawFilledRect(screen, float32(x-3*scale), float32(y-3*scale), float32(6*scale), float32(6*scale), color.RGBA{0xff, 0, 0, 0xff}, false)
		if showPlanes {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dm", plane)
			xPos := x - 3*scale
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(xPos), float64(y-3*scale)-metrics.HAscent)
			op.ColorScale.ScaleWithColor(color.White)
			text.Draw(screen, lbl, mainFont, op)
		}
	}
}

// drawPicture renders a single picture sprite.
func drawPicture(screen *ebiten.Image, p framePicture, alpha float64, fade float32, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, shiftX, shiftY int) {
	if hideMoving && p.Moving {
		return
	}
	offX := float64(int(p.PrevH)-int(p.H)) * (1 - alpha)
	offY := float64(int(p.PrevV)-int(p.V)) * (1 - alpha)
	if p.Moving && !smoothMoving {
		offX = 0
		offY = 0
	}

	frame := 0
	plane := 0
	if clImages != nil {
		frame = clImages.FrameIndex(uint32(p.PictID), frameCounter)
		plane = clImages.Plane(uint32(p.PictID))
	}

	img := loadImageFrame(p.PictID, frame)
	var prevImg *ebiten.Image
	if blendPicts && clImages != nil {
		prevFrame := clImages.FrameIndex(uint32(p.PictID), frameCounter-1)
		if prevFrame != frame {
			prevImg = loadImageFrame(p.PictID, prevFrame)
		}
	}

	var mobileX, mobileY float64
	w, h := 0, 0
	if img != nil {
		w, h = img.Bounds().Dx(), img.Bounds().Dy()
		if w <= 64 && h <= 64 && interp && smoothMoving {
			if dx, dy, ok := pictureMobileOffset(p, mobiles, prevMobiles, alpha, shiftX, shiftY); ok {
				mobileX, mobileY = dx, dy
				offX = 0
				offY = 0
			}
		}
	}

	x := (int(math.Round(float64(p.H)+offX+mobileX)) + fieldCenterX) * scale
	y := (int(math.Round(float64(p.V)+offY+mobileY)) + fieldCenterY) * scale

	if img != nil {
		if blendPicts && prevImg != nil {
			size := w
			if h > size {
				size = h
			}
			tmp := getTempImage(size)
			off := tmp.Bounds()
			offXPix := (off.Dx() - w) / 2
			offYPix := (off.Dy() - h) / 2
			op1 := &ebiten.DrawImageOptions{}
			op1.ColorScale.ScaleAlpha(1 - fade)
			op1.Blend = ebiten.BlendCopy
			op1.GeoM.Translate(float64(offXPix), float64(offYPix))
			tmp.DrawImage(prevImg, op1)
			op2 := &ebiten.DrawImageOptions{}
			op2.ColorScale.ScaleAlpha(fade)
			op2.Blend = ebiten.BlendLighter
			op2.GeoM.Translate(float64(offXPix), float64(offYPix))
			tmp.DrawImage(img, op2)
			op := &ebiten.DrawImageOptions{}
			op.Filter = drawFilter
			if linear {
				op.GeoM.Scale(float64(scale)+epsilon, float64(scale)+epsilon)
			} else {
				op.GeoM.Scale(float64(scale), float64(scale))
			}
			op.GeoM.Translate(float64(x-tmp.Bounds().Dx()*scale/2), float64(y-tmp.Bounds().Dy()*scale/2))
			screen.DrawImage(tmp, op)
			recycleTempImage(tmp)
		} else {
			op := &ebiten.DrawImageOptions{}
			op.Filter = drawFilter
			if linear {
				op.GeoM.Scale(float64(scale)+epsilon, float64(scale)+epsilon)
			} else {
				op.GeoM.Scale(float64(scale), float64(scale))
			}
			op.GeoM.Translate(float64(x-w*scale/2), float64(y-h*scale/2))
			if smoothDebug && p.Moving {
				op.ColorM.Scale(1, 0, 0, 1)
			}
			screen.DrawImage(img, op)
		}

		if showPlanes {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dp", plane)
			xPos := x - w*scale/2
			opTxt := &text.DrawOptions{}
			opTxt.GeoM.Translate(float64(xPos), float64(y-h*scale/2)-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(color.RGBA{255, 255, 0, 0})
			text.Draw(screen, lbl, mainFont, opTxt)
		}
	} else {
		clr := color.RGBA{0, 0, 0xff, 0xff}
		if smoothDebug && p.Moving {
			clr = color.RGBA{0xff, 0, 0, 0xff}
		}
		vector.DrawFilledRect(screen, float32(x-2*scale), float32(y-2*scale), float32(4*scale), float32(4*scale), clr, false)
		if showPlanes {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dp", plane)
			xPos := x - 2*scale
			opTxt := &text.DrawOptions{}
			opTxt.GeoM.Translate(float64(xPos), float64(y-2*scale)-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(color.RGBA{255, 255, 0, 0})
			text.Draw(screen, lbl, mainFont, opTxt)
		}
	}
}

// pictureMobileOffset returns the interpolated offset for a picture that
// aligns with a mobile which moved between frames.
func pictureMobileOffset(p framePicture, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, alpha float64, shiftX, shiftY int) (float64, float64, bool) {
	for _, m := range mobiles {
		if m.H == p.H && m.V == p.V {
			if pm, ok := prevMobiles[m.Index]; ok {
				dh := int(m.H) - int(pm.H) - shiftX
				dv := int(m.V) - int(pm.V) - shiftY
				if dh != 0 || dv != 0 {
					if dh*dh+dv*dv <= maxMobileInterpPixels*maxMobileInterpPixels {
						h := float64(pm.H)*(1-alpha) + float64(m.H)*alpha
						v := float64(pm.V)*(1-alpha) + float64(m.V)*alpha
						return h - float64(m.H), v - float64(m.V), true
					}
				}
			}
			break
		}
	}
	return 0, 0, false
}

// lerpBar interpolates status bar values, skipping interpolation when
// fastBars is enabled and the value decreases.
func lerpBar(prev, cur int, alpha float64) int {
	if fastBars && cur < prev {
		return cur
	}
	return int(math.Round(float64(prev) + alpha*float64(cur-prev)))
}

// drawStatusBars renders health, balance and spirit bars.
func drawStatusBars(screen *ebiten.Image, snap drawSnapshot, alpha float64) {
	if hudPixel == nil {
		hudPixel = ebiten.NewImage(1, 1)
		hudPixel.Fill(color.White)
	}
	drawRect := func(x, y, w, h int, clr color.RGBA) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(w), float64(h))
		op.GeoM.Translate(float64(x), float64(y))
		op.ColorM.Scale(float64(clr.R)/255, float64(clr.G)/255, float64(clr.B)/255, float64(clr.A)/255)
		screen.DrawImage(hudPixel, op)
	}
	barWidth := 110 * scale
	barHeight := 8 * scale
	fieldWidth := gameAreaSizeX * scale
	slot := (fieldWidth - 3*barWidth) / 6
	barY := gameAreaSizeY*scale - 20*scale - barHeight
	x := slot
	step := barWidth + 2*slot
	drawBar := func(x int, cur, max int, clr color.RGBA) {
		frameClr := color.RGBA{0xff, 0xff, 0xff, 0xff}
		vector.StrokeRect(screen, float32(x-scale), float32(barY-scale), float32(barWidth+2*scale), float32(barHeight+2*scale), 1, frameClr, false)
		if max > 0 && cur > 0 {
			w := barWidth * cur / max
			fillClr := color.RGBA{clr.R, clr.G, clr.B, 128}
			drawRect(x, barY, w, barHeight, fillClr)
		}
	}
	hp := lerpBar(snap.prevHP, snap.hp, alpha)
	hpMax := lerpBar(snap.prevHPMax, snap.hpMax, alpha)
	drawBar(x, hp, hpMax, color.RGBA{0x00, 0xff, 0, 0xff})
	x += step
	bal := lerpBar(snap.prevBalance, snap.balance, alpha)
	balMax := lerpBar(snap.prevBalanceMax, snap.balanceMax, alpha)
	drawBar(x, bal, balMax, color.RGBA{0x00, 0x00, 0xff, 0xff})
	x += step
	sp := lerpBar(snap.prevSP, snap.sp, alpha)
	spMax := lerpBar(snap.prevSPMax, snap.spMax, alpha)
	drawBar(x, sp, spMax, color.RGBA{0xff, 0x00, 0x00, 0xff})
}

// drawMessages prints chat messages on the HUD.
func drawMessages(screen *ebiten.Image, msgs []string) {
	y := (gameAreaSizeY - 50) * scale
	maxWidth := float64(gameAreaSizeX*scale - 8*scale)
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		width, lines := wrapText(msg, mainFont, maxWidth)
		iw := width + 8*scale + 4
		ih := 14 * scale
		for j := len(lines) - 1; j >= 0; j-- {
			y -= 15 * scale
			ebitenutil.DrawRect(screen, 0, float64(y), float64(iw), float64(ih), color.RGBA{0, 0, 0, 128})
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(4*scale), float64(y))
			op.ColorScale.ScaleWithColor(color.White)
			text.Draw(screen, lines[j], mainFont, op)
		}
	}
}

func drawServerFPS(screen *ebiten.Image, fps int) {
	if fps <= 0 {
		return
	}
	msg := fmt.Sprintf("FPS: %v UPS: %d", ebiten.ActualFPS(), fps)
	w, _ := text.Measure(msg, mainFont, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(gameAreaSizeX*scale)-w-float64(4*scale), float64(4*scale))
	op.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, msg, mainFont, op)
}

// drawInputOverlay renders the text entry box when chatting.
func drawInputOverlay(screen *ebiten.Image, txt string) {
	if inputBg == nil {
		inputBg = ebiten.NewImage(gameAreaSizeX*scale, 12*scale)
		inputBg.Fill(color.RGBA{0, 0, 0, 128})
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, float64(gameAreaSizeY*scale-(12+1)*scale))
	screen.DrawImage(inputBg, op)
	top := gameAreaSizeY*scale - (12+2)*scale
	opTxt := &text.DrawOptions{}
	opTxt.GeoM.Translate(float64(4*scale), float64(top))
	opTxt.ColorScale.ScaleWithColor(color.White)
	text.Draw(screen, txt, mainFont, opTxt)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	eui.Layout(gameAreaSizeX*scale, gameAreaSizeY*scale)
	return gameAreaSizeX * scale, gameAreaSizeY * scale
}

func runGame(ctx context.Context) {
	gameCtx = ctx
	eui.SetUIScale(1)
	initUI()

	ebiten.SetWindowSize(gameAreaSizeX*scale, gameAreaSizeY*scale)
	ebiten.SetWindowTitle("ThoomSpeak")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetVsyncEnabled(vsync)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebiten.SetCursorShape(ebiten.CursorShapeDefault)

	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Printf("ebiten: %v", err)
	}
}

func noteFrame() {
	now := time.Now()
	frameMu.Lock()
	if !lastFrameTime.IsZero() {
		dt := now.Sub(lastFrameTime)
		ms := int(dt.Round(10*time.Millisecond) / time.Millisecond)
		if ms > 0 {
			intervalHist[ms]++
			var modeMS, modeCount int
			for v, c := range intervalHist {
				if c > modeCount {
					modeMS, modeCount = v, c
				}
			}
			if modeMS > 0 {
				fps := int(math.Round(1000.0 / float64(modeMS)))
				if fps < 1 {
					fps = 1
				}
				serverFPS = fps
				frameInterval = time.Second / time.Duration(fps)
			}
		}
	}
	lastFrameTime = now
	frameMu.Unlock()
	select {
	case frameCh <- struct{}{}:
	default:
	}
}

func sendInputLoop(ctx context.Context, conn net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-frameCh:
		}
		frameMu.Lock()
		interval := frameInterval
		last := lastFrameTime
		frameMu.Unlock()
		if time.Since(last) > 2*time.Second || conn == nil {
			continue
		}
		delay := interval / 2
		if delay <= 0 {
			delay = 200 * time.Millisecond
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		frameMu.Lock()
		last = lastFrameTime
		frameMu.Unlock()
		if time.Since(last) > 2*time.Second || conn == nil {
			continue
		}
		if err := sendPlayerInput(conn); err != nil {
			logError("send player input: %v", err)
		}
	}
}

func udpReadLoop(ctx context.Context, conn net.Conn) {
	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			logError("udp deadline: %v", err)
			return
		}
		m, err := readUDPMessage(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			logError("udp read error: %v", err)
			return
		}
		tag := binary.BigEndian.Uint16(m[:2])
		if tag == 2 { // kMsgDrawState
			noteFrame()
			handleDrawState(m)
			continue
		}
		if txt := decodeMessage(m); txt != "" {
			addMessage("udpReadLoop: decodeMessage: " + txt)
		} else {
			logDebug("udp msg tag %d len %d", tag, len(m))
		}
	}
}

func tcpReadLoop(ctx context.Context, conn net.Conn) {
loop:
	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			logError("set read deadline: %v", err)
			break
		}
		m, err := readTCPMessage(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					break loop
				default:
					continue
				}
			}
			logError("read error: %v", err)
			break
		}
		tag := binary.BigEndian.Uint16(m[:2])
		if tag == 2 { // kMsgDrawState
			noteFrame()
			handleDrawState(m)
			continue
		}
		if txt := decodeMessage(m); txt != "" {
			//fmt.Println(txt)
			addMessage("tcpReadLoop: decodeMessage: " + txt)
		} else {
			logDebug("msg tag %d len %d", tag, len(m))
		}
		select {
		case <-ctx.Done():
			break loop
		default:
		}
	}
}
