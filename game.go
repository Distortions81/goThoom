package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
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
const defaultHandPictID = 6
const initialWindowW, initialWindowH = 1920, 720

var MHOX, MHOY int

var blackPixel *ebiten.Image

// worldRT is the offscreen render target for the game world when
// arbitrary window sizing is enabled. It stays at an integer-scaled
// multiple of the native field size and is composited into the window.
var worldRT *ebiten.Image

// gameImageItem is the UI image item inside the game window that displays
// the rendered world, and gameImage is its backing texture.
var gameImageItem *eui.ItemData
var gameImage *ebiten.Image
var inAspectResize bool

// gameWindowBG picks a background color for the game window content area.
// Prefers the game window's theme BGColor when opaque, otherwise falls
// back to any other window's BGColor, and finally to black.
func ensureWorldRT(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if worldRT == nil || worldRT.Bounds().Dx() != w || worldRT.Bounds().Dy() != h {
		// Use unmanaged images for faster off-screen rendering.
		worldRT = ebiten.NewImageWithOptions(image.Rect(0, 0, w, h), &ebiten.NewImageOptions{Unmanaged: true})
	}
}

// updateGameImageSize ensures the game image item exists and matches the
// current inner content size of the game window.
func updateGameImageSize() {
	if gameWin == nil {
		return
	}
	size := gameWin.GetSize()
	pad := float64(2 * gameWin.Padding)
	title := float64(gameWin.GetTitleSize())
	// Inner content size (exclude titlebar and inside padding)
	cw := int(float64(int(size.X)&^1) - pad)
	ch := int(float64(int(size.Y)&^1) - pad - title)
	// Leave a 2px margin on all sides for window edges
	w := cw - 4
	h := ch - 4
	if w <= 0 || h <= 0 {
		return
	}
	if gameImageItem == nil {
		it, img := eui.NewImageItem(w, h)
		gameImageItem = it
		gameImage = img
		gameImageItem.Position = eui.Point{X: 2, Y: 2}
		gameWin.AddItem(gameImageItem)
		return
	}
	// Resize backing image only when dimensions change
	iw, ih := 0, 0
	if gameImage != nil {
		b := gameImage.Bounds()
		iw, ih = b.Dx(), b.Dy()
	}
	if iw != w || ih != h {
		gameImage = ebiten.NewImage(w, h)
		gameImageItem.Image = gameImage
		gameImageItem.Size = eui.Point{X: float32(w), Y: float32(h)}
		gameImageItem.Position = eui.Point{X: 2, Y: 2}
		if gameWin != nil {
			gameWin.Dirty = true
		}
	}
}

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

var keyX, keyY int16
var walkToggled bool

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

// Deprecated: sound settings window removed; kept other windows.
var gameCtx context.Context
var frameCounter int
var gameStarted = make(chan struct{})

const framems = 200

