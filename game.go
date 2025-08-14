package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image/color"
	"log"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	dark "github.com/thiagokokada/dark-mode-go"
)

const lateRatio = 85
const gameAreaSizeX, gameAreaSizeY = 547, 540
const fieldCenterX, fieldCenterY = gameAreaSizeX / 2, gameAreaSizeY / 2

const initialWindowW, initialWindowH = 100, 100

var blackPixel *ebiten.Image
var offscreen *ebiten.Image

// scaleForFiltering returns adjusted scale values for width and height to reduce
// filtering seams. If either dimension is zero, the original scale is returned
// unchanged to avoid division by zero on the half-texel offset.
func scaleForFiltering(scale float64, w, h int) (float64, float64) {
	if w == 0 || h == 0 {
		// Zero-sized image: keep the original scale.
		return scale, scale
	}

	ps, exact := exactScale(scale, 8, 1e-6) // denom ≤ 8, ε = 1e-6

	if exact {
		// Exact integer or exact small rational: no offset needed.
		return ps, ps
	}

	// Not exact → keep requested scale but nudge by half-texel to reduce seams.
	return scale + 0.5/float64(w), scale + 0.5/float64(h)
}

func exactScale(scale float64, maxDenom int, eps float64) (float64, bool) {
	// Exact integer?
	r := math.Round(scale)
	if math.Abs(scale-r) <= eps {
		return r, true
	}

	// Exact small rational num/den?
	// We look for a den <= maxDenom where num/den ≈ scale within eps.
	best := scale
	for den := 2; den <= maxDenom; den++ {
		num := math.Round(scale * float64(den))
		ideal := num / float64(den)
		if math.Abs(scale-ideal) <= eps {
			best = ideal
			return best, true
		}
	}
	return best, false
}

type inputState struct {
	mouseX, mouseY int16
	mouseDown      bool
}

var (
	latestInput inputState
	inputMu     sync.Mutex
)

var keyWalk bool
var keyX, keyY int16
var walkToggled bool
var walkTargetX, walkTargetY int16

var inputActive bool
var inputText []rune
var inputHistory []string
var historyPos int

var (
	recorder            *movieRecorder
	gPlayersListIsStale bool
	loginGameState      []byte
	loginMobileData     []byte
	loginPictureTable   []byte
	wroteLoginBlocks    bool
)

// gameWin represents the main playfield window. Its size corresponds to the
// classic client field box (547×540) defined in old_mac_client/client/source/
// GameWin_cl.cp and Public_cl.h (Layout.layoFieldBox).
var gameWin *eui.WindowData
var settingsWin *eui.WindowData
var debugWin *eui.WindowData
var qualityWin *eui.WindowData
var graphicsWin *eui.WindowData
var soundWin *eui.WindowData
var gameCtx context.Context
var frameCounter int
var gameStarted = make(chan struct{})

var (
	frameCh       = make(chan struct{}, 1)
	lastFrameTime time.Time
	frameInterval = 200 * time.Millisecond
	intervalHist  = map[int]int{}
	frameMu       sync.Mutex
	serverFPS     float64
	netLatency    time.Duration
	lastInputSent time.Time
	latencyMu     sync.Mutex
)

// drawState tracks information needed by the Ebiten renderer.
type drawState struct {
	descriptors map[uint8]frameDescriptor
	pictures    []framePicture
	picShiftX   int
	picShiftY   int
	// worldShiftX/Y accumulate pictureShift over frames when stable.
	// Used to compute relative (anchored) positions for background stability.
	worldShiftX int
	worldShiftY int
	// Stability tracker for background sprites across frames
	bgStable    map[bgKey]bgStableInfo
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
	ackCmd                      uint8
	lightingFlags               uint8

	// Fallback handling for transient pictureShift misses
	lastGoodShiftX int
	lastGoodShiftY int
	missStreak     int
}

var (
	state = drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
	initialState drawState
	stateMu      sync.Mutex
)

// bubble stores temporary bubble debug information.
type bubble struct {
	Index       uint8
	H, V        int16
	Far         bool
	NoArrow     bool
	Text        string
	Type        int
	ExpireFrame int
}

// drawSnapshot is a read-only copy of the current draw state.
type drawSnapshot struct {
	descriptors                 map[uint8]frameDescriptor
	pictures                    []framePicture
	picShiftX                   int
	picShiftY                   int
	worldShiftX                 int
	worldShiftY                 int
	bgStable                    map[bgKey]bgStableInfo
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
	ackCmd                      uint8
	lightingFlags               uint8
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
		worldShiftX:    state.worldShiftX,
		worldShiftY:    state.worldShiftY,
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
		ackCmd:         state.ackCmd,
		lightingFlags:  state.lightingFlags,
	}

	for idx, d := range state.descriptors {
		snap.descriptors[idx] = d
	}
	for _, m := range state.mobiles {
		snap.mobiles = append(snap.mobiles, m)
	}
	if state.bgStable != nil {
		snap.bgStable = make(map[bgKey]bgStableInfo, len(state.bgStable))
		for k, v := range state.bgStable {
			snap.bgStable[k] = v
		}
	}
	if len(state.bubbles) > 0 {
		curFrame := frameCounter
		kept := state.bubbles[:0]
		for _, b := range state.bubbles {
			if b.ExpireFrame > curFrame {
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
	if gs.MotionSmoothing || gs.BlendMobiles {
		snap.prevMobiles = make(map[uint8]frameMobile, len(state.prevMobiles))
		for idx, m := range state.prevMobiles {
			snap.prevMobiles[idx] = m
		}
	}
	if gs.BlendMobiles {
		snap.prevDescs = make(map[uint8]frameDescriptor, len(state.prevDescs))
		for idx, d := range state.prevDescs {
			snap.prevDescs[idx] = d
		}
	}
	return snap
}

// cloneDrawState makes a deep copy of a drawState.
func cloneDrawState(src drawState) drawState {
	dst := drawState{
		descriptors:    make(map[uint8]frameDescriptor, len(src.descriptors)),
		pictures:       append([]framePicture(nil), src.pictures...),
		picShiftX:      src.picShiftX,
		picShiftY:      src.picShiftY,
		mobiles:        make(map[uint8]frameMobile, len(src.mobiles)),
		prevMobiles:    make(map[uint8]frameMobile, len(src.prevMobiles)),
		prevDescs:      make(map[uint8]frameDescriptor, len(src.prevDescs)),
		prevTime:       src.prevTime,
		curTime:        src.curTime,
		bubbles:        append([]bubble(nil), src.bubbles...),
		hp:             src.hp,
		hpMax:          src.hpMax,
		sp:             src.sp,
		spMax:          src.spMax,
		balance:        src.balance,
		balanceMax:     src.balanceMax,
		prevHP:         src.prevHP,
		prevHPMax:      src.prevHPMax,
		prevSP:         src.prevSP,
		prevSPMax:      src.prevSPMax,
		prevBalance:    src.prevBalance,
		prevBalanceMax: src.prevBalanceMax,
		ackCmd:         src.ackCmd,
		lightingFlags:  src.lightingFlags,
	}
	for idx, d := range src.descriptors {
		dst.descriptors[idx] = d
	}
	for idx, m := range src.mobiles {
		dst.mobiles[idx] = m
	}
	for idx, m := range src.prevMobiles {
		dst.prevMobiles[idx] = m
	}
	for idx, d := range src.prevDescs {
		dst.prevDescs[idx] = d
	}
	return dst
}

// computeInterpolation returns the blend factors for frame interpolation and onion skinning.
// It returns separate fade values for mobiles and pictures based on their respective rates.
func computeInterpolation(prevTime, curTime time.Time, mobileRate, pictRate float64) (alpha float64, mobileFade, pictFade float32) {
	alpha = 1.0
	mobileFade = 1.0
	pictFade = 1.0
	if (gs.MotionSmoothing || gs.BlendMobiles || gs.BlendPicts) && !curTime.IsZero() && curTime.After(prevTime) {
		elapsed := time.Since(prevTime)
		interval := curTime.Sub(prevTime)
		if gs.MotionSmoothing {
			alpha = float64(elapsed) / float64(interval)
			if alpha < 0 {
				alpha = 0
			}
			if alpha > 1 {
				alpha = 1
			}
		}
		if gs.BlendMobiles {
			half := float64(interval) * mobileRate
			if half > 0 {
				mobileFade = float32(float64(elapsed) / float64(half))
			}
			if mobileFade < 0 {
				mobileFade = 0
			}
			if mobileFade > 1 {
				mobileFade = 1
			}
		}
		if gs.BlendPicts {
			half := float64(interval) * pictRate
			if half > 0 {
				pictFade = float32(float64(elapsed) / float64(half))
			}
			if pictFade < 0 {
				pictFade = 0
			}
			if pictFade > 1 {
				pictFade = 1
			}
		}
	}
	return alpha, mobileFade, pictFade
}

type Game struct{}

var once sync.Once

func (g *Game) Update() error {
	eui.Update()

	once.Do(func() {
		initGame()
	})

	if inventoryDirty {
		updateInventoryWindow()
		inventoryDirty = false
	}

	if syncWindowSettings() {
		settingsDirty = true
	}
	if settingsDirty && qualityPresetDD != nil {
		qualityPresetDD.Selected = detectQualityPreset()
	}
	if time.Since(lastSettingsSave) >= 5*time.Second {
		if settingsDirty {
			saveSettings()
			settingsDirty = false
		}
		lastSettingsSave = time.Now()
	}

	if inputActive {
		inputText = append(inputText, ebiten.AppendInputChars(nil)...)
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			if len(inputHistory) > 0 {
				if historyPos > 0 {
					historyPos--
				} else {
					historyPos = 0
				}
				inputText = []rune(inputHistory[historyPos])
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			if len(inputHistory) > 0 {
				if historyPos < len(inputHistory)-1 {
					historyPos++
					inputText = []rune(inputHistory[historyPos])
				} else {
					historyPos = len(inputHistory)
					inputText = inputText[:0]
				}
			}
		}
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
				if strings.HasPrefix(txt, "/play ") {
					playTuneSimple(strings.TrimSpace(txt[len("/play "):]))
				} else {
					pendingCommand = txt
					//consoleMessage("> " + txt)
				}
				inputHistory = append(inputHistory, txt)
			}
			inputActive = false
			inputText = inputText[:0]
			historyPos = len(inputHistory)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			inputActive = false
			inputText = inputText[:0]
			historyPos = len(inputHistory)
		}
	} else {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			inputActive = true
			inputText = inputText[:0]
			historyPos = len(inputHistory)
		}
	}

	updateConsoleWindow()

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
			speed := gs.KBWalkSpeed
			if ebiten.IsKeyPressed(ebiten.KeyShift) {
				speed = 1.0
			}
			keyX = int16(float64(dx) * float64(fieldCenterX) * speed)
			keyY = int16(float64(dy) * float64(fieldCenterY) * speed)
		} else {
			keyWalk = false
		}

		mx, my := ebiten.CursorPosition()
		overUI := pointInUI(mx, my)
		gx, gy := gameWindowOrigin()

		// Debug wheel zoom: centered zoom that disables curtain
		if gs.DebugZoomEnabled {
			_, wy := ebiten.Wheel()
			if wy != 0 {
				// Exponential step for smooth scaling
				factor := math.Pow(1.1, float64(wy))
				nz := gs.DebugZoom * factor
				if nz < 0.25 {
					nz = 0.25
				} else if nz > 4.0 {
					nz = 4.0
				}
				if nz != gs.DebugZoom {
					gs.DebugZoom = nz
					settingsDirty = true
				}
			}
		}

		if gs.ClickToToggle {
			if !overUI && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				if walkToggled {
					walkToggled = false
				} else {
					effScale := gs.GameScale
					if gs.DebugZoomEnabled {
						effScale = gs.GameScale * gs.DebugZoom
					}
					walkTargetX = int16(float64(mx-gx)/effScale - float64(fieldCenterX))
					walkTargetY = int16(float64(my-gy)/effScale - float64(fieldCenterY))
					walkToggled = true
				}
			}
			if walkToggled {
				if gameWin == nil {
					walkToggled = false
				} else {
					size := gameWin.GetSize()
					x1 := gx + int(size.X)
					y1 := gy + int(size.Y)
					if overUI || mx < gx || my < gy || mx >= x1 || my >= y1 {
						walkToggled = false
					} else {
						effScale := gs.GameScale
						if gs.DebugZoomEnabled {
							effScale = gs.GameScale * gs.DebugZoom
						}
						walkTargetX = int16(float64(mx-gx)/effScale - float64(fieldCenterX))
						walkTargetY = int16(float64(my-gy)/effScale - float64(fieldCenterY))
					}
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

	mx, my := ebiten.CursorPosition()
	gx, gy := gameWindowOrigin()
	effScale := gs.GameScale
	if gs.DebugZoomEnabled {
		effScale = gs.GameScale * gs.DebugZoom
	}
	baseX := int16(float64(mx-gx)/effScale - float64(fieldCenterX))
	baseY := int16(float64(my-gy)/effScale - float64(fieldCenterY))
	baseDown := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	if pointInUI(mx, my) {
		baseDown = false
	}
	x, y := baseX, baseY
	down := baseDown
	if keyWalk {
		x, y, down = keyX, keyY, true
	} else if gs.ClickToToggle {
		x, y = walkTargetX, walkTargetY
		down = walkToggled
	}
	if down && !keyWalk {
		ebiten.SetCursorShape(ebiten.CursorShapeCrosshair)
	} else {
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
	}
	inputMu.Lock()
	latestInput = inputState{mouseX: x, mouseY: y, mouseDown: down}
	inputMu.Unlock()

	return nil
}

func updateGameScale() {
	if gameWin == nil {
		return
	}
	size := gameWin.GetRawSize()
	pad := float64(2 * gameWin.Padding)
	w := float64(size.X) - pad
	h := float64(size.Y) - pad
	if w <= 0 || h <= 0 {
		return
	}

	scaleW := w / float64(gameAreaSizeX)
	scaleH := h / float64(gameAreaSizeY)
	newScale := math.Min(scaleW, scaleH)
	if newScale < 0.25 {
		newScale = 0.25
	}

	if gs.AnyGameWindowSize {
		if gs.GameScale != newScale {
			gs.GameScale = newScale
			initFont()
		}
		updateGameWindowSize()
		return
	}

	snapped, exact := exactScale(newScale, 8, 1e-6)
	if !exact {
		snapped = math.Max(1, math.Round(newScale))
	}

	if gs.GameScale != snapped {
		gs.GameScale = snapped
		initFont()
	}
}

func updateGameWindowSize() {
	if gameWin == nil {
		return
	}
	if gs.AnyGameWindowSize {
		size := gameWin.GetRawSize()
		desiredW := int(math.Round(float64(size.X)))
		desiredH := int(math.Round(float64(size.Y)))
		gameWin.SetSize(eui.Point{X: float32(desiredW), Y: float32(desiredH)})
		return
	}
	scale := float32(gs.GameScale)
	desiredSize := eui.Point{
		X: float32(gameAreaSizeX)*scale + 2*gameWin.Padding,
		Y: float32(gameAreaSizeY)*scale + 2*gameWin.Padding,
	}
	gameWin.SetSize(desiredSize)
}

func gameWindowOrigin() (int, int) {
	if gameWin == nil {
		return 0, 0
	}
	pos := gameWin.GetRawPos()
	frame := gameWin.Margin + gameWin.Border + gameWin.BorderPad + gameWin.Padding
	x := pos.X + frame
	y := pos.Y + frame + gameWin.GetRawTitleSize()
	return int(x), int(y)
}

func gameContentOrigin() (int, int) {
	x, y := gameWindowOrigin()
	if gameWin == nil {
		return x, y
	}
	size := gameWin.GetSize()
	pad := float64(2 * gameWin.Padding)
	w := float64(int(size.X)&^1) - pad
	h := float64(int(size.Y)&^1) - pad
	fw := float64(gameAreaSizeX) * gs.GameScale
	fh := float64(gameAreaSizeY) * gs.GameScale
	if w > fw {
		x += int(math.Round((w - fw) / 2))
	}
	if h > fh {
		y += int(math.Round((h - fh) / 2))
	}
	return x, y
}

func (g *Game) Draw(screen *ebiten.Image) {
	if clmov == "" && tcpConn == nil {
		ox, oy := gameContentOrigin()
		drawSplash(screen, ox, oy)
		eui.Draw(screen)
		if gs.ShowFPS {
			drawServerFPS(screen, screen.Bounds().Dx()-40, 4, serverFPS)
		}
		return
	}
	snap := captureDrawSnapshot()
	alpha, mobileFade, pictFade := computeInterpolation(snap.prevTime, snap.curTime, gs.MobileBlendAmount, gs.BlendAmount)

	if gs.AnyGameWindowSize {
		updateGameScale()
		if offscreen == nil {
			offscreen = newImage(gameAreaSizeX*2, gameAreaSizeY*2)
		}
		offscreen.Clear()
		saved := gs.GameScale
		gs.GameScale = 2
		initFont()
		drawScene(offscreen, 0, 0, snap, alpha, mobileFade, pictFade)
		if gs.nightEffect {
			drawNightOverlay(offscreen, 0, 0)
		}
		// drawEquippedItems(offscreen, 0, 0) // disabled for now
		drawGameCurtain(offscreen, 0, 0)
		drawStatusBars(offscreen, 0, 0, snap, alpha)
		gs.GameScale = saved
		initFont()
		ox, oy := gameContentOrigin()
		size := gameWin.GetSize()
		pad := float64(2 * gameWin.Padding)
		scaleW := (float64(size.X) - pad) / (gameAreaSizeX * 2)
		scaleH := (float64(size.Y) - pad) / (gameAreaSizeY * 2)
		scale := math.Min(scaleW, scaleH)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(float64(ox), float64(oy))
		screen.DrawImage(offscreen, op)
		eui.Draw(screen)
		if gs.ShowFPS {
			drawServerFPS(screen, screen.Bounds().Dx()-40, 4, serverFPS)
		}
		return
	}

	// Apply debug zoom by temporarily scaling GameScale for the draw pass.
	savedScale := gs.GameScale
	if gs.DebugZoomEnabled {
		gs.GameScale = savedScale * gs.DebugZoom
	}
	ox, oy := gameContentOrigin()
	drawScene(screen, ox, oy, snap, alpha, mobileFade, pictFade)
	if gs.nightEffect {
		drawNightOverlay(screen, ox, oy)
	}
	// drawEquippedItems(screen, ox, oy) // disabled for now
	if !gs.DebugZoomEnabled { // disable blackout curtain during debug zoom
		drawGameCurtain(screen, ox, oy)
	}
	drawStatusBars(screen, ox, oy, snap, alpha)
	// Restore base scale and update for any window changes
	if gs.DebugZoomEnabled {
		gs.GameScale = savedScale
	}
	updateGameScale()
	eui.Draw(screen)
	if gs.ShowFPS {
		drawServerFPS(screen, screen.Bounds().Dx()-40, 4, serverFPS)
	}
}

// drawScene renders all world objects for the current frame.
func drawScene(screen *ebiten.Image, ox, oy int, snap drawSnapshot, alpha float64, mobileFade, pictFade float32) {
	descSlice := make([]frameDescriptor, 0, len(snap.descriptors))
	for _, d := range snap.descriptors {
		descSlice = append(descSlice, d)
	}
	sortDescriptors(descSlice)
	descMap := make(map[uint8]frameDescriptor, len(descSlice))
	for _, d := range descSlice {
		descMap[d.Index] = d
	}

	sortPictures(snap.pictures)

	dead := make([]frameMobile, 0, len(snap.mobiles))
	live := make([]frameMobile, 0, len(snap.mobiles))
	for _, m := range snap.mobiles {
		if m.State == poseDead {
			dead = append(dead, m)
		}
		live = append(live, m)
	}
	sortMobiles(dead)
	sortMobiles(live)

	negPics := make([]framePicture, 0)
	zeroPics := make([]framePicture, 0)
	posPics := make([]framePicture, 0)
	for _, p := range snap.pictures {
		switch {
		case p.Plane < 0:
			negPics = append(negPics, p)
		case p.Plane == 0:
			zeroPics = append(zeroPics, p)
		default:
			posPics = append(posPics, p)
		}
	}

	for _, p := range negPics {
		drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY)
	}

	if gs.hideMobiles {
		for _, p := range zeroPics {
			drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY)
		}
	} else {
		for _, m := range dead {
			drawMobile(screen, ox, oy, m, descMap, snap.prevMobiles, snap.prevDescs, snap.picShiftX, snap.picShiftY, alpha, mobileFade)
		}
		i, j := 0, 0
		maxInt := int(^uint(0) >> 1)
		for i < len(live) || j < len(zeroPics) {
			mV, mH := maxInt, maxInt
			if i < len(live) {
				mV = int(live[i].V)
				mH = int(live[i].H)
			}
			pV, pH := maxInt, maxInt
			if j < len(zeroPics) {
				pV = int(zeroPics[j].V)
				pH = int(zeroPics[j].H)
			}
			if mV < pV || (mV == pV && mH <= pH) {
				if live[i].State != poseDead {
					drawMobile(screen, ox, oy, live[i], descMap, snap.prevMobiles, snap.prevDescs, snap.picShiftX, snap.picShiftY, alpha, mobileFade)
				}
				i++
			} else {
				drawPicture(screen, ox, oy, zeroPics[j], alpha, pictFade, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY)
				j++
			}
		}
	}

	for _, p := range posPics {
		drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY)
	}

	if gs.SpeechBubbles {
		for _, b := range snap.bubbles {
			hpos := float64(b.H)
			vpos := float64(b.V)
			var img *ebiten.Image
			if !b.Far {
				var m *frameMobile
				for i := range snap.mobiles {
					if snap.mobiles[i].Index == b.Index {
						m = &snap.mobiles[i]
						break
					}
				}
				if m != nil {
					if d, ok := descMap[m.Index]; ok {
						colors := d.Colors
						playersMu.RLock()
						if p, ok := players[d.Name]; ok && len(p.Colors) > 0 {
							colors = append([]byte(nil), p.Colors...)
						}
						playersMu.RUnlock()
						img = loadMobileFrame(d.PictID, m.State, colors)
					}
					hpos = float64(m.H)
					vpos = float64(m.V)
					if gs.MotionSmoothing {
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
			x := int((math.Round(hpos) + float64(fieldCenterX)) * gs.GameScale)
			y := int((math.Round(vpos) + float64(fieldCenterY)) * gs.GameScale)
			if !b.Far {
				if d, ok := descMap[b.Index]; ok {
					if size := mobileSize(d.PictID); size > 0 {
						scaled := math.Round(float64(size) * gs.GameScale)
						tailHeight := int(10 * gs.GameScale)
						y += tailHeight - int(scaled/2)
					}
				}
			}
			x += ox
			y += oy
			if img != nil && !b.Far {
				y -= int(float64(img.Bounds().Dy()) * gs.GameScale / 2)
			}
			borderCol, bgCol, textCol := bubbleColors(b.Type)
			drawBubble(screen, b.Text, x, y, b.Type, b.Far, b.NoArrow, borderCol, bgCol, textCol)
		}
	}
}

// drawMobile renders a single mobile object with optional interpolation and onion skinning.
func drawMobile(screen *ebiten.Image, ox, oy int, m frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, prevDescs map[uint8]frameDescriptor, shiftX, shiftY int, alpha float64, fade float32) {
	h := float64(m.H)
	v := float64(m.V)
	if gs.MotionSmoothing {
		if pm, ok := prevMobiles[m.Index]; ok {
			dh := int(m.H) - int(pm.H) - shiftX
			dv := int(m.V) - int(pm.V) - shiftY
			if dh*dh+dv*dv <= maxMobileInterpPixels*maxMobileInterpPixels {
				h = float64(pm.H)*(1-alpha) + float64(m.H)*alpha
				v = float64(pm.V)*(1-alpha) + float64(m.V)*alpha
			}
		}
	}
	x := int((math.Round(h) + float64(fieldCenterX)) * gs.GameScale)
	y := int((math.Round(v) + float64(fieldCenterY)) * gs.GameScale)
	x += ox
	y += oy
	viewW := int(float64(gameAreaSizeX) * gs.GameScale)
	viewH := int(float64(gameAreaSizeY) * gs.GameScale)
	var img *ebiten.Image
	plane := 0
	var d frameDescriptor
	var colors []byte
	var state uint8
	if desc, ok := descMap[m.Index]; ok {
		d = desc
		colors = d.Colors
		playersMu.RLock()
		if p, ok := players[d.Name]; ok && len(p.Colors) > 0 {
			colors = append([]byte(nil), p.Colors...)
		}
		playersMu.RUnlock()
		state = m.State
		img = loadMobileFrame(d.PictID, state, colors)
		if clImages != nil {
			plane = clImages.Plane(uint32(d.PictID))
		}
	}
	var prevImg *ebiten.Image
	var prevColors []byte
	var prevPict uint16
	var prevState uint8
	if gs.BlendMobiles {
		if pm, ok := prevMobiles[m.Index]; ok {
			pd := descMap[m.Index]
			if d, ok := prevDescs[m.Index]; ok {
				pd = d
			}
			prevColors = pd.Colors
			playersMu.RLock()
			if p, ok := players[pd.Name]; ok && len(p.Colors) > 0 {
				prevColors = append([]byte(nil), p.Colors...)
			}
			playersMu.RUnlock()
			prevImg = loadMobileFrame(pd.PictID, pm.State, prevColors)
			prevPict = pd.PictID
			prevState = pm.State
		}
	}
	if img != nil {
		size := img.Bounds().Dx()
		blend := gs.BlendMobiles && prevImg != nil && fade > 0 && fade < 1
		var src *ebiten.Image
		drawSize := size
		if blend {
			steps := gs.MobileBlendFrames
			idx := int(fade * float32(steps))
			if idx <= 0 {
				idx = 1
			}
			if idx >= steps {
				idx = steps - 1
			}
			prevKey := makeMobileKey(prevPict, prevState, prevColors)
			curKey := makeMobileKey(d.PictID, state, colors)
			if b := mobileBlendFrame(prevKey, curKey, prevImg, img, idx, steps); b != nil {
				src = b
				drawSize = b.Bounds().Dx()
			} else {
				src = img
			}
		} else if gs.BlendMobiles && prevImg != nil {
			if fade <= 0 {
				src = prevImg
				drawSize = prevImg.Bounds().Dx()
			} else {
				src = img
			}
		} else {
			src = img
		}
		scale := gs.GameScale
		scaled := math.Round(float64(drawSize) * scale)
		scale = scaled / float64(drawSize)
		half := int(scaled / 2)
		if !gs.DebugZoomEnabled {
			if x+half <= ox || y+half <= oy || x-half >= ox+viewW || y-half >= oy+viewH {
				return
			}
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		op.GeoM.Scale(scale, scale)
		tx := math.Round(float64(x) - scaled/2)
		ty := math.Round(float64(y) - scaled/2)
		op.GeoM.Translate(tx, ty)
		screen.DrawImage(src, op)
		if d, ok := descMap[m.Index]; ok {
			alpha := uint8(gs.NameBgOpacity * 255)
			if d.Name != "" {
				textClr, bgClr, frameClr := mobileNameColors(m.Colors)
				bgClr.A = alpha
				frameClr.A = alpha
				w, h := text.Measure(d.Name, mainFont, 0)
				iw := int(math.Ceil(w))
				ih := int(math.Ceil(h))
				top := y + int(20*gs.GameScale)
				left := x - iw/2
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(float64(iw+5), float64(ih))
				op.GeoM.Translate(float64(left), float64(top))
				op.ColorScale.ScaleWithColor(bgClr)
				screen.DrawImage(whiteImage, op)
				vector.StrokeRect(screen, float32(left), float32(top), float32(iw+5), float32(ih), 1, frameClr, false)
				opTxt := &text.DrawOptions{}
				opTxt.GeoM.Translate(float64(left+2), float64(top+2))
				opTxt.ColorScale.ScaleWithColor(textClr)
				text.Draw(screen, d.Name, mainFont, opTxt)
			} else {
				back := int((m.Colors >> 4) & 0x0f)
				if back != kColorCodeBackWhite && back != kColorCodeBackBlue && !(back == kColorCodeBackBlack && d.Type == kDescMonster) {
					if back >= len(nameBackColors) {
						back = 0
					}
					barClr := nameBackColors[back]
					barClr.A = alpha
					top := y + int(float64(size)*gs.GameScale/2+2*gs.GameScale)
					left := x - int(6*gs.GameScale)
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Scale(12*gs.GameScale, 2*gs.GameScale)
					op.GeoM.Translate(float64(left), float64(top))
					op.ColorScale.ScaleWithColor(barClr)
					screen.DrawImage(whiteImage, op)
				}
			}
		}
		if gs.imgPlanesDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dm", plane)
			xPos := x - int(float64(size)*gs.GameScale/2)
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(xPos), float64(y)-float64(size)*gs.GameScale/2-metrics.HAscent)
			op.ColorScale.ScaleWithColor(color.RGBA{0, 255, 255, 255})
			text.Draw(screen, lbl, mainFont, op)
		}
	} else {
		half := int(3 * gs.GameScale)
		if !gs.DebugZoomEnabled {
			if x+half <= ox || y+half <= oy || x-half >= ox+viewW || y-half >= oy+viewH {
				return
			}
		}
		vector.DrawFilledRect(screen, float32(float64(x)-3*gs.GameScale), float32(float64(y)-3*gs.GameScale), float32(6*gs.GameScale), float32(6*gs.GameScale), color.RGBA{0xff, 0, 0, 0xff}, false)
		if gs.imgPlanesDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dm", plane)
			xPos := x - int(3*gs.GameScale)
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(xPos), float64(y)-3*gs.GameScale-metrics.HAscent)
			op.ColorScale.ScaleWithColor(color.White)
			text.Draw(screen, lbl, mainFont, op)
		}
	}
}