var (
	frameCh       = make(chan struct{}, 1)
	lastFrameTime time.Time
	frameInterval = framems * time.Millisecond
	intervalHist  = map[int]int{}
	frameMu       sync.Mutex
	serverFPS     float64
	netLatency    time.Duration
	lastInputSent time.Time
	latencyMu     sync.Mutex
	// Throttled refresh for movie controller window
	lastMovieWinTick time.Time
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
	ackCmd                      uint8
	lightingFlags               uint8

	// Prepared render caches populated only when a new game state arrives.
	// These avoid per-frame sorting and partitioning work in Draw.
	picsNeg  []framePicture
	picsZero []framePicture
	picsPos  []framePicture
	liveMobs []frameMobile
	deadMobs []frameMobile
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

// prepareRenderCacheLocked populates render-ready, sorted/partitioned slices.
// Call with stateMu held and only when a new game state is applied.
func prepareRenderCacheLocked() {
	// Mobiles: split into live and dead, then sort by V then H.
	state.liveMobs = state.liveMobs[:0]
	state.deadMobs = state.deadMobs[:0]
	for _, m := range state.mobiles {
		if m.State == poseDead {
			state.deadMobs = append(state.deadMobs, m)
		}
		state.liveMobs = append(state.liveMobs, m)
	}
	sortMobiles(state.deadMobs)
	sortMobiles(state.liveMobs)

	// Pictures: sort once, then partition by plane while preserving order.
	// Work on a copy to avoid reordering the canonical state.pictures slice
	// which is also copied into snapshots.
	tmp := append([]framePicture(nil), state.pictures...)
	sortPictures(tmp)
	state.picsNeg = state.picsNeg[:0]
	state.picsZero = state.picsZero[:0]
	state.picsPos = state.picsPos[:0]
	for _, p := range tmp {
		switch {
		case p.Plane < 0:
			state.picsNeg = append(state.picsNeg, p)
		case p.Plane == 0:
			state.picsZero = append(state.picsZero, p)
		default:
			state.picsPos = append(state.picsPos, p)
		}
	}
}

// bubble stores temporary chat bubble information. Bubbles expire after a
// fixed number of game update frames from when they were created — no FPS
// correction or wall-clock timing is applied to keep playback simple.
const bubbleLifeFrames = (1000 / framems) * 4 // ~4s

type bubble struct {
	Index        uint8
	H, V         int16
	Far          bool
	NoArrow      bool
	Text         string
	Type         int
	CreatedFrame int
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
	ackCmd                      uint8
	lightingFlags               uint8

	// Precomputed, sorted/partitioned data for rendering
	picsNeg  []framePicture
	picsZero []framePicture
	picsPos  []framePicture
	liveMobs []frameMobile
	deadMobs []frameMobile
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
		ackCmd:         state.ackCmd,
		lightingFlags:  state.lightingFlags,
		// prepared caches
		picsNeg:  append([]framePicture(nil), state.picsNeg...),
		picsZero: append([]framePicture(nil), state.picsZero...),
		picsPos:  append([]framePicture(nil), state.picsPos...),
		liveMobs: append([]frameMobile(nil), state.liveMobs...),
		deadMobs: append([]frameMobile(nil), state.deadMobs...),
	}

	for idx, d := range state.descriptors {
		snap.descriptors[idx] = d
	}
	for _, m := range state.mobiles {
		snap.mobiles = append(snap.mobiles, m)
	}
	if len(state.bubbles) > 0 {
		curFrame := frameCounter
		kept := state.bubbles[:0]
		for _, b := range state.bubbles {
			if (curFrame - b.CreatedFrame) < bubbleLifeFrames {
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
	select {
	case <-gameCtx.Done():
		return errors.New("shutdown")
	default:
	}
	eui.Update() //We really need this to return eaten clicks

	once.Do(func() {
		initGame()
	})

	/* this should not be here */
	/* this should not be here */
	if inventoryDirty {
		updateInventoryWindow()
		updateHandsWindow()
		inventoryDirty = false
	}

	if playersDirty {
		updatePlayersWindow()
		playersDirty = false
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

	// Periodically persist players if there were changes.
	if time.Since(lastPlayersSave) >= 5*time.Second {
		if playersDirty || playersPersistDirty {
			savePlayersPersist()
			playersPersistDirty = false
		}
		lastPlayersSave = time.Now()
	}

	// Ensure the movie controller window repaints at least once per second
	// while open, even without other UI events.
	if movieWin != nil && movieWin.IsOpen() {
		if time.Since(lastMovieWinTick) >= time.Second {
			lastMovieWinTick = time.Now()
			movieWin.Refresh()
		}
	}
	/* this should not be here */
	/* this should not be here */

	/* Console input */
	changedInput := false
	if inputActive {
		if newChars := ebiten.AppendInputChars(nil); len(newChars) > 0 {
			inputText = append(inputText, newChars...)
			changedInput = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			if len(inputHistory) > 0 {
				if historyPos > 0 {
					historyPos--
				} else {
					historyPos = 0
				}
				inputText = []rune(inputHistory[historyPos])
				changedInput = true
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			if len(inputHistory) > 0 {
				if historyPos < len(inputHistory)-1 {
					historyPos++
					inputText = []rune(inputHistory[historyPos])
					changedInput = true
				} else {
					historyPos = len(inputHistory)
					inputText = inputText[:0]
					changedInput = true
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
			if len(inputText) > 0 {
				inputText = inputText[:len(inputText)-1]
				changedInput = true
			}
		} else if d := inpututil.KeyPressDuration(ebiten.KeyBackspace); d > 30 && d%3 == 0 {
			if len(inputText) > 0 {
				inputText = inputText[:len(inputText)-1]
				changedInput = true
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
			changedInput = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			inputActive = false
			inputText = inputText[:0]
			historyPos = len(inputHistory)
			changedInput = true
		}
	} else {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			inputActive = true
			inputText = inputText[:0]
			historyPos = len(inputHistory)
			changedInput = true
		}
	}

	if changedInput {
		updateConsoleWindow()
		if consoleWin != nil {
			consoleWin.Refresh()
		}
	}

	/* WASD / ARROWS */

	var keyWalk bool
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
	}

	mx, my := ebiten.CursorPosition()
	gx, gy := gameWindowOrigin()
	baseX := int16(float64(mx-gx)/gs.GameScale - float64(fieldCenterX))
	baseY := int16(float64(my-gy)/gs.GameScale - float64(fieldCenterY))
	heldTime := inpututil.MouseButtonPressDuration(ebiten.MouseButtonLeft)
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

	if click && heldTime <= 1 {
		MHOX, MHOY = mx, my
	}

	/*
	 * Detect used in UI
	 * TODO CLEANUP
	 */
	walk := false
	if pointInUI(mx, my) {
		if !click && heldTime > 1 {
			if pointInUI(MHOX, MHOY) {
				click = false
				heldTime = 0
			}
		}
		click = false
	}

	x, y := baseX, baseY
	if keyWalk {
		x, y, walk = keyX, keyY, true
		walkToggled = false
	} else if gs.ClickToToggle && click {
		walkToggled = !walkToggled
		walk = walkToggled
	} else if !gs.ClickToToggle && heldTime > 1 && !click {
		walk = true
		walkToggled = false
	}

	if gs.ClickToToggle && walkToggled {
		walk = walkToggled
	}

	/* Change Cursor */
	if walk && !keyWalk {
		ebiten.SetCursorShape(ebiten.CursorShapeCrosshair)
	} else {
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
	}

	inputMu.Lock()
	latestInput = inputState{mouseX: x, mouseY: y, mouseDown: walk}
	inputMu.Unlock()

	return nil
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
	// Ensure the game image item/buffer exists and matches window content.
	updateGameImageSize()
	if gameImage == nil {
		// UI not ready yet
		eui.Draw(screen)
		return
	}

	// Determine offscreen integer render scale and composite scale.
	// A user-selected render scale (gs.GameScale) in 1x..10x controls
	// the apparent size of the world. In integer mode we render exactly
	// at that integer and composite with nearest-neighbor.
	bufW := gameImage.Bounds().Dx()
	bufH := gameImage.Bounds().Dy()
	const maxSuperSampleScale = 4
	worldW, worldH := gameAreaSizeX, gameAreaSizeY

	// Clamp desired render scale from settings (treat as integer steps)
	desired := int(math.Round(gs.GameScale))
	if desired < 1 {
		desired = 1
	}
	if desired > 10 {
		desired = 10
	}
	// Maximum scale that fits the current buffer without clipping
	fit := int(math.Floor(math.Min(float64(bufW)/float64(worldW), float64(bufH)/float64(worldH))))
	if fit < 1 {
		fit = 1
	}

	var offIntScale int
	var target int // final intended on-screen integer scale
	var finalFilter ebiten.Filter
	if gs.IntegerScaling {
		// Render and composite at the same exact integer scale.
		target = desired
		if target > fit {
			target = fit
		}
		offIntScale = target
		finalFilter = ebiten.FilterNearest
	} else if gs.AnyGameWindowSize {
		// Any-size mode: always fill the window with linear filtering.
		// Use the slider value as a supersample factor (up to a safe cap),
		// but prefer at least the integer fit so we don't upscale the RT.
		target = fit // final on-screen scale is whatever fits the window
		offIntScale = int(math.Ceil(float64(fit)))
		if desired > offIntScale {
			offIntScale = desired
		}
		if offIntScale > maxSuperSampleScale {
			offIntScale = maxSuperSampleScale
		}
		if offIntScale < 1 {
			offIntScale = 1
		}
		finalFilter = ebiten.FilterLinear
	} else {
		// Classic fixed-size mode: render to the target integer scale.
		target = desired
		offIntScale = target
		if offIntScale < 1 {
			offIntScale = 1
		}
		finalFilter = ebiten.FilterNearest
	}

	// Prepare variable-sized offscreen target (supersampled in any-size)
	offW := worldW * offIntScale
	offH := worldH * offIntScale
	ensureWorldRT(offW, offH)
	worldRT.Clear()

	// Render splash or live frame into worldRT using offscreen integer scale
	if clmov == "" && tcpConn == nil && pcapPath == "" {
		prev := gs.GameScale
		gs.GameScale = float64(offIntScale)
		drawSplash(worldRT, 0, 0)
		gs.GameScale = prev
	} else {
		snap := captureDrawSnapshot()
		alpha, mobileFade, pictFade := computeInterpolation(snap.prevTime, snap.curTime, gs.MobileBlendAmount, gs.BlendAmount)
		prev := gs.GameScale
		gs.GameScale = float64(offIntScale)
		drawScene(worldRT, 0, 0, snap, alpha, mobileFade, pictFade)
		if gs.nightEffect {
			drawNightOverlay(worldRT, 0, 0)
		}
		drawStatusBars(worldRT, 0, 0, snap, alpha)
		gs.GameScale = prev
	}

	// Composite worldRT into the gameImage buffer: scale/center
	gameImage.Clear()
	// In integer mode we render at target and do not rescale.
	// Any-size mode fills the window; otherwise scale to the target.
	scaleDown := 1.0
	if !gs.IntegerScaling {
		if gs.AnyGameWindowSize {
			scaleDown = math.Min(float64(bufW)/float64(offW), float64(bufH)/float64(offH))
		} else {
			scaleDown = float64(target) / float64(offIntScale)
		}
	}
	drawW := float64(offW) * scaleDown
	drawH := float64(offH) * scaleDown
	tx := (float64(bufW) - drawW) / 2
	ty := (float64(bufH) - drawH) / 2
	op := &ebiten.DrawImageOptions{Filter: finalFilter, DisableMipmaps: true}
	op.GeoM.Scale(scaleDown, scaleDown)
	op.GeoM.Translate(tx, ty)
	gameImage.DrawImage(worldRT, op)

	// Finally, draw UI (which includes the game window image)
	eui.Draw(screen)
	if gs.ShowFPS {
		drawServerFPS(screen, screen.Bounds().Dx()-40, 4, serverFPS)
	}
}

// drawScene renders all world objects for the current frame.
func drawScene(screen *ebiten.Image, ox, oy int, snap drawSnapshot, alpha float64, mobileFade, pictFade float32) {
	// Use cached descriptor map directly; no need to rebuild/sort it per frame.
	descMap := snap.descriptors

	// Use precomputed, sorted partitions
	negPics := snap.picsNeg
	zeroPics := snap.picsZero
	posPics := snap.picsPos
	live := snap.liveMobs
	dead := snap.deadMobs

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
			x := roundToInt((hpos + float64(fieldCenterX)) * gs.GameScale)
			y := roundToInt((vpos + float64(fieldCenterY)) * gs.GameScale)
			if !b.Far {
				if d, ok := descMap[b.Index]; ok {
					if size := mobileSize(d.PictID); size > 0 {
						// Equivalent to: tailHeight - half(sprite) - half(sprite image height)
						// Replace image-based height with mobileSize to avoid texture fetch.
						tailHeight := int(10 * gs.GameScale)
						y += tailHeight - int(math.Round(float64(size)*gs.GameScale))
					}
				}
			}
			x += ox
			y += oy
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
	x := roundToInt((h + float64(fieldCenterX)) * gs.GameScale)
	y := roundToInt((v + float64(fieldCenterY)) * gs.GameScale)
	x += ox
	y += oy
	// view bounds culling is handled during state parse; no per-frame check here
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
		plane = d.Plane
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
		scaled := float64(roundToInt(float64(drawSize) * scale))
		scale = scaled / float64(drawSize)
		// No per-frame bounds check (culled earlier).
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		op.GeoM.Scale(scale, scale)
		tx := float64(x) - scaled/2
		ty := float64(y) - scaled/2
		op.GeoM.Translate(tx, ty)
		screen.DrawImage(src, op)
		if d, ok := descMap[m.Index]; ok {
			alpha := uint8(gs.NameBgOpacity * 255)
			if d.Name != "" {
				// Prefer cached name tag if parameters match current settings.
				if m.nameTag != nil && m.nameTagKey.FontGen == fontGen && m.nameTagKey.Opacity == alpha && m.nameTagKey.Text == d.Name && m.nameTagKey.Colors == m.Colors {
					top := y + int(20*gs.GameScale)
					left := x - m.nameTagW/2
					op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
					op.GeoM.Translate(float64(left), float64(top))
					screen.DrawImage(m.nameTag, op)
				} else {
					textClr, bgClr, frameClr := mobileNameColors(m.Colors)
					bgClr.A = alpha
					frameClr.A = alpha
					w, h := text.Measure(d.Name, mainFont, 0)
					iw := int(math.Ceil(w))
					ih := int(math.Ceil(h))
					top := y + int(20*gs.GameScale)
					left := x - iw/2
					op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
					op.GeoM.Scale(float64(iw+5), float64(ih))
					op.GeoM.Translate(float64(left), float64(top))
					op.ColorScale.ScaleWithColor(bgClr)
					screen.DrawImage(whiteImage, op)
					vector.StrokeRect(screen, float32(left), float32(top), float32(iw+5), float32(ih), 1, frameClr, false)
					opTxt := &text.DrawOptions{}
					opTxt.GeoM.Translate(float64(left+2), float64(top+2))
					opTxt.ColorScale.ScaleWithColor(textClr)
					text.Draw(screen, d.Name, mainFont, opTxt)
				}
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
					op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
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
		// Fallback marker when image missing; no per-frame bounds check.
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

	x := roundToInt(((float64(p.H) + offX + mobileX) + float64(fieldCenterX)) * gs.GameScale)
	y := roundToInt(((float64(p.V) + offY + mobileY) + float64(fieldCenterY)) * gs.GameScale)
	x += ox
	y += oy

	// No per-frame bounds check (culled earlier).

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
		scaledW := float64(roundToInt(float64(drawW) * sx))
		scaledH := float64(roundToInt(float64(drawH) * sy))
		sx = scaledW / float64(drawW)
		sy = scaledH / float64(drawH)
		// No per-frame bounds check (culled earlier).
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		op.GeoM.Scale(sx, sy)
		tx := float64(x) - scaledW/2
		ty := float64(y) - scaledH/2
		op.GeoM.Translate(tx, ty)
		if gs.pictAgainDebug && p.Again {
			op.ColorScale.Scale(0, 0, 1, 1)
		} else if src == img && gs.smoothingDebug && p.Moving {
			op.ColorScale.Scale(1, 0, 0, 1)
		}
		screen.DrawImage(src, op)

		if gs.pictIDDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%d", p.PictID)
			txtW, _ := text.Measure(lbl, mainFont, 0)
			xPos := x + int(float64(w)*gs.GameScale/2) - roundToInt(txtW)
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
		// No per-frame bounds check (culled earlier).
		clr := color.RGBA{0, 0, 0xff, 0xff}
		if gs.smoothingDebug && p.Moving {
			clr = color.RGBA{0xff, 0, 0, 0xff}
		}
		if gs.pictAgainDebug && p.Again {
			clr = color.RGBA{0, 0, 0xff, 0xff}
		}
		vector.DrawFilledRect(screen, float32(float64(x)-2*gs.GameScale), float32(float64(y)-2*gs.GameScale), float32(4*gs.GameScale), float32(4*gs.GameScale), clr, false)
		if gs.pictIDDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%d", p.PictID)
			txtW, _ := text.Measure(lbl, mainFont, 0)
			half := int(2 * gs.GameScale)
			xPos := x + half - roundToInt(txtW)
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

	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}

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
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
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

// equippedItemPicts returns pict IDs for items equipped in right and left hands.
func equippedItemPicts() (uint16, uint16) {
	items := getInventory()
	var rightID, leftID uint16
	var bothIDRight, bothIDLeft uint16
	if clImages != nil {
		for _, it := range items {
			if !it.Equipped {
				continue
			}
			slot := clImages.ItemSlot(uint32(it.ID))
			switch slot {
			case kItemSlotRightHand:
				if id := clImages.ItemRightHandPict(uint32(it.ID)); id != 0 {
					rightID = uint16(id)
				} else if id := clImages.ItemWornPict(uint32(it.ID)); id != 0 {
					rightID = uint16(id)
				}
			case kItemSlotLeftHand:
				if id := clImages.ItemLeftHandPict(uint32(it.ID)); id != 0 {
					leftID = uint16(id)
				} else if id := clImages.ItemWornPict(uint32(it.ID)); id != 0 {
					leftID = uint16(id)
				}
			case kItemSlotBothHands:
				if id := clImages.ItemRightHandPict(uint32(it.ID)); id != 0 {
					bothIDRight = uint16(id)
				} else if id := clImages.ItemWornPict(uint32(it.ID)); id != 0 {
					bothIDRight = uint16(id)
				}
				if id := clImages.ItemLeftHandPict(uint32(it.ID)); id != 0 {
					bothIDLeft = uint16(id)
				} else if id := clImages.ItemWornPict(uint32(it.ID)); id != 0 {
					bothIDLeft = uint16(id)
				}
			}
		}
	}
	if rightID == 0 && leftID == 0 {
		if bothIDRight != 0 || bothIDLeft != 0 {
			if rightID == 0 {
				rightID = bothIDRight
				if rightID == 0 {
					rightID = bothIDLeft
				}
			}
			if leftID == 0 {
				leftID = bothIDLeft
				if leftID == 0 {
					leftID = bothIDRight
				}
			}
		}
	}
	return rightID, leftID
}

// drawEquippedItems renders icons for all currently equipped items in the top left.
func drawEquippedItems(screen *ebiten.Image, ox, oy int) {
	rightID, leftID := equippedItemPicts()
	x := ox + int(4*gs.GameScale)
	y := oy + int(4*gs.GameScale)
	if rightID == 0 && leftID == 0 {
		img := loadImage(defaultHandPictID)
		if img == nil {
			return
		}
		w := int(float64(img.Bounds().Dx()) * gs.GameScale)
		opRight := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		opRight.GeoM.Scale(gs.GameScale, gs.GameScale)
		opRight.GeoM.Translate(float64(x), float64(y))
		screen.DrawImage(img, opRight)

		opLeft := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
		opLeft.GeoM.Scale(-gs.GameScale, gs.GameScale)
		opLeft.GeoM.Translate(float64(w), 0)
		opLeft.GeoM.Translate(float64(x+w)+4*gs.GameScale, float64(y))
		screen.DrawImage(img, opLeft)
		return
	}

	if rightID != 0 {
		if img := loadImage(rightID); img != nil {
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
			op.GeoM.Scale(gs.GameScale, gs.GameScale)
			op.GeoM.Translate(float64(x), float64(y))
			screen.DrawImage(img, op)
			x += int(float64(img.Bounds().Dx())*gs.GameScale) + int(4*gs.GameScale)
		}
	}
	if leftID != 0 {
		if img := loadImage(leftID); img != nil {
			w := int(float64(img.Bounds().Dx()) * gs.GameScale)
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
			op.GeoM.Scale(-gs.GameScale, gs.GameScale)
			op.GeoM.Translate(float64(w), 0)
			op.GeoM.Translate(float64(x), float64(y))
			screen.DrawImage(img, op)
		}
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
		ebiten.SetWindowFloating(true)
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

	resetInventory()

	loadSettings()
	theme := gs.Theme
	if theme == "" {
		darkMode, err := dark.IsDarkMode()
		if err == nil {
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
	gameWin.Title = "Clan Lord"
	gameWin.Closable = false
	gameWin.Resizable = true
	gameWin.NoBGColor = true
	gameWin.Movable = true
	gameWin.NoScroll = true
	gameWin.NoCache = true
	gameWin.NoScale = true
	gameWin.AlwaysDrawFirst = true
	gameWin.SetZone(eui.HZoneCenter, eui.VZoneTop)
	gameWin.Size = eui.Point{X: 8000, Y: 8000}
	gameWin.MarkOpen()
	gameWin.OnResize = func() { onGameWindowResize() }
	// Titlebar maximize button controlled by settings (now default on)
	gameWin.Maximizable = true
	// Keep same horizontal center on maximize
	gameWin.OnMaximize = func() {
		if gameWin == nil {
			return
		}
		// Record current center X before size change
		pos := gameWin.GetPos()
		sz := gameWin.GetSize()
		centerX := float64(pos.X) + float64(sz.X)/2
		// Maximize to screen bounds first
		w, h := eui.ScreenSize()
		gameWin.ClearZone()
		_ = gameWin.SetPos(eui.Point{X: 0, Y: 0})
		_ = gameWin.SetSize(eui.Point{X: float32(w), Y: float32(h)})
		// Aspect ratio handler will adjust size via OnResize; recalc size
		sz2 := gameWin.GetSize()
		newW := float64(sz2.X)
		// Recenter horizontally to keep same center
		newX := centerX - newW/2
		if newX < 0 {
			newX = 0
		}
		maxX := float64(w) - newW
		if newX > maxX {
			newX = maxX
		}
		_ = gameWin.SetPos(eui.Point{X: float32(newX), Y: 0})
		updateGameImageSize()
	}
	updateGameWindowSize()
	updateGameImageSize()
}

// maximizeGameWindow resizes the game window to fill the Ebiten screen area.
func maximizeGameWindow() {
	if gameWin == nil {
		return
	}
	w, h := eui.ScreenSize()
	gameWin.ClearZone()
	_ = gameWin.SetPos(eui.Point{X: 0, Y: 0})
	_ = gameWin.SetSize(eui.Point{X: float32(w), Y: float32(h)})
	updateGameImageSize()
}

// onGameWindowResize enforces the game's aspect ratio on the window's
// content area (excluding titlebar and padding) and updates the image size.
func onGameWindowResize() {
	if gameWin == nil {
		return
	}
	if inAspectResize {
		updateGameImageSize()
		return
	}

	size := gameWin.GetSize()
	if size.X <= 0 || size.Y <= 0 {
		return
	}

	// Available inner content area (exclude titlebar and padding)
	pad := float64(2 * gameWin.Padding)
	title := float64(gameWin.GetTitleSize())
	availW := float64(int(size.X)&^1) - pad
	availH := float64(int(size.Y)&^1) - pad - title
	if availW <= 0 || availH <= 0 {
		updateGameImageSize()
		return
	}

	// Fit the content to the largest rectangle with the game's aspect ratio.
	targetW := float64(gameAreaSizeX)
	targetH := float64(gameAreaSizeY)
	scale := math.Min(availW/targetW, availH/targetH)
	if scale < 0.25 {
		scale = 0.25
	}
	fitW := targetW * scale
	fitH := targetH * scale
	newW := float32(math.Round(fitW + pad))
	newH := float32(math.Round(fitH + pad + title))

	if math.Abs(float64(size.X)-float64(newW)) > 0.5 || math.Abs(float64(size.Y)-float64(newH)) > 0.5 {
		inAspectResize = true
		_ = gameWin.SetSize(eui.Point{X: newW, Y: newH})
		inAspectResize = false
	}
	updateGameImageSize()
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
		processServerMessage(m)
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
		processServerMessage(m)
		// Allow maintenance queues to issue commands even when the
		// player isn't moving; this keeps /be-info and /be-who flowing
		// during idle periods on live connections.
		if pendingCommand == "" {
			if !maybeEnqueueInfo() {
				_ = maybeEnqueueWho()
			}
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

// roundToInt returns the nearest integer to f. It avoids calling math.Round
// and handles negative values correctly.
func roundToInt(f float64) int {
	if f >= 0 {
		return int(f + 0.5)
	}
	return int(f - 0.5)
}