// drawPicture renders a single picture sprite.
func drawPicture(screen *ebiten.Image, ox, oy int, p framePicture, alpha float64, fade float32, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, shiftX, shiftY int) {
	if gs.hideMoving && p.Moving {
		return
	}
	if p.Hidden {
		return
	}
	offX := float64(int(p.PrevH)-int(p.H)) * (1 - alpha)
	offY := float64(int(p.PrevV)-int(p.V)) * (1 - alpha)
	if p.Moving && !gs.smoothMoving {
		if int(p.PrevH) == int(p.H)-shiftX && int(p.PrevV) == int(p.V)-shiftY {
			if gs.dontShiftNewSprites {
				offX = 0
				offY = 0
			}
		} else {
			offX = 0
			offY = 0
		}
	}

	frame := 0
	if clImages != nil {
		frame = clImages.FrameIndex(uint32(p.PictID), frameCounter)
	}
	plane := p.Plane

	w, h := 0, 0
	if clImages != nil {
		w, h = clImages.Size(uint32(p.PictID))
	}

	var mobileX, mobileY float64
	if w <= 64 && h <= 64 && gs.MotionSmoothing && gs.smoothMoving {
		if dx, dy, ok := pictureMobileOffset(p, mobiles, prevMobiles, alpha, shiftX, shiftY); ok {
			mobileX, mobileY = dx, dy
			offX = 0
			offY = 0
		}
	}

	x := int((math.Round(float64(p.H)+offX+mobileX) + float64(fieldCenterX)) * gs.GameScale)
	y := int((math.Round(float64(p.V)+offY+mobileY) + float64(fieldCenterY)) * gs.GameScale)
	x += ox
	y += oy

	pfW := int(math.Round(float64(gameAreaSizeX) * gs.GameScale))
	pfH := int(math.Round(float64(gameAreaSizeY) * gs.GameScale))
	left, top := ox, oy
	right, bottom := ox+pfW, oy+pfH

	scaledW := int(math.Round(float64(w) * gs.GameScale))
	scaledH := int(math.Round(float64(h) * gs.GameScale))
	halfW := scaledW / 2
	halfH := scaledH / 2
	if !gs.DebugZoomEnabled {
		if x+halfW <= left || y+halfH <= top || x-halfW >= right || y-halfH >= bottom {
			return
		}
	}

	img := loadImageFrame(p.PictID, frame)
	var prevImg *ebiten.Image
	var prevFrame int
	if gs.BlendPicts && clImages != nil {
		prevFrame = clImages.FrameIndex(uint32(p.PictID), frameCounter-1)
		if prevFrame != frame {
			prevImg = loadImageFrame(p.PictID, prevFrame)
		}
	}

	if img != nil {
		drawW, drawH := w, h
		blend := gs.BlendPicts && prevImg != nil && fade > 0 && fade < 1
		var src *ebiten.Image
		if blend {
			steps := gs.PictBlendFrames
			idx := int(fade * float32(steps))
			if idx <= 0 {
				idx = 1
			}
			if idx >= steps {
				idx = steps - 1
			}
			if b := pictBlendFrame(p.PictID, prevFrame, frame, prevImg, img, idx, steps); b != nil {
				src = b
			} else {
				src = img
				blend = false
			}
		} else if gs.BlendPicts && prevImg != nil {
			if fade <= 0 {
				src = prevImg
			} else {
				src = img
			}
		} else {
			src = img
		}
		if src != nil {
			drawW, drawH = src.Bounds().Dx(), src.Bounds().Dy()
		}
		sx, sy := scaleForFiltering(gs.GameScale, drawW, drawH)
		scaledW := math.Round(float64(drawW) * sx)
		scaledH := math.Round(float64(drawH) * sy)
		sx = scaledW / float64(drawW)
		sy = scaledH / float64(drawH)
		halfW := int(scaledW / 2)
		halfH := int(scaledH / 2)
		if !gs.DebugZoomEnabled {
			if x+halfW <= left || y+halfH <= top || x-halfW >= right || y-halfH >= bottom {
				return
			}
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		op.GeoM.Scale(sx, sy)
		tx := math.Round(float64(x) - float64(drawW)*sx/2)
		ty := math.Round(float64(y) - float64(drawH)*sy/2)
		op.GeoM.Translate(tx, ty)
		if p.Ghost && gs.BGStabilityDebug {
			// Tint cyan to indicate a retained ghost sprite
			op.ColorScale.Scale(0, 1, 1, 1)
		} else if gs.pictAgainDebug && p.Again {
			op.ColorScale.Scale(0, 0, 1, 1)
		} else if src == img && gs.smoothingDebug && p.Moving {
			op.ColorScale.Scale(1, 0, 0, 1)
		}
		screen.DrawImage(src, op)

		// Debug overlay: background stability count
		if p.Background && gs.BGStabilityDebug {
			// Need access to world shift and bgStable: use current global state snapshot
			// We passed shift in drawScene; grab world shift via state snapshot
			// drawScene has a local 'snap', but not available here; recompute anchor via globals
			// Safer approach: compute via current state (minor race acceptable for debug).
			stateMu.Lock()
			wsx, wsy := state.worldShiftX, state.worldShiftY
			var cnt int
			if state.bgStable != nil {
				anchorH := int16(int(p.H) - wsx)
				anchorV := int16(int(p.V) - wsy)
				if info, ok := state.bgStable[bgKey{id: p.PictID, h: anchorH, v: anchorV}]; ok {
					cnt = info.count
				}
			}
			stateMu.Unlock()
			if cnt > 0 {
				lbl := fmt.Sprintf("%d", cnt)
				metrics := mainFont.Metrics()
				// position label above sprite
				xPos := x - int(float64(w)*gs.GameScale/2)
				opTxt := &text.DrawOptions{}
				opTxt.GeoM.Translate(float64(xPos), float64(y)-float64(h)*gs.GameScale/2-metrics.HAscent)
				opTxt.ColorScale.ScaleWithColor(color.RGBA{0, 255, 0, 255})
				text.Draw(screen, lbl, mainFont, opTxt)
			}
		}

		if gs.pictIDDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%d", p.PictID)
			txtW, _ := text.Measure(lbl, mainFont, 0)
			xPos := x + int(float64(w)*gs.GameScale/2) - int(math.Round(txtW))
			opTxt := &text.DrawOptions{}
			opTxt.GeoM.Translate(float64(xPos), float64(y)-float64(h)*gs.GameScale/2-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(color.Black)
			text.Draw(screen, lbl, mainFont, opTxt)
		}

		if gs.imgPlanesDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dp", plane)
			xPos := x - int(float64(w)*gs.GameScale/2)
			opTxt := &text.DrawOptions{}
			opTxt.GeoM.Translate(float64(xPos), float64(y)-float64(h)*gs.GameScale/2-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(color.RGBA{255, 255, 0, 0})
			text.Draw(screen, lbl, mainFont, opTxt)
		}
	} else {
		half := int(2 * gs.GameScale)
		if !gs.DebugZoomEnabled {
			if x+half <= left || y+half <= top || x-half >= right || y-half >= bottom {
				return
			}
		}
		clr := color.RGBA{0, 0, 0xff, 0xff}
		if p.Ghost && gs.BGStabilityDebug {
			clr = color.RGBA{0, 255, 255, 255}
		}
		if gs.smoothingDebug && p.Moving {
			clr = color.RGBA{0xff, 0, 0, 0xff}
		}
		if gs.pictAgainDebug && p.Again {
			clr = color.RGBA{0, 0, 0xff, 0xff}
		}
		vector.DrawFilledRect(screen, float32(float64(x)-2*gs.GameScale), float32(float64(y)-2*gs.GameScale), float32(4*gs.GameScale), float32(4*gs.GameScale), clr, false)
		if p.Background && gs.BGStabilityDebug {
			stateMu.Lock()
			wsx, wsy := state.worldShiftX, state.worldShiftY
			var cnt int
			if state.bgStable != nil {
				anchorH := int16(int(p.H) - wsx)
				anchorV := int16(int(p.V) - wsy)
				if info, ok := state.bgStable[bgKey{id: p.PictID, h: anchorH, v: anchorV}]; ok {
					cnt = info.count
				}
			}
			stateMu.Unlock()
			if cnt > 0 {
				lbl := fmt.Sprintf("%d", cnt)
				metrics := mainFont.Metrics()
				xPos := x - int(2*gs.GameScale)
				opTxt := &text.DrawOptions{}
				opTxt.GeoM.Translate(float64(xPos), float64(y)-2*gs.GameScale-metrics.HAscent)
				opTxt.ColorScale.ScaleWithColor(color.RGBA{0, 255, 0, 255})
				text.Draw(screen, lbl, mainFont, opTxt)
			}
		}
		if gs.pictIDDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%d", p.PictID)
			txtW, _ := text.Measure(lbl, mainFont, 0)
			xPos := x + half - int(math.Round(txtW))
			opTxt := &text.DrawOptions{}
			opTxt.GeoM.Translate(float64(xPos), float64(y)-float64(half)-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(color.RGBA{R: 1, A: 1})
			text.Draw(screen, lbl, mainFont, opTxt)
		}
		if gs.imgPlanesDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dp", plane)
			xPos := x - int(2*gs.GameScale)
			opTxt := &text.DrawOptions{}
			opTxt.GeoM.Translate(float64(xPos), float64(y)-2*gs.GameScale-metrics.HAscent)
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
	if gs.fastBars && cur < prev {
		return cur
	}
	return int(math.Round(float64(prev) + alpha*float64(cur-prev)))
}

func drawGameCurtain(screen *ebiten.Image, ox, oy int) {
	w := int(math.Round(float64(gameAreaSizeX) * gs.GameScale))
	h := int(math.Round(float64(gameAreaSizeY) * gs.GameScale))
	sw := screen.Bounds().Dx()
	sh := screen.Bounds().Dy()

	if blackPixel == nil {
		blackPixel = newImage(1, 1)
		blackPixel.Fill(color.Black)
	}

	op := &ebiten.DrawImageOptions{}

	if oy > 0 {
		op.GeoM.Scale(float64(sw), float64(oy))
		screen.DrawImage(blackPixel, op)
	}
	if bottom := sh - (oy + h); bottom > 0 {
		op.GeoM.Reset()
		op.GeoM.Scale(float64(sw), float64(bottom))
		op.GeoM.Translate(0, float64(oy+h))
		screen.DrawImage(blackPixel, op)
	}
	if ox > 0 {
		op.GeoM.Reset()
		op.GeoM.Scale(float64(ox), float64(h))
		op.GeoM.Translate(0, float64(oy))
		screen.DrawImage(blackPixel, op)
	}
	if right := sw - (ox + w); right > 0 {
		op.GeoM.Reset()
		op.GeoM.Scale(float64(right), float64(h))
		op.GeoM.Translate(float64(ox+w), float64(oy))
		screen.DrawImage(blackPixel, op)
	}
}

// drawStatusBars renders health, balance and spirit bars.
func drawStatusBars(screen *ebiten.Image, ox, oy int, snap drawSnapshot, alpha float64) {
	drawRect := func(x, y, w, h int, clr color.RGBA) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(w), float64(h))
		op.GeoM.Translate(float64(ox+x), float64(oy+y))
		op.ColorScale.ScaleWithColor(clr)
		screen.DrawImage(whiteImage, op)
	}
	barWidth := int(110 * gs.GameScale)
	barHeight := int(8 * gs.GameScale)
	fieldWidth := int(float64(gameAreaSizeX) * gs.GameScale)
	slot := (fieldWidth - 3*barWidth) / 6
	barY := int(float64(gameAreaSizeY)*gs.GameScale-20*gs.GameScale) - barHeight
	screenH := screen.Bounds().Dy()
	minY := -oy
	maxY := screenH - oy - barHeight
	if barY < minY {
		barY = minY
	} else if barY > maxY {
		barY = maxY
	}
	x := slot
	step := barWidth + 2*slot
	drawBar := func(x int, cur, max int, clr color.RGBA) {
		frameClr := color.RGBA{0xff, 0xff, 0xff, 0xff}
		vector.StrokeRect(screen, float32(float64(ox+x)-gs.GameScale), float32(float64(oy+barY)-gs.GameScale), float32(barWidth)+float32(2*gs.GameScale), float32(barHeight)+float32(2*gs.GameScale), 1, frameClr, false)
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

func drawServerFPS(screen *ebiten.Image, ox, oy int, fps float64) {
	if fps <= 0 {
		return
	}
	lat := netLatency
	msg := fmt.Sprintf("FPS: %0.2f UPS: %0.2f LAT: %dms", ebiten.ActualFPS(), fps, lat.Milliseconds())
	w, _ := text.Measure(msg, mainFont, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(ox)-w, float64(oy))
	text.Draw(screen, msg, mainFont, op)
}

// drawEquippedItems renders icons for all currently equipped items in the top left.
func drawEquippedItems(screen *ebiten.Image, ox, oy int) {
	items := getInventory()
	x := ox + int(4*gs.GameScale)
	y := oy + int(4*gs.GameScale)
	drawn := 0
	for _, it := range items {
		if !it.Equipped {
			continue
		}
		img := loadImage(it.ID)
		if img == nil {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(gs.GameScale, gs.GameScale)
		op.GeoM.Translate(float64(x), float64(y))
		screen.DrawImage(img, op)
		x += int(float64(img.Bounds().Dx())*gs.GameScale) + int(4*gs.GameScale)
		drawn++
	}
	if drawn == 0 {
		// No equipped items; previously drew default hands (pictid 6). Now draw nothing.
	}
}

// drawInputOverlay renders the text entry box when chatting.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	eui.Layout(outsideWidth, outsideHeight)
	return outsideWidth, outsideHeight
}

func runGame(ctx context.Context) {
	gameCtx = ctx

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	w, h := ebiten.Monitor().Size()
	if w == 0 || h == 0 {
		w, h = initialWindowW, initialWindowH
	}
	if gameWin != nil {
		gameWin.SetSize(eui.Point{X: float32(w), Y: float32(h)})
	}
	if gs.Fullscreen {
		ebiten.SetFullscreen(true)
	} else {
		//ebiten.MaximizeWindow()
	}

	op := &ebiten.RunGameOptions{ScreenTransparent: false}
	if err := ebiten.RunGameWithOptions(&Game{}, op); err != nil {
		log.Printf("ebiten: %v", err)
	}
	syncWindowSettings()
	saveSettings()
}

func initGame() {
	ebiten.SetWindowTitle("goThoom Client")
	ebiten.SetVsyncEnabled(gs.vsync)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebiten.SetCursorShape(ebiten.CursorShapeDefault)

	loadSettings()
	theme := gs.Theme
	if theme == "" {
		if darkMode, err := dark.IsDarkMode(); err == nil {
			if darkMode {
				theme = "AccentDark"
			} else {
				theme = "AccentLight"
			}
		} else {
			theme = "AccentDark"
		}
	}
	eui.LoadTheme(theme)
	eui.LoadStyle("RoundHybrid")
	initUI()
	updateCharacterButtons()

	close(gameStarted)
}

func makeGameWindow() {
	if gameWin != nil {
		return
	}
	gameWin = eui.NewWindow()
	gameWin.Margin = 0
	gameWin.Padding = 1 // one-pixel border for easier resizing
	gameWin.Border = 0
	gameWin.BorderPad = 0
	th := *gameWin.Theme
	th.Window.Theme = &th
	th.Window.BGColor = eui.Color{R: 0, G: 0, B: 0, A: 0}
	th.Window.ShadowColor = eui.Color{R: 0, G: 0, B: 0, A: 0}
	gameWin.Theme = &th
	gameWin.ShadowColor = eui.Color{R: 0, G: 0, B: 0, A: 0}
	gameWin.NoBGColor = true
	gameWin.Title = "Clan Lord"
	gameWin.Closable = false
	gameWin.Resizable = true
	gameWin.Movable = true
	gameWin.NoScale = true
	gameWin.AlwaysDrawFirst = true
	gameWin.Size = eui.Point{X: gameAreaSizeX, Y: gameAreaSizeY}
	gameWin.SetZone(eui.HZoneCenter, eui.VZoneTop)
	gameWin.MarkOpen()
	updateGameWindowSize()
}

func noteFrame() {
	if playingMovie {
		return
	}
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
				fps := (1000.0 / float64(modeMS))
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
		delay := interval
		if delay <= 0 {
			delay = 200 * time.Millisecond
		}
		if gs.lateInputUpdates {
			latencyMu.Lock()
			lat := netLatency
			latencyMu.Unlock()
			// Send the input early enough for the server to receive it
			// before the next update, adding a safety margin to the
			// measured latency.
			adjusted := (lat * lateRatio) / 100
			delay = interval - adjusted
			if delay < 0 {
				delay = 0
			}
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
		inputMu.Lock()
		s := latestInput
		inputMu.Unlock()
		if err := sendPlayerInput(conn, s.mouseX, s.mouseY, s.mouseDown); err != nil {
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
			handleDisconnect()
			return
		}
		tag := binary.BigEndian.Uint16(m[:2])
		flags := frameFlags(m)
		if recorder != nil {
			if !wroteLoginBlocks {
				if tag == 2 { // first draw state
					if len(loginGameState) > 0 {
						recorder.AddBlock(gameStateBlock(loginGameState), flagGameState)
					}
					if len(loginMobileData) > 0 {
						recorder.AddBlock(loginMobileData, flagMobileData)
					}
					if len(loginPictureTable) > 0 {
						recorder.AddBlock(loginPictureTable, flagPictureTable)
					}
					wroteLoginBlocks = true
					if err := recorder.WriteFrame(m, flags); err != nil {
						logError("record frame: %v", err)
					}
				} else {
					if flags&flagGameState != 0 {
						payload := append([]byte(nil), m[2:]...)
						parseGameState(payload, uint16(clientVersion), uint16(movieRevision))
						loginGameState = payload
					}
					if flags&flagMobileData != 0 {
						payload := append([]byte(nil), m[2:]...)
						parseMobileTable(payload, 0, uint16(clientVersion), uint16(movieRevision))
						loginMobileData = payload
					}
					if flags&flagPictureTable != 0 {
						payload := append([]byte(nil), m[2:]...)
						loginPictureTable = payload
					}
				}
			} else {
				if err := recorder.WriteFrame(m, flags); err != nil {
					logError("record frame: %v", err)
				}
			}
		}
		latencyMu.Lock()
		if !lastInputSent.IsZero() {
			rtt := time.Since(lastInputSent)
			if netLatency == 0 {
				netLatency = rtt
			} else {
				netLatency = (netLatency*7 + rtt) / 8
			}
			lastInputSent = time.Time{}
		}
		latencyMu.Unlock()
		if tag == 2 { // kMsgDrawState
			noteFrame()
			handleDrawState(m)
			continue
		}
		if txt := decodeMessage(m); txt != "" {
			consoleMessage("udpReadLoop: decodeMessage: " + txt)
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
			handleDisconnect()
			break
		}
		tag := binary.BigEndian.Uint16(m[:2])
		flags := frameFlags(m)
		if recorder != nil {
			if !wroteLoginBlocks {
				if tag == 2 { // first draw state
					if len(loginGameState) > 0 {
						recorder.AddBlock(gameStateBlock(loginGameState), flagGameState)
					}
					if len(loginMobileData) > 0 {
						recorder.AddBlock(loginMobileData, flagMobileData)
					}
					if len(loginPictureTable) > 0 {
						recorder.AddBlock(loginPictureTable, flagPictureTable)
					}
					wroteLoginBlocks = true
					if err := recorder.WriteFrame(m, flags); err != nil {
						logError("record frame: %v", err)
					}
				} else {
					if flags&flagGameState != 0 {
						payload := append([]byte(nil), m[2:]...)
						parseGameState(payload, uint16(clientVersion), uint16(movieRevision))
						loginGameState = payload
					}
					if flags&flagMobileData != 0 {
						payload := append([]byte(nil), m[2:]...)
						parseMobileTable(payload, 0, uint16(clientVersion), uint16(movieRevision))
						loginMobileData = payload
					}
					if flags&flagPictureTable != 0 {
						payload := append([]byte(nil), m[2:]...)
						loginPictureTable = payload
					}
				}
			} else {
				if err := recorder.WriteFrame(m, flags); err != nil {
					logError("record frame: %v", err)
				}
			}
		}
		if tag == 2 { // kMsgDrawState
			noteFrame()
			handleDrawState(m)
			continue
		}
		if txt := decodeMessage(m); txt != "" {
			//fmt.Println(txt)
			consoleMessage("tcpReadLoop: decodeMessage: " + txt)
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

func frameFlags(m []byte) uint16 {
	flags := uint16(0)
	if gPlayersListIsStale {
		flags |= flagStale
	}
	switch {
	case looksLikeGameState(m):
		flags |= flagGameState
	case looksLikeMobileData(m):
		flags |= flagMobileData
	case looksLikePictureTable(m):
		flags |= flagPictureTable
	}
	return flags
}

func looksLikeGameState(m []byte) bool {
	if i := bytes.IndexByte(m, 0); i >= 0 {
		rest := m[i+1:]
		return looksLikePictureTable(rest) || looksLikeMobileData(rest)
	}
	return false
}

func looksLikeMobileData(m []byte) bool {
	return bytes.Contains(m, []byte{0xff, 0xff, 0xff, 0xff})
}

func looksLikePictureTable(m []byte) bool {
	if len(m) < 2 {
		return false
	}
	count := int(binary.BigEndian.Uint16(m[:2]))
	size := 2 + 6*count + 4
	return count > 0 && size == len(m)
}
