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
	"math/rand"
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
	clipboard "golang.design/x/clipboard"
)

const keyRepeatRate = 32
const gameAreaSizeX, gameAreaSizeY = 547, 540
const fieldCenterX, fieldCenterY = gameAreaSizeX / 2, gameAreaSizeY / 2
const defaultHandPictID = 6
const initialWindowW, initialWindowH = 1920, 1080

var uiMouseDown bool

// worldRT is the offscreen render target for the game world. It stays at an
// integer multiple of the native field size and is composited into the window.
var worldRT *ebiten.Image

// worldRTUsedRect tracks the active portion of worldRT used for the latest
// frame, and worldViewRect tracks the region of gameImage that displays the
// composited world.
var (
	worldRTUsedRect image.Rectangle
	worldViewRect   image.Rectangle
)

// gameImageItem is the UI image item inside the game window that displays
// the rendered world. gameImage is the current visible view, while
// gameImageBacking may be larger so interactive resize does not churn texture
// allocations.
var gameImageItem *eui.ItemData
var gameImage *ebiten.Image
var gameImageBacking *ebiten.Image
var inAspectResize bool

// dimmedScreenBG holds the theme window background color dimmed by 25%.
// updateDimmedScreenBG refreshes this color when the theme changes.
var dimmedScreenBG = color.RGBA{0, 0, 0, 255}

// No pooling for draw options; use locals to favor stack allocation.

func updateDimmedScreenBG() {
	c := color.RGBA{0, 0, 0, 255}
	if gameWin != nil && gameWin.Theme != nil {
		if tc := color.RGBA(gameWin.Theme.Window.BGColor); tc.A > 0 {
			c = tc
		}
	}
	dimmedScreenBG = color.RGBA{
		R: uint8(uint16(c.R) / 2),
		G: uint8(uint16(c.G) / 2),
		B: uint8(uint16(c.B) / 2),
		A: 255,
	}
}

func ensureWorldRT(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	// Grow-only allocation to avoid churn during interactive resize.
	// We will draw using a subimage matching the requested w,h.
	if worldRT == nil || worldRT.Bounds().Dx() < w || worldRT.Bounds().Dy() < h {
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
	s := eui.UIScale()
	if gameImageItem == nil {
		it, img := eui.NewImageFastItem(w, h)
		gameImageItem = it
		gameImageBacking = img
		gameImage = img
		gameImageItem.Image = gameImage
		gameImageItem.Size = eui.Point{X: float32(w) / s, Y: float32(h) / s}
		gameImageItem.Position = eui.Point{X: 2 / s, Y: 2 / s}
		gameWin.AddItem(gameImageItem)
		return
	}
	// Grow the backing image only when needed, but expose a current-size
	// subimage so eui draws the game view 1:1 instead of scaling the larger
	// backing texture during window shrink.
	iw, ih := 0, 0
	if gameImageBacking != nil {
		b := gameImageBacking.Bounds()
		iw, ih = b.Dx(), b.Dy()
	}
	if gameImageBacking == nil || iw < w || ih < h {
		_, gameImageBacking = eui.NewImageFastItem(w, h)
		if gameWin != nil {
			gameWin.Dirty = true
		}
	}
	gameImage = gameImageBacking.SubImage(image.Rect(0, 0, w, h)).(*ebiten.Image)
	gameImageItem.Image = gameImage
	// Always update the item size/position even if we reuse a larger backing image.
	gameImageItem.Size = eui.Point{X: float32(w) / s, Y: float32(h) / s}
	gameImageItem.Position = eui.Point{X: 2 / s, Y: 2 / s}
}

// In-world rendering uses integer scaling (nearest) only.

// acquireDrawOpts returns a DrawImageOptions from the shared pool initialized
// with nearest filtering and mipmaps disabled. Call releaseDrawOpts when done.
func acquireDrawOpts() *ebiten.DrawImageOptions {
	op := &ebiten.DrawImageOptions{}
	op.Filter = ebiten.FilterNearest
	op.DisableMipmaps = true
	return op
}

// releaseDrawOpts is a no-op when not pooling.
func releaseDrawOpts(op *ebiten.DrawImageOptions) {}

func acquireTextDrawOpts() *text.DrawOptions {
	op := &text.DrawOptions{}
	op.DrawImageOptions.Filter = ebiten.FilterNearest
	op.DrawImageOptions.DisableMipmaps = true
	return op
}

// releaseTextDrawOpts is a no-op when not pooling.
func releaseTextDrawOpts(op *text.DrawOptions) {}

type inputState struct {
	mouseX, mouseY int16
	mouseDown      bool
}

var (
	latestInput inputState
	inputQueue  []inputState
	inputMu     sync.Mutex
)

var keyX, keyY int16
var walkToggled bool
var keyWalkPrev bool
var keyStopFrames int
var joyCursorX, joyCursorY float64

var inputActive bool
var inputText []rune
var inputPos int
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
var bubbleWin *eui.WindowData
var notificationsWin *eui.WindowData

var (
	lastDebugStatsUpdate   time.Time
	lastQualityPresetCheck time.Time
	lastMovieWinRefresh    time.Time
)

// Deprecated: sound settings window removed; kept other windows.
var gameCtx context.Context
var frameCounter int
var gameStarted = make(chan struct{})

const framems = 200

var (
	frameCh         = make(chan struct{}, 1)
	lastFrameTime   time.Time
	frameInterval   = framems * time.Millisecond
	intervalHist    = map[int]int{}
	frameMu         sync.Mutex
	netLatency      time.Duration
	netJitter       time.Duration
	lastInputSent   time.Time
	latencyMu       sync.Mutex
	lowFPSSince     time.Time
	shaderWarnShown bool
)

var (
	worldOriginX int
	worldOriginY int
	worldScale   float64 = 1.0
)

// drawState tracks information needed by the Ebiten renderer.
type drawState struct {
	descriptors  map[uint8]frameDescriptor
	pictures     []framePicture
	prevPictures []framePicture
	picShiftX    int
	picShiftY    int
	mobiles      map[uint8]frameMobile
	prevMobiles  map[uint8]frameMobile
	prevDescs    map[uint8]frameDescriptor
	prevTime     time.Time
	curTime      time.Time

	bubbles []bubble

	hp, hpMax                   int
	sp, spMax                   int
	balance, balanceMax         int
	prevHP, prevHPMax           int
	prevSP, prevSPMax           int
	prevBalance, prevBalanceMax int
	ackCmd                      uint8
	lightingFlags               uint8
	dropped                     int

	// Prepared render caches populated only when a new game state arrives.
	// These avoid per-frame sorting and partitioning work in Draw.
	picsNeg  []framePicture
	picsZero []framePicture
	picsPos  []framePicture
	liveMobs []frameMobile
	deadMobs []frameMobile
	nameMobs []frameMobile
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

// resetDrawState clears all game state and interpolation data.
// It also resets timing counters so new sessions start from a clean slate.
func resetDrawState() {
	stateMu.Lock()
	state = drawState{
		descriptors: make(map[uint8]frameDescriptor),
		mobiles:     make(map[uint8]frameMobile),
		prevMobiles: make(map[uint8]frameMobile),
		prevDescs:   make(map[uint8]frameDescriptor),
	}
	stateMu.Unlock()

	resetInterpolation()

	frameCounter = 0
	frameInterval = framems * time.Millisecond

	// Clear frame timing history so new sessions start fresh without
	// inherited intervals from previous connections.
	frameMu.Lock()
	lastFrameTime = time.Time{}
	intervalHist = map[int]int{}
	frameMu.Unlock()

	stateMu.Lock()
	initialState = cloneDrawState(state)
	stateMu.Unlock()
}

// prepareRenderCacheLocked populates render-ready, sorted/partitioned slices.
// Call with stateMu held and only when a new game state is applied.
func prepareRenderCacheLocked() {
	// Mobiles: split into live and dead, sort by V then H, and prepare
	// a separate slice sorted right-to-left/top-to-bottom for name tags.
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

	state.nameMobs = append(state.nameMobs[:0], state.liveMobs...)
	sortMobilesNameTags(state.nameMobs)

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
// number of frames determined when they are created. No FPS correction or
// wall-clock timing is applied to keep playback simple.
type bubble struct {
	Index        uint8
	H, V         int16
	Far          bool
	NoArrow      bool
	Text         string
	Type         int
	CreatedFrame int
	LifeFrames   int
}

// drawSnapshot is a read-only copy of the current draw state.
type drawSnapshot struct {
	descriptors                 map[uint8]frameDescriptor
	pictures                    []framePicture
	prevPictures                []framePicture
	picShiftX                   int
	picShiftY                   int
	mobiles                     []frameMobile // sorted right-to-left, top-to-bottom
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
	dropped                     int

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
		prevPictures:   append([]framePicture(nil), state.prevPictures...),
		picShiftX:      state.picShiftX,
		picShiftY:      state.picShiftY,
		mobiles:        append([]frameMobile(nil), state.nameMobs...),
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
		dropped:        state.dropped,
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
	if len(state.bubbles) > 0 {
		curFrame := frameCounter
		kept := state.bubbles[:0]
		for _, b := range state.bubbles {
			if (curFrame - b.CreatedFrame) < b.LifeFrames {
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
		dropped:        src.dropped,
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
func computeInterpolation(now, prevTime, curTime time.Time, mobileRate, pictRate float64) (alpha float64, mobileFade, pictFade float32) {
	if suppressInterpOnce {
		// Skip interpolation for a single frame (e.g., after start/seek).
		suppressInterpOnce = false
		return 1.0, 1.0, 1.0
	}
	alpha = 1.0
	mobileFade = 1.0
	pictFade = 1.0
	if (gs.MotionSmoothing || gs.BlendMobiles || gs.BlendPicts) && !curTime.IsZero() && curTime.After(prevTime) {
		// Use cached frame time to avoid repeated runtime.Now calls
		elapsed := now.Sub(prevTime)
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
var lastBackpace time.Time
var lastPlayersRefreshTick time.Time
var lastFocused bool

// suppressInterpOnce skips interpolation for the next draw frame.
var suppressInterpOnce bool

func (g *Game) Update() error {
	// Background behaviors: mute and slow render when unfocused
	focused := ebiten.IsFocused()
	if focused != lastFocused {
		if !focused {
			if gs.MuteWhenUnfocused {
				focusMuted = true
			}
		} else {
			focusMuted = false
		}
		// Immediately propagate effective master volume change to active players.
		updateSoundVolume()
		lastFocused = focused
	}
	// Cache the current time once per frame and reuse everywhere.
	now := time.Now()
	select {
	case <-gameCtx.Done():
		syncWindowSettings()
		return errors.New("shutdown")
	default:
	}
	once.Do(func() {
		initGame()
	})
	processMusicRequests()

	if classicSplashFilterPending && gs.ShowClanLordSplashImage {
		prepareClassicSplash()
	}

	if inputFlow != nil && len(inputFlow.Contents) > 0 {
		eui.ClearFocus(inputFlow.Contents[0])
		inputFlow.Contents[0].Focused = false
	}
	eui.Update() //We really need this to return eaten clicks
	// Advance script tick waiters once per frame
	scriptAdvanceTick()
	typingElsewhere := typingInUI()
	if inputActive && inputFlow != nil && len(inputFlow.Contents) > 0 {
		item := inputFlow.Contents[0]
		inputPos = plainCursorPos(item.Text, item.CursorPos)
		plain := strings.ReplaceAll(item.Text, "\n", "")
		inputText = []rune(plain)
	}
	checkForScriptEdit()
	updateNotifications()
	updateThinkMessages()
	// Throttle player maintenance to reduce idle CPU (every ~250ms)
	if now.Sub(lastPlayersRefreshTick) >= 250*time.Millisecond {
		requestPlayersData()
		lastPlayersRefreshTick = now
	}

	mx, my := eui.PointerPosition()
	hx := int16(float64(mx-worldOriginX)/worldScale - float64(fieldCenterX))
	hy := int16(float64(my-worldOriginY)/worldScale - float64(fieldCenterY))
	updateWorldHover(hx, hy)

	joyClick1, joyClick2, joyClick3 := false, false, false
	if gs.JoystickEnabled && selectedJoystick >= 0 && selectedJoystick < len(joystickIDs) {
		id := joystickIDs[selectedJoystick]
		if b, ok := gs.JoystickBindings["click1"]; ok {
			joyClick1 = inpututil.IsGamepadButtonJustPressed(id, b)
		}
		if b, ok := gs.JoystickBindings["click2"]; ok {
			joyClick2 = inpututil.IsGamepadButtonJustPressed(id, b)
		}
		if b, ok := gs.JoystickBindings["click3"]; ok {
			joyClick3 = inpututil.IsGamepadButtonJustPressed(id, b)
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) || joyClick2 {
		// Input bar menu takes precedence when right-clicking on input.
		if !handleConsoleInputContext(mx, my) {
			// Try players list first, then inventory, then chat/console copy.
			if !handlePlayersContextClick(mx, my) {
				if !handleInventoryContextClick(mx, my) {
					if !handleChatCopyRightClick(mx, my) {
						_ = handleConsoleCopyRightClick(mx, my)
					}
				}
			}
		}
	}

	if debugWin != nil && debugWin.IsOpen() {
		if now.Sub(lastDebugStatsUpdate) >= time.Second {
			updateDebugStats()
			lastDebugStatsUpdate = now
		}
	}

	if joystickWin != nil && joystickWin.IsOpen() {
		updateJoystickWindow()
	}

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

	if now.Sub(lastQualityPresetCheck) >= time.Second {
		if settingsDirty && qualityPresetDD != nil {
			qualityPresetDD.Selected = detectQualityPreset()
		}
		lastQualityPresetCheck = now
	}

	if now.Sub(lastSettingsSave) >= time.Second {
		if settingsDirty {
			saveSettings()
			settingsDirty = false
		}
		lastSettingsSave = now
	}

	if now.Sub(lastPlayersSave) >= 10*time.Second {
		if clmov == "" && !playingMovie && (playersDirty || playersPersistDirty) {
			savePlayersPersist()
			playersPersistDirty = false
		}
		lastPlayersSave = now
	}

	if movieWin != nil && movieWin.IsOpen() {
		if now.Sub(lastMovieWinRefresh) >= time.Second {
			movieWin.Refresh()
			lastMovieWinRefresh = now
		}
	}

	/* Console input */
	changedInput := false
	textChanged := false
	if typingElsewhere && inputActive {
		inputActive = false
		inputText = inputText[:0]
		inputPos = 0
		historyPos = len(inputHistory)
		changedInput = true
		textChanged = true
	}
	if inputActive {
		if newChars := ebiten.AppendInputChars(nil); len(newChars) > 0 {
			if inputPos < 0 {
				inputPos = 0
			}
			if inputPos > len(inputText) {
				inputPos = len(inputText)
			}
			inputText = append(inputText[:inputPos], append(newChars, inputText[inputPos:]...)...)
			inputPos += len(newChars)
			changedInput = true
			textChanged = true
		}
		ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
		if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyV) {
			if txt := clipboard.Read(clipboard.FmtText); len(txt) > 0 {
				runes := []rune(string(txt))
				inputText = append(inputText[:inputPos], append(runes, inputText[inputPos:]...)...)
				inputPos += len(runes)
				changedInput = true
				textChanged = true
			}
		}
		if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyC) {
			clipboard.Write(clipboard.FmtText, []byte(string(inputText)))
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if inputPos > 0 {
				inputPos--
				changedInput = true
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if inputPos < len(inputText) {
				inputPos++
				changedInput = true
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			if len(inputHistory) > 0 {
				if historyPos > 0 {
					historyPos--
				} else {
					historyPos = 0
				}
				inputText = []rune(inputHistory[historyPos])
				inputPos = len(inputText)
				changedInput = true
				textChanged = true
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			if len(inputHistory) > 0 {
				if historyPos < len(inputHistory)-1 {
					historyPos++
					inputText = []rune(inputHistory[historyPos])
					inputPos = len(inputText)
					changedInput = true
					textChanged = true
				} else {
					historyPos = len(inputHistory)
					inputText = inputText[:0]
					inputPos = 0
					changedInput = true
					textChanged = true
				}
			}
		}
		if len(inputText) > 0 && now.Sub(lastBackpace) > time.Millisecond*keyRepeatRate {
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
				if inputPos > 0 {
					lastBackpace = now
					inputText = append(inputText[:inputPos-1], inputText[inputPos:]...)
					inputPos--
					changedInput = true
					textChanged = true
				}
			} else if d := inpututil.KeyPressDuration(ebiten.KeyBackspace); d > 30 {
				if inputPos > 0 {
					lastBackpace = now
					inputText = append(inputText[:inputPos-1], inputText[inputPos:]...)
					inputPos--
					changedInput = true
					textChanged = true
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			orig := string(inputText)
			txt := runInputHandlers(orig)
			txt = strings.TrimSpace(txt)
			if txt == "" {
				// If handlers removed the text, fall back to the user's
				// original entry so it's still sent.
				txt = strings.TrimSpace(orig)
			}
			if txt != "" {
				if strings.HasPrefix(txt, "/play ") {
					tune := strings.TrimSpace(txt[len("/play "):])
					if musicDebug {
						msg := "/play " + tune
						consoleMessage(msg)
						chatMessage(msg)
						log.Print(msg)
					}
					go func() {
						if err := playClanLordTune(tune); err != nil {
							log.Printf("play tune: %v", err)
							if musicDebug {
								consoleMessage("play tune: " + err.Error())
								chatMessage("play tune: " + err.Error())
							}
						}
					}()
				} else {
					// Try built-in or script-registered commands first
					if strings.HasPrefix(txt, "/") {
						lower := strings.ToLower(txt)
						if strings.HasPrefix(lower, "/testhooks") {
							consoleMessage("> " + txt)
							arg := strings.TrimSpace(txt[len("/testhooks"):])
							testScriptHooks(arg)
						} else {
							parts := strings.SplitN(strings.TrimPrefix(txt, "/"), " ", 2)
							name := strings.ToLower(parts[0])
							args := ""
							if len(parts) > 1 {
								args = parts[1]
							}
							if handler, ok := scriptCommands[name]; ok && handler != nil {
								owner := scriptCommandOwners[name]
								if !scriptDisabled[owner] {
									consoleMessage("> " + txt)
									scriptLogEvent(owner, "Command", args)
									go handler(args)
								} else {
									// Disabled script commands should fall through so the
									// server still receives the user's input.
									pendingCommand = txt
								}
							} else {
								pendingCommand = txt
							}
						}
					} else {
						pendingCommand = txt
					}
					// consoleMessage("> " + txt)
				}
				inputHistory = append(inputHistory, txt)
			}
			if gs.InputBarAlwaysOpen {
				inputActive = true
			} else {
				inputActive = false
			}
			inputText = inputText[:0]
			inputPos = 0
			historyPos = len(inputHistory)
			changedInput = true
			textChanged = true
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			inputActive = false
			inputText = inputText[:0]
			inputPos = 0
			historyPos = len(inputHistory)
			changedInput = true
			textChanged = true
		}
	} else if !typingElsewhere {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			inputActive = true
			inputText = inputText[:0]
			inputPos = 0
			historyPos = len(inputHistory)
			changedInput = true
			textChanged = true
		}
	}

	if textChanged {
		spellDirty = true
	}
	if changedInput {
		updateConsoleWindow()
		if consoleWin != nil {
			consoleWin.Refresh()
		}
	}

	if inputFlow != nil && len(inputFlow.Contents) > 0 {
		showSpellSuggestions(inputFlow.Contents[0])
	}

	focused = ebiten.IsFocused()

	/* WASD / ARROWS */

	var keyWalk bool
	if focused && !inputActive && !typingElsewhere {
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
		}
	}
	if focused && !inputActive && !typingElsewhere && gs.JoystickEnabled && selectedJoystick >= 0 && selectedJoystick < len(joystickIDs) && gs.JoystickWalkStick >= 0 {
		id := joystickIDs[selectedJoystick]
		axis := gs.JoystickWalkStick * 2
		if axis+1 < ebiten.GamepadAxisCount(id) {
			ax := ebiten.GamepadAxisValue(id, axis)
			ay := ebiten.GamepadAxisValue(id, axis+1)
			if math.Abs(ax) > gs.JoystickWalkDeadzone || math.Abs(ay) > gs.JoystickWalkDeadzone {
				keyWalk = true
				keyX = int16(ax * float64(fieldCenterX))
				keyY = int16(ay * float64(fieldCenterY))
			}
		}
	}
	if !keyWalk && keyWalkPrev {
		keyStopFrames = 3
	}
	keyWalkPrev = keyWalk

	mx, my = eui.PointerPosition()
	if gs.JoystickEnabled && selectedJoystick >= 0 && selectedJoystick < len(joystickIDs) && gs.JoystickCursorStick >= 0 {
		id := joystickIDs[selectedJoystick]
		axis := gs.JoystickCursorStick * 2
		if axis+1 < ebiten.GamepadAxisCount(id) {
			ax := ebiten.GamepadAxisValue(id, axis)
			ay := ebiten.GamepadAxisValue(id, axis+1)
			if math.Abs(ax) > gs.JoystickCursorDeadzone || math.Abs(ay) > gs.JoystickCursorDeadzone {
				if joyCursorX == 0 && joyCursorY == 0 {
					joyCursorX, joyCursorY = float64(mx), float64(my)
				}
				joyCursorX += float64(ax) * 5
				joyCursorY += float64(ay) * 5
				winW, winH := eui.ScreenSize()
				if joyCursorX < 0 {
					joyCursorX = 0
				} else if joyCursorX > float64(winW-1) {
					joyCursorX = float64(winW - 1)
				}
				if joyCursorY < 0 {
					joyCursorY = 0
				} else if joyCursorY > float64(winH-1) {
					joyCursorY = float64(winH - 1)
				}
				mx, my = int(joyCursorX), int(joyCursorY)
			} else {
				joyCursorX, joyCursorY = float64(mx), float64(my)
			}
		}
	}
	inGame := pointInGameWindow(mx, my)
	// Map mouse to world coordinates accounting for current draw scale/offset.
	baseX := int16(float64(mx-worldOriginX)/worldScale - float64(fieldCenterX))
	baseY := int16(float64(my-worldOriginY)/worldScale - float64(fieldCenterY))
	heldTime := inpututil.MouseButtonPressDuration(ebiten.MouseButtonLeft)
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || joyClick1
	rightClick := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) || joyClick2
	middleClick := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) || joyClick3

	inWindow := pointInAppScreen(mx, my)
	if !focused {
		if walkToggled {
			walkToggled = false
		}
	}
	if !focused || !inWindow {
		click = false
		rightClick = false
		middleClick = false
		heldTime = 0
	}

	stopWalkIfOutside(click, inGame)
	inputMu.Lock()
	prev := latestInput
	inputMu.Unlock()
	if click && pointInUI(mx, my) {
		uiMouseDown = true
	}
	if uiMouseDown {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			uiMouseDown = false
		} else {
			click = false
			heldTime = 0
		}
	}
	if click && !uiMouseDown && inGame {
		handleWorldClick(baseX, baseY, ebiten.MouseButtonLeft)
	}
	if rightClick && inGame && !pointInUI(mx, my) {
		handleWorldClick(baseX, baseY, ebiten.MouseButtonRight)
	}
	if middleClick && inGame && !pointInUI(mx, my) {
		handleWorldClick(baseX, baseY, ebiten.MouseButtonMiddle)
	}
	// (right-click handling for menus/copy is handled earlier)

	// Default desired target from current pointer, even if outside game window.
	// We'll freeze it to the previous value only when we're NOT walking.
	x, y := baseX, baseY
	walk := false
	if !uiMouseDown {
		if keyWalk {
			x, y, walk = keyX, keyY, true
			walkToggled = false
		} else if gs.ClickToToggle {
			if click && inGame {
				walkToggled = !walkToggled
			}
			walk = walkToggled
		} else if continueHeldWalk(prev, inGame, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), heldTime, click) {
			walk = true
			walkToggled = false
		}
	}
	if !focused {
		walk = false
	}

	/* Change Cursor */
	if walk && !keyWalk {
		ebiten.SetCursorShape(ebiten.CursorShapeCrosshair)
	} else {
		ebiten.SetCursorShape(ebiten.CursorShapeDefault)
	}

	// If the pointer is outside the game window and we're not walking,
	// keep the last target so idle mouse movement doesn't jitter the server
	// input. When walking, continue tracking the live pointer position even
	// outside the window as requested.
	if !inGame && !walk {
		x, y = prev.mouseX, prev.mouseY
	}

	queueInput(inputState{mouseX: x, mouseY: y, mouseDown: walk})

	// Warn about poor performance and suggest disabling shaders.
	// Suppress this while intentionally lowering FPS due to power saving
	// (background/unfocused or always-on power save).
	if tcpConn != nil && gs.ShaderLighting && gs.PromptDisableShaders && !shaderWarnShown {
		powerSaving := gs.PowerSaveAlways || (!ebiten.IsFocused() && gs.PowerSaveBackground)
		if !powerSaving && ebiten.ActualFPS() < 50 {
			if lowFPSSince.IsZero() {
				lowFPSSince = now
			} else if now.Sub(lowFPSSince) >= 30*time.Second {
				shaderWarnShown = true
				showShaderDisablePrompt()
			}
		} else {
			lowFPSSince = time.Time{}
		}
	}

	updateHotkeyRecording()
	checkHotkeys()

	return nil
}

func stopWalkIfOutside(click, inGame bool) {
	if gs.ClickToToggle && click && !inGame {
		walkToggled = false
	}
}

func continueHeldWalk(prev inputState, inGame, buttonPressed bool, heldTime int, click bool) bool {
	return (heldTime > 1 && !click && inGame) || (prev.mouseDown && buttonPressed)
}

func queueInput(s inputState) {
	inputMu.Lock()
	switch len(inputQueue) {
	case 0:
		if latestInput != s {
			inputQueue = append(inputQueue, s)
		}
	case 1:
		if inputQueue[0] != s {
			inputQueue = append(inputQueue, s)
		}
	default:
		if inputQueue[len(inputQueue)-1] != s {
			inputQueue[len(inputQueue)-1] = s
		}
	}
	inputMu.Unlock()
}

func updateGameWindowSize() {
	if gameWin == nil {
		return
	}
	size := gameWin.GetRawSize()
	desiredW := int(math.Round(float64(size.X)))
	desiredH := int(math.Round(float64(size.Y)))
	gameWin.SetSize(eui.Point{X: float32(desiredW), Y: float32(desiredH)})
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

// worldDrawInfo reports the on-screen origin (top-left) of the rendered world
// inside the game window, and the effective scale in pixels per world unit.
// This matches the draw-time composition logic so input stays aligned even
// when the window size or aspect ratio changes.
func worldDrawInfo() (int, int, float64) {
	gx, gy := gameWindowOrigin()
	if gameWin == nil {
		// Fallback to current game scale with no offset.
		if gs.GameScale <= 0 {
			return gx, gy, 1.0
		}
		return gx, gy, gs.GameScale
	}

	// Derive the inner content buffer size used for the game image.
	size := gameWin.GetSize()
	pad := float64(2 * gameWin.Padding)
	title := float64(gameWin.GetTitleSize())
	cw := int(float64(int(size.X)&^1) - pad)         // content width
	ch := int(float64(int(size.Y)&^1) - pad - title) // content height
	// Leave a 2px margin on all sides (matches gameImageItem.Position and sizing).
	bufW := cw - 4
	bufH := ch - 4
	if bufW <= 0 || bufH <= 0 {
		if gs.GameScale <= 0 {
			return gx, gy, 1.0
		}
		return gx, gy, gs.GameScale
	}

	// Match Draw() scaling rules.
	const maxSuperSampleScale = 4
	worldW, worldH := gameAreaSizeX, gameAreaSizeY

	// Slider-desired scale.
	desired := int(math.Round(gs.GameScale))
	if desired < 1 {
		desired = 1
	}
	if desired > 10 {
		desired = 10
	}

	// Use the slider-selected scale directly for the offscreen render target.
	offIntScale := desired
	if offIntScale > maxSuperSampleScale {
		offIntScale = maxSuperSampleScale
	}
	if offIntScale < 1 {
		offIntScale = 1
	}

	offW := worldW * offIntScale
	offH := worldH * offIntScale

	scaleDown := math.Min(float64(bufW)/float64(offW), float64(bufH)/float64(offH))

	drawW := float64(offW) * scaleDown
	drawH := float64(offH) * scaleDown
	tx := (float64(bufW) - drawW) / 2
	ty := (float64(bufH) - drawH) / 2

	// Add the 2px inner margin to the window origin to reach the game image.
	originX := gx + 2 + int(math.Round(tx))
	originY := gy + 2 + int(math.Round(ty))
	// Effective world scale on screen in pixels per world unit.
	effScale := float64(offIntScale) * scaleDown
	if effScale <= 0 {
		effScale = 1.0
	}
	return originX, originY, effScale
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Power-save throttling: measure draw duration and sleep remaining time
	// to achieve the requested FPS when active.
	if gs.PowerSaveAlways || (!ebiten.IsFocused() && gs.PowerSaveBackground) {
		frameStart := time.Now()
		fps := gs.PowerSaveFPS
		if fps < 1 {
			fps = 1
		}
		if fps > 45 {
			fps = 45
		}
		target := time.Second / time.Duration(fps)
		defer func() {
			if elapsed := time.Since(frameStart); elapsed < target {
				time.Sleep(target - elapsed)
			}
		}()
	}
	worldOriginX, worldOriginY, worldScale = worldDrawInfo()
	// Cache now for the whole draw to reduce time.Now overhead.
	now := time.Now()

	//Reduce render load while seeking clMov
	if seekingMov {
		if now.Sub(lastSeekPrev) < time.Millisecond*200 {
			return
		}
		lastSeekPrev = now
		gameImageItem.Disabled = true
	} else {
		gameImageItem.Disabled = false
	}
	if backgroundImg != nil {
		drawBackground(screen)
	} else {
		screen.Fill(dimmedScreenBG)
	}

	// Ensure the game image item/buffer exists and matches window content.
	updateGameImageSize()
	if gameImage == nil {
		// UI not ready yet
		worldViewRect = image.Rectangle{}
		worldRTUsedRect = image.Rectangle{}
		eui.Draw(screen)
		return
	}

	// Determine offscreen render scale and composite scale.
	// A user-selected render scale (gs.GameScale) in 1x..10x acts as a
	// supersample factor. The window is always filled using linear filtering.
	bufW := gameImage.Bounds().Dx()
	bufH := gameImage.Bounds().Dy()
	gameImage.Fill(color.Black)
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
	// Use the slider-selected scale directly for offscreen rendering
	offIntScale := desired
	if offIntScale > maxSuperSampleScale {
		offIntScale = maxSuperSampleScale
	}
	if offIntScale < 1 {
		offIntScale = 1
	}

	// Prepare variable-sized offscreen target (supersampled)
	offW := worldW * offIntScale
	offH := worldH * offIntScale
	ensureWorldRT(offW, offH)
	worldRect := image.Rect(0, 0, offW, offH)
	worldRTUsedRect = worldRect
	worldView := worldRT.SubImage(worldRect).(*ebiten.Image)
	worldView.Fill(color.Black)

	// Render splash or live frame into worldRT using the offscreen scale
	var snap drawSnapshot
	var alpha float64
	var haveSnap bool
	if clmov == "" && !playingMovie && tcpConn == nil && pcapPath == "" && !fake {
		prev := gs.GameScale
		gs.GameScale = float64(offIntScale)
		drawSplash(worldView, 0, 0)
		gs.GameScale = prev
	} else {
		snap = captureDrawSnapshot()
		var mobileFade, pictFade float32
		alpha, mobileFade, pictFade = computeInterpolation(now, snap.prevTime, snap.curTime, gs.MobileBlendAmount, gs.BlendAmount)
		prev := gs.GameScale
		gs.GameScale = float64(offIntScale)
		drawScene(worldView, 0, 0, snap, alpha, mobileFade, pictFade)
		if gs.ShaderLighting {
			// Use shader-based night darkening with inverse-square falloff.
			addNightDarkSources(offW, offH, float32(alpha))
		} else {
			// Classic overlay path when shader is off.
			//drawNightAmbient(worldView, 0, 0)
			drawNightOverlay(worldView, 0, 0)
		}
		if gs.ShaderLighting {
			// Apply lighting on the active subimage only
			applyLightingShader(worldView, frameLights, frameDarks, float32(alpha))
		}
		drawStatusBars(worldView, 0, 0, snap, alpha)
		gs.GameScale = prev
		haveSnap = true
	}

	// Composite worldRT into the gameImage buffer: scale/center
	// Keep this simple: the offscreen world is rendered at integer scale
	// (nearest) and the final composite to the resizable window uses linear.
	scaleDown := math.Min(float64(bufW)/float64(offW), float64(bufH)/float64(offH))
	sx, sy := scaleDown, scaleDown
	drawW := float64(offW) * sx
	drawH := float64(offH) * sy
	tx := (float64(bufW) - drawW) / 2
	ty := (float64(bufH) - drawH) / 2
	op := acquireDrawOpts()
	if gs.PixelArtScaling {
		op.Filter = ebiten.FilterNearest
	} else {
		op.Filter = ebiten.FilterLinear
	}
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(tx, ty)
	gameImage.DrawImage(worldView, op)
	releaseDrawOpts(op)
	left := roundToInt(tx)
	top := roundToInt(ty)
	right := left + roundToInt(drawW)
	bottom := top + roundToInt(drawH)
	if left < 0 {
		left = 0
	}
	if top < 0 {
		top = 0
	}
	if right > bufW {
		right = bufW
	}
	if bottom > bufH {
		bottom = bufH
	}
	viewRect := image.Rect(left, top, right, bottom).Intersect(gameImage.Bounds())
	if viewRect.Empty() {
		worldViewRect = image.Rectangle{}
	} else {
		worldViewRect = viewRect
	}
	var finalScale float64
	if haveSnap {
		prev := gs.GameScale
		finalScale = float64(offIntScale) * scaleDown
		if finalScale <= 0 {
			finalScale = worldScale
		}
		if finalScale <= 0 {
			finalScale = 1
		}
		windowScale := finalScale / gsdef.GameScale
		if windowScale <= 0 {
			windowScale = 1
		}
		gs.GameScale = finalScale
		if !viewRect.Empty() {
			worldView := gameImage.SubImage(viewRect).(*ebiten.Image)
			drawSpeechBubbles(worldView, snap, alpha, windowScale)
			// Draw script overlays on top of the world view.
			drawScriptOverlays(worldView, finalScale)
			// Recording/Playback badge in top-left of world view
			drawRecPlayBadge(worldView)
		}
		gs.GameScale = prev
	}

	// Finally, draw UI (which includes the game window image)
	eui.Draw(screen)

	// Old fixed background sleep replaced by deferred power-save throttle above.

	//if gs.ShowFPS {
	//}

	if seekingMov {
		x, y := float64(screen.Bounds().Dx())/2, float64(screen.Bounds().Dy())/2
		vector.FillRect(screen, float32(x+2), float32(y+2), 90, 40, color.Black, false)

		op := acquireTextDrawOpts()
		op.GeoM.Translate(x, y)
		text.Draw(screen, "SEEKING...", mainFontBold, op)
		releaseTextDrawOpts(op)
	}
}

var lastSeekPrev time.Time

func drawRecPlayBadge(dst *ebiten.Image) {
	// Only show when actively recording/armed or playing back.
	showRec := recorder != nil || recordingMovie
	showPlay := !showRec && playingMovie
	if !showRec && !showPlay {
		return
	}
	// Pulse alpha between ~0.5 and 1.0
	t := float64(time.Now().UnixNano()) / 1e9
	s := 0.5 + 0.5*math.Sin(t*2*math.Pi/1.6)
	alpha := 0.6 + 0.4*s
	var base color.RGBA
	var label string
	if showRec {
		base = color.RGBA{R: 203, G: 67, B: 53, A: 255} // red
		label = "REC"
	} else {
		base = color.RGBA{R: 40, G: 180, B: 99, A: 255} // green
		label = "PLAY"
	}
	col := color.RGBA{R: base.R, G: base.G, B: base.B, A: uint8(alpha * 255)}
	// Position near top-left
	pad := float32(6)
	cx := float32(10)
	cy := float32(10)
	r := float32(6)
	vector.FillCircle(dst, cx+pad, cy+pad, r, col, false)
	// Text to the right
	op := acquireTextDrawOpts()
	op.GeoM.Translate(float64(2*pad+r*2), float64(4+pad))
	op.ColorScale.Scale(1, 1, 1, float32(alpha))
	text.Draw(dst, label, mainFontBold, op)
	releaseTextDrawOpts(op)
}

// drawScene renders all world objects for the current frame.
func drawScene(screen *ebiten.Image, ox, oy int, snap drawSnapshot, alpha float64, mobileFade, pictFade float32) {
	if gs.ShaderLighting {
		frameLights = frameLights[:0]
		frameDarks = frameDarks[:0]
	}

	// Use cached descriptor map directly; no need to rebuild/sort it per frame.
	descMap := snap.descriptors
	mobileLimit := maxMobileInterpPixels * (snap.dropped + 1)

	// Use precomputed, sorted partitions
	negPics := snap.picsNeg
	zeroPics := snap.picsZero
	posPics := snap.picsPos
	live := snap.liveMobs
	dead := snap.deadMobs

	for _, p := range negPics {
		drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, descMap, snap.prevMobiles, snap.prevPictures, snap.picShiftX, snap.picShiftY)
	}

	if gs.hideMobiles {
		for _, p := range zeroPics {
			drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, descMap, snap.prevMobiles, snap.prevPictures, snap.picShiftX, snap.picShiftY)
		}
	} else {
		for _, m := range dead {
			drawMobile(screen, ox, oy, m, descMap, snap.prevMobiles, snap.prevDescs, snap.picShiftX, snap.picShiftY, alpha, mobileFade, mobileLimit)
			drawMobileNameTag(screen, snap, m, alpha)
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
					drawMobile(screen, ox, oy, live[i], descMap, snap.prevMobiles, snap.prevDescs, snap.picShiftX, snap.picShiftY, alpha, mobileFade, mobileLimit)
					drawMobileNameTag(screen, snap, live[i], alpha)
				}
				i++
			} else {
				drawPicture(screen, ox, oy, zeroPics[j], alpha, pictFade, snap.mobiles, descMap, snap.prevMobiles, snap.prevPictures, snap.picShiftX, snap.picShiftY)
				j++
			}
		}
	}

	for _, p := range posPics {
		drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, descMap, snap.prevMobiles, snap.prevPictures, snap.picShiftX, snap.picShiftY)
	}
}

// drawMobile renders a single mobile object with optional interpolation and onion skinning.
// When a mobile lacks history but the world shifts, a pseudo-previous position
// derived from picShift provides a one-frame interpolation. maxDist sets the
// maximum allowed pixel delta for interpolation.
func drawMobile(screen *ebiten.Image, ox, oy int, m frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, prevDescs map[uint8]frameDescriptor, shiftX, shiftY int, alpha float64, fade float32, maxDist int) {
	h := float64(m.H)
	v := float64(m.V)
	if gs.MotionSmoothing {
		if pm, ok := prevMobiles[m.Index]; ok {
			dh := int(m.H) - int(pm.H) - shiftX
			dv := int(m.V) - int(pm.V) - shiftY
			dist := dh*dh + dv*dv
			if dist <= maxDist*maxDist {
				h = float64(pm.H)*(1-alpha) + float64(m.H)*alpha
				v = float64(pm.V)*(1-alpha) + float64(m.V)*alpha
			}
		} else if shiftX != 0 || shiftY != 0 {
			dh := shiftX
			dv := shiftY
			if dh*dh+dv*dv <= maxDist*maxDist {
				prevH := float64(int(m.H) - shiftX)
				prevV := float64(int(m.V) - shiftY)
				h = prevH*(1-alpha) + float64(m.H)*alpha
				v = prevV*(1-alpha) + float64(m.V)*alpha
			}
		}
	}
	x := roundToInt((h + float64(fieldCenterX)) * gs.GameScale)
	y := roundToInt((v + float64(fieldCenterY)) * gs.GameScale)
	x += ox
	y += oy
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
	curKey := makeMobileKey(d.PictID, state, colors)
	img = getScaledMobileFrame(curKey, img)
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
			prevKey := makeMobileKey(prevPict, prevState, prevColors)
			prevImg = getScaledMobileFrame(prevKey, prevImg)
		}
	}
	if img != nil {
		size := mobileSize(d.PictID)
		if size == 0 {
			size = img.Bounds().Dx()
		}
		addLightSource(uint32(d.PictID), float64(x), float64(y), size)
		blend := gs.BlendMobiles && prevImg != nil && fade > 0 && fade < 1
		var src *ebiten.Image
		drawSize := img.Bounds().Dx()
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
			drawSize = img.Bounds().Dx()
		}
		target := float64(roundToInt(float64(size) * gs.GameScale))
		scale := gs.GameScale
		if drawSize > 0 {
			scale = target / float64(drawSize)
		}
		scaled := float64(drawSize) * scale
		op := acquireDrawOpts()
		op.Filter = ebiten.FilterNearest
		op.DisableMipmaps = true
		op.GeoM.Scale(scale, scale)
		tx := float64(x) - scaled/2
		ty := float64(y) - scaled/2
		op.GeoM.Translate(tx, ty)
		screen.DrawImage(src, op)
		releaseDrawOpts(op)
		if gs.imgPlanesDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dm", plane)
			xPos := x - int(float64(size)*gs.GameScale/2)
			op := acquireTextDrawOpts()
			op.GeoM.Translate(float64(xPos), float64(y)-float64(size)*gs.GameScale/2-metrics.HAscent)
			op.ColorScale.ScaleWithColor(color.RGBA{0, 255, 255, 255})
			text.Draw(screen, lbl, mainFont, op)
			releaseTextDrawOpts(op)
		}
	} else {
		// Fallback marker when image missing; no per-frame bounds check.
		vector.FillRect(screen, float32(float64(x)-3*gs.GameScale), float32(float64(y)-3*gs.GameScale), float32(6*gs.GameScale), float32(6*gs.GameScale), color.RGBA{0xff, 0, 0, 0xff}, false)
		if gs.imgPlanesDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dm", plane)
			xPos := x - int(3*gs.GameScale)
			op := acquireTextDrawOpts()
			op.GeoM.Translate(float64(xPos), float64(y)-3*gs.GameScale-metrics.HAscent)
			op.ColorScale.ScaleWithColor(color.White)
			text.Draw(screen, lbl, mainFont, op)
			releaseTextDrawOpts(op)
		}
	}
}

func pictureObscuresMobileAt(pictID uint16, frame int, pictH, pictV int16, mob frameMobile, mobDesc frameDescriptor) bool {
	if clImages == nil || clImages.IsSemiTransparent(uint32(pictID)) {
		return false
	}
	w, h := clImages.Size(uint32(pictID))
	if w <= 0 || h <= 0 {
		return false
	}
	frames := clImages.NumFrames(uint32(pictID))
	if frames > 1 {
		h /= frames
	}
	size := mobileSize(mobDesc.PictID)
	if size == 0 {
		return false
	}

	picL := int(pictH) - w/2
	picR := picL + w
	picT := int(pictV) - h/2
	picB := picT + h
	mL := int(mob.H) - size/2
	mR := mL + size
	mT := int(mob.V) - size/2
	mB := mT + size
	interL := picL
	if mL > interL {
		interL = mL
	}
	interR := picR
	if mR < interR {
		interR = mR
	}
	interT := picT
	if mT > interT {
		interT = mT
	}
	interB := picB
	if mB < interB {
		interB = mB
	}
	if interR <= interL || interB <= interT {
		return false
	}

	picMask := clImages.AlphaMaskQuarter(uint32(pictID), false)
	mobMask := clImages.AlphaMaskQuarter(uint32(mobDesc.PictID), true)
	if picMask == nil || mobMask == nil {
		return false
	}

	picFrameOffsetY := (frame * h) >> 2
	picX0 := (interL - picL) >> 2
	picY0 := picFrameOffsetY + ((interT - picT) >> 2)
	picX1 := (interR - picL + 3) >> 2
	picY1 := picFrameOffsetY + ((interB - picT + 3) >> 2)

	mobFrameX := int(mob.State&0x0F) * size
	mobFrameY := int(mob.State>>4) * size
	mobX0 := (mobFrameX + (interL - mL)) >> 2
	mobY0 := (mobFrameY + (interT - mT)) >> 2
	mobX1 := (mobFrameX + (interR - mL) + 3) >> 2
	mobY1 := (mobFrameY + (interB - mT) + 3) >> 2

	if picX0 < 0 {
		picX0 = 0
	}
	if picY0 < 0 {
		picY0 = 0
	}
	if mobX0 < 0 {
		mobX0 = 0
	}
	if mobY0 < 0 {
		mobY0 = 0
	}
	if picX1 > picMask.W {
		picX1 = picMask.W
	}
	if picY1 > picMask.H {
		picY1 = picMask.H
	}
	if mobX1 > mobMask.W {
		mobX1 = mobMask.W
	}
	if mobY1 > mobMask.H {
		mobY1 = mobMask.H
	}

	width := picX1 - picX0
	if w := mobX1 - mobX0; w < width {
		width = w
	}
	height := picY1 - picY0
	if h := mobY1 - mobY0; h < height {
		height = h
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if picMask.Opaque(picX0+x, picY0+y) && mobMask.Opaque(mobX0+x, mobY0+y) {
				return true
			}
		}
	}
	return false
}

// pictureDrawsAfterMobileAt reports whether a picture at the given position
// would be drawn after a mobile based on plane and sort order.
func pictureDrawsAfterMobileAt(p framePicture, pictH, pictV int16, mobH, mobV int16, mobPlane int) bool {
	if p.Plane > mobPlane {
		return true
	}
	if p.Plane < mobPlane {
		return false
	}
	if int(mobV) < int(pictV) {
		return true
	}
	if int(mobV) > int(pictV) {
		return false
	}
	return int(mobH) <= int(pictH)
}

// drawPicture renders a single picture sprite.
func drawPicture(screen *ebiten.Image, ox, oy int, p framePicture, alpha float64, fade float32, mobiles []frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, prevPictures []framePicture, shiftX, shiftY int) {
	if gs.hideMoving && p.Moving {
		return
	}
	offX := float64(int(p.PrevH)-int(p.H)) * (1 - alpha)
	offY := float64(int(p.PrevV)-int(p.V)) * (1 - alpha)
	if p.Moving && !gs.smoothMoving {
		if int(p.PrevH) == int(p.H)-shiftX && int(p.PrevV) == int(p.V)-shiftY {
			//
		} else {
			offX = 0
			offY = 0
		}
	}

	frame := 0
	prevFrame := 0
	if clImages != nil {
		frame = clImages.FrameIndex(uint32(p.PictID), frameCounter)
		prevFrame = clImages.FrameIndex(uint32(p.PictID), frameCounter-1)
	}
	plane := p.Plane

	w, h := 0, 0
	if clImages != nil {
		w, h = clImages.Size(uint32(p.PictID))
		if frames := clImages.NumFrames(uint32(p.PictID)); frames > 1 {
			h /= frames
		}
	}

	var mobileX, mobileY float64
	if gs.ObjectPinning && gs.MotionSmoothing && w <= 500 && h <= 500 {
		if dx, dy, ok := pictureMobileOffset(p, mobiles, prevMobiles, prevPictures, alpha); ok {
			mobileX, mobileY = dx, dy
			offX = 0
			offY = 0
		}
	}

	x := roundToInt(((float64(p.H) + offX + mobileX) + float64(fieldCenterX)) * gs.GameScale)
	y := roundToInt(((float64(p.V) + offY + mobileY) + float64(fieldCenterY)) * gs.GameScale)
	x += ox
	y += oy

	addLightSource(uint32(p.PictID), float64(x), float64(y), w)

	img := loadImageFrame(p.PictID, frame)
	img = getScaledPictureFrame(p.PictID, frame, img)
	fadeAlpha := float32(1.0)
	if gs.FadeObscuringPictures && w > 0 && h > 0 && clImages != nil && !clImages.IsSemiTransparent(uint32(p.PictID)) {
		for _, m := range mobiles {
			d, ok := descMap[m.Index]
			if !ok {
				continue
			}
			prevMob := m
			if pm, ok := prevMobiles[m.Index]; ok {
				prevMob = pm
			}
			prevDrawAfter := pictureDrawsAfterMobileAt(p, p.PrevH, p.PrevV, prevMob.H, prevMob.V, d.Plane)
			currDrawAfter := pictureDrawsAfterMobileAt(p, p.H, p.V, m.H, m.V, d.Plane)
			if !prevDrawAfter && !currDrawAfter {
				continue
			}
			prevObscure := pictureObscuresMobileAt(p.PictID, prevFrame, p.PrevH, p.PrevV, prevMob, d)
			currObscure := pictureObscuresMobileAt(p.PictID, frame, p.H, p.V, m, d)
			prevAlpha := float32(1.0)
			if prevObscure && prevDrawAfter {
				prevAlpha = float32(gs.ObscuringPictureOpacity)
			}
			targetAlpha := float32(1.0)
			if currObscure && currDrawAfter {
				targetAlpha = float32(gs.ObscuringPictureOpacity)
			}
			mobFade := prevAlpha + (targetAlpha-prevAlpha)*fade
			if mobFade < fadeAlpha {
				fadeAlpha = mobFade
			}
		}
	}
	var prevImg *ebiten.Image
	if gs.BlendPicts && clImages != nil {
		if prevFrame != frame {
			prevImg = loadImageFrame(p.PictID, prevFrame)
			prevImg = getScaledPictureFrame(p.PictID, prevFrame, prevImg)
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
		targetW := float64(roundToInt(float64(w) * gs.GameScale))
		targetH := float64(roundToInt(float64(h) * gs.GameScale))
		if targetW <= 0 && drawW > 0 {
			targetW = float64(drawW)
		}
		if targetH <= 0 && drawH > 0 {
			targetH = float64(drawH)
		}
		sx := gs.GameScale
		sy := gs.GameScale
		if drawW > 0 {
			sx = targetW / float64(drawW)
		}
		if drawH > 0 {
			sy = targetH / float64(drawH)
		}
		scaledW := float64(drawW) * sx
		scaledH := float64(drawH) * sy
		op := acquireDrawOpts()
		op.Filter = ebiten.FilterNearest
		op.DisableMipmaps = true
		op.GeoM.Scale(sx, sy)
		tx := float64(x) - scaledW/2
		ty := float64(y) - scaledH/2
		op.GeoM.Translate(tx, ty)
		if gs.pictAgainDebug && p.Again {
			op.ColorScale.Scale(0, 0, 1, 1)
		} else if src == img && gs.smoothingDebug && p.Moving {
			op.ColorScale.Scale(1, 0, 0, 1)
		}
		if fadeAlpha < 1 {
			op.ColorScale.ScaleAlpha(fadeAlpha)
		}
		screen.DrawImage(src, op)
		releaseDrawOpts(op)

		if gs.pictIDDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%d", p.PictID)
			txtW, _ := text.Measure(lbl, mainFont, 0)
			xPos := x + int(float64(w)*gs.GameScale/2) - roundToInt(txtW)
			opTxt := acquireTextDrawOpts()
			opTxt.GeoM.Translate(float64(xPos), float64(y)-float64(h)*gs.GameScale/2-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(eui.ColorRed)
			text.Draw(screen, lbl, mainFont, opTxt)
			releaseTextDrawOpts(opTxt)
		}

		if gs.imgPlanesDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dp", plane)
			xPos := x - int(float64(w)*gs.GameScale/2)
			opTxt := acquireTextDrawOpts()
			opTxt.GeoM.Translate(float64(xPos), float64(y)-float64(h)*gs.GameScale/2-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(color.RGBA{255, 255, 0, 0})
			text.Draw(screen, lbl, mainFont, opTxt)
			releaseTextDrawOpts(opTxt)
		}
	} else {
		clr := color.RGBA{0, 0, 0xff, 0xff}
		if gs.smoothingDebug && p.Moving {
			clr = color.RGBA{0xff, 0, 0, 0xff}
		}
		if gs.pictAgainDebug && p.Again {
			clr = color.RGBA{0, 0, 0xff, 0xff}
		}
		vector.FillRect(screen, float32(float64(x)-2*gs.GameScale), float32(float64(y)-2*gs.GameScale), float32(4*gs.GameScale), float32(4*gs.GameScale), clr, false)
		if gs.pictIDDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%d", p.PictID)
			txtW, _ := text.Measure(lbl, mainFont, 0)
			half := int(2 * gs.GameScale)
			xPos := x + half - roundToInt(txtW)
			opTxt := acquireTextDrawOpts()
			opTxt.GeoM.Translate(float64(xPos), float64(y)-float64(half)-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(eui.ColorRed)
			text.Draw(screen, lbl, mainFont, opTxt)
			releaseTextDrawOpts(opTxt)
		}
		if gs.imgPlanesDebug {
			metrics := mainFont.Metrics()
			lbl := fmt.Sprintf("%dp", plane)
			xPos := x - int(2*gs.GameScale)
			opTxt := acquireTextDrawOpts()
			opTxt.GeoM.Translate(float64(xPos), float64(y)-2*gs.GameScale-metrics.HAscent)
			opTxt.ColorScale.ScaleWithColor(color.RGBA{255, 255, 0, 0})
			text.Draw(screen, lbl, mainFont, opTxt)
			releaseTextDrawOpts(opTxt)
		}
	}
}

// pictureMobileOffset returns the interpolated offset for a picture that
// should track a mobile when the picture maintains the exact same offset to a
// mobile between frames. This ignores camera shift and only considers
// candidates within a 64x64 box around the mobile. The interpolated result is
// the mobile's interpolation minus the mobile's current position so callers
// can add it to the picture position.
// pictureMobileOffset checks for exact offset match between the picture and a
// mobile across frames using raw coordinates only (no picShift). When matched,
// it returns the mobile's interpolated delta so the picture follows smoothly.
func pictureMobileOffset(p framePicture, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, prevPictures []framePicture, alpha float64) (float64, float64, bool) {
	// Use exact previous picture position for the same PictID to verify the
	// picture-to-mobile offset stayed identical across frames.
	// Try the hero (playerIndex) first to ensure centered player effects pin.
	for i := range mobiles {
		if mobiles[i].Index != playerIndex {
			continue
		}
		m := mobiles[i]
		pm, ok := prevMobiles[m.Index]
		if !ok {
			break
		}
		offH := int(p.H) - int(m.H)
		offV := int(p.V) - int(m.V)
		if offH < -64 || offH > 64 || offV < -64 || offV > 64 {
			break
		}
		expPrevH := int(pm.H) + offH
		expPrevV := int(pm.V) + offV
		for j := range prevPictures {
			pp := prevPictures[j]
			if pp.PictID != p.PictID {
				continue
			}
			if int(pp.H) == expPrevH && int(pp.V) == expPrevV {
				h := float64(pm.H)*(1-alpha) + float64(m.H)*alpha
				v := float64(pm.V)*(1-alpha) + float64(m.V)*alpha
				return h - float64(m.H), v - float64(m.V), true
			}
		}
		break
	}
	bestDist := 64*64 + 1
	var bestDX, bestDY float64
	found := false
	for _, m := range mobiles {
		pm, ok := prevMobiles[m.Index]
		if !ok {
			continue
		}
		offH := int(p.H) - int(m.H)
		offV := int(p.V) - int(m.V)
		if offH < -64 || offH > 64 || offV < -64 || offV > 64 {
			continue
		}
		// Expected previous picture position if offset is identical
		expPrevH := int(pm.H) + offH
		expPrevV := int(pm.V) + offV
		match := false
		for i := range prevPictures {
			pp := prevPictures[i]
			if pp.PictID != p.PictID {
				continue
			}
			if int(pp.H) == expPrevH && int(pp.V) == expPrevV {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		// Interpolate mobile
		h := float64(pm.H)*(1-alpha) + float64(m.H)*alpha
		v := float64(pm.V)*(1-alpha) + float64(m.V)*alpha
		dist := offH*offH + offV*offV
		if dist < bestDist {
			bestDX = h - float64(m.H)
			bestDY = v - float64(m.V)
			bestDist = dist
			found = true
		}
	}
	if found {
		return bestDX, bestDY, true
	}
	// Exact-center case: picture exactly at a mobile; requires prev sample
	for _, m := range mobiles {
		if int(p.H) == int(m.H) && int(p.V) == int(m.V) {
			if pm, ok := prevMobiles[m.Index]; ok {
				h := float64(pm.H)*(1-alpha) + float64(m.H)*alpha
				v := float64(pm.V)*(1-alpha) + float64(m.V)*alpha
				return h - float64(m.H), v - float64(m.V), true
			}
		}
	}
	return 0, 0, false
}

// drawMobileNameTag renders the name tag and color bar for a single mobile.
// It respects motion smoothing and the native name tag setting based on the
// current gs.GameScale.
func drawMobileNameTag(screen *ebiten.Image, snap drawSnapshot, m frameMobile, alpha float64) {
	if wasmPrivacyActive() {
		return
	}
	h := float64(m.H)
	v := float64(m.V)
	if gs.MotionSmoothing {
		if pm, ok := snap.prevMobiles[m.Index]; ok {
			dh := int(m.H) - int(pm.H) - snap.picShiftX
			dv := int(m.V) - int(pm.V) - snap.picShiftY
			maxDist := maxMobileInterpPixels * (snap.dropped + 1)
			if dh*dh+dv*dv <= maxDist*maxDist {
				h = float64(pm.H)*(1-alpha) + float64(m.H)*alpha
				v = float64(pm.V)*(1-alpha) + float64(m.V)*alpha
			}
		}
	}
	x := roundToInt((h + float64(fieldCenterX)) * gs.GameScale)
	y := roundToInt((v + float64(fieldCenterY)) * gs.GameScale)
	if d, ok := snap.descriptors[m.Index]; ok {
		showName := d.Name != ""
		if showName && gs.NameTagsOnHoverOnly {
			lastHoverMu.Lock()
			hovered := lastHover.OnMobile && lastHover.Mobile.Index == m.Index
			lastHoverMu.Unlock()
			if !hovered {
				showName = false
			}
		}
		nameAlpha := uint8(gs.NameBgOpacity*255 + 0.5)
		size := mobileSize(d.PictID)
		if size <= 0 {
			size = 40
		}
		offset := float64(size) * gs.GameScale / 2
		drawGenericBar := func() {
			back := int((m.Colors >> 4) & 0x0f)
			if back != kColorCodeBackWhite && back != kColorCodeBackBlue && !(back == kColorCodeBackBlack && d.Type == kDescMonster) {
				if back >= len(nameBackColors) {
					back = 0
				}
				barClr := nameBackColors[back]
				barClr.A = nameAlpha
				top := y + int(offset+2*gs.GameScale)
				left := x - int(6*gs.GameScale)
				op := acquireDrawOpts()
				op.Filter = ebiten.FilterNearest
				op.DisableMipmaps = true
				op.GeoM.Scale(12*gs.GameScale, 2*gs.GameScale)
				op.GeoM.Translate(float64(left), float64(top))
				op.ColorScale.ScaleWithColor(barClr)
				screen.DrawImage(whiteImage, op)
				releaseDrawOpts(op)
			}
		}
		if showName {
			style := styleRegular
			playersMu.RLock()
			if p, ok := players[d.Name]; ok {
				if p.Sharing && p.Sharee {
					style = styleBoldItalic
				} else if p.Sharing {
					style = styleBold
				} else if p.Sharee {
					style = styleItalic
				}
			}
			playersMu.RUnlock()
			if m.nameTag != nil && m.nameTagKey.FontGen == fontGen && m.nameTagKey.Opacity == nameAlpha && m.nameTagKey.Text == d.Name && m.nameTagKey.Colors == m.Colors && m.nameTagKey.Style == style {
				top := y + int(offset)
				left := x - int(float64(m.nameTagW)/2)
				op := acquireDrawOpts()
				op.Filter = ebiten.FilterNearest
				op.DisableMipmaps = true
				op.GeoM.Translate(float64(left), float64(top))
				screen.DrawImage(m.nameTag, op)
				releaseDrawOpts(op)
			} else {
				// Rebuild the cached name tag image on mismatch to avoid per-frame vector draws.
				// Respect label color frames if enabled.
				_, _, frameClr := mobileNameColors(m.Colors)
				if gs.NameTagLabelColors {
					playersMu.RLock()
					if p, ok := players[d.Name]; ok && p.FriendLabel > 0 && p.FriendLabel <= len(labelColors) {
						lc := labelColors[p.FriendLabel-1]
						frameClr = color.RGBA{lc.R, lc.G, lc.B, 0xff}
					}
					playersMu.RUnlock()
				}
				frameClr.A = nameAlpha
				img, iw, ih := buildNameTagImage(d.Name, m.Colors, nameAlpha, style, frameClr)
				if img != nil {
					// Update shared cache so next frames reuse this image.
					stateMu.Lock()
					if sm, ok := state.mobiles[m.Index]; ok {
						sm.nameTag = img
						sm.nameTagW = iw
						sm.nameTagH = ih
						sm.nameTagKey = nameTagKey{Text: d.Name, Colors: m.Colors, Opacity: nameAlpha, FontGen: fontGen, Style: style}
						state.mobiles[m.Index] = sm
					}
					stateMu.Unlock()

					top := y + int(offset)
					left := x - int(float64(iw)/2)
					op := acquireDrawOpts()
					op.Filter = ebiten.FilterNearest
					op.DisableMipmaps = true
					op.GeoM.Translate(float64(left), float64(top))
					screen.DrawImage(img, op)
					releaseDrawOpts(op)
				}
			}
		} else {
			drawGenericBar()
		}
	}
}

// drawSpeechBubbles renders speech bubbles at native resolution.
func drawSpeechBubbles(screen *ebiten.Image, snap drawSnapshot, alpha float64, windowScale float64) {
	if !gs.SpeechBubbles {
		return
	}
	if wasmPrivacyActive() {
		return
	}
	if windowScale <= 0 {
		windowScale = 1
	}
	bubbleScale := gs.BubbleScale * windowScale
	if bubbleScale < 0.1 {
		bubbleScale = 0.1
	}
	fontScale := windowScale
	if fontScale < 0.1 {
		fontScale = 0.1
	}
	descMap := snap.descriptors
	maxDist := maxMobileInterpPixels * (snap.dropped + 1)
	for _, b := range snap.bubbles {
		bubbleType := b.Type & kBubbleTypeMask
		typeOK := true
		switch bubbleType {
		case kBubbleNormal:
			typeOK = gs.BubbleNormal
		case kBubbleWhisper:
			typeOK = gs.BubbleWhisper
		case kBubbleYell:
			typeOK = gs.BubbleYell
		case kBubbleThought:
			typeOK = gs.BubbleThought
		case kBubbleRealAction:
			typeOK = gs.BubbleRealAction
		case kBubbleMonster:
			typeOK = gs.BubbleMonster
		case kBubblePlayerAction:
			typeOK = gs.BubblePlayerAction
		case kBubblePonder:
			typeOK = gs.BubblePonder
		case kBubbleNarrate:
			typeOK = gs.BubbleNarrate
		}
		originOK := true
		switch {
		case b.Index == playerIndex:
			originOK = gs.BubbleSelf
		case bubbleType == kBubbleMonster:
			originOK = gs.BubbleMonsters
		case bubbleType == kBubbleNarrate:
			originOK = gs.BubbleNarration
		default:
			originOK = gs.BubbleOtherPlayers
		}
		if !(typeOK && originOK) {
			continue
		}
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
						if dh*dh+dv*dv <= maxDist*maxDist {
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
					tailHeight := int(math.Round(10 * bubbleScale))
					y += tailHeight - int(math.Round(float64(size)*gs.GameScale))
				}
			}
		}
		borderCol, bgCol, textCol := bubbleColors(b.Type)
		drawBubble(screen, b.Text, x, y, b.Type, b.Far, b.NoArrow, borderCol, bgCol, textCol, bubbleScale, fontScale)
	}
}

// lerpBar interpolates status bar values, skipping interpolation when the
// current value is lower than the previous.
func lerpBar(prev, cur int, alpha float64) int {
	if cur < prev {
		return cur
	}
	return int(math.Round(float64(prev) + alpha*float64(cur-prev)))
}

// drawStatusBars renders health, balance and spirit bars.
func drawStatusBars(screen *ebiten.Image, ox, oy int, snap drawSnapshot, alpha float64) {
	drawRect := func(x, y, w, h int, clr color.RGBA) {
		op := acquireDrawOpts()
		op.Filter = ebiten.FilterNearest
		op.DisableMipmaps = true
		op.GeoM.Scale(float64(w), float64(h))
		op.GeoM.Translate(float64(ox+x), float64(oy+y))
		op.ColorScale.ScaleWithColor(clr)
		op.ColorScale.ScaleAlpha(float32(gs.BarOpacity))
		screen.DrawImage(whiteImage, op)
		releaseDrawOpts(op)
	}
	barWidth := int(110 * gs.GameScale)
	barHeight := int(8 * gs.GameScale)

	fieldWidth := int(float64(gameAreaSizeX) * gs.GameScale)
	fieldHeight := int(float64(gameAreaSizeY) * gs.GameScale)

	var x, y, dx, dy int
	switch gs.BarPlacement {
	case BarPlacementLowerLeft:
		x = int(20 * gs.GameScale)
		spacing := int(4 * gs.GameScale)
		y = fieldHeight - int(20*gs.GameScale) - 3*barHeight - 2*spacing
		dx = 0
		dy = barHeight + spacing
	case BarPlacementLowerRight:
		x = fieldWidth - int(20*gs.GameScale) - barWidth
		spacing := int(4 * gs.GameScale)
		y = fieldHeight - int(20*gs.GameScale) - 3*barHeight - 2*spacing
		dx = 0
		dy = barHeight + spacing
	case BarPlacementUpperRight:
		x = fieldWidth - int(20*gs.GameScale) - barWidth
		spacing := int(4 * gs.GameScale)
		y = int(20 * gs.GameScale)
		dx = 0
		dy = barHeight + spacing
	default: // BarPlacementBottom
		slot := (fieldWidth - 3*barWidth) / 6
		x = slot
		y = fieldHeight - int(20*gs.GameScale) - barHeight
		dx = barWidth + 2*slot
		dy = 0
	}

	screenW := screen.Bounds().Dx()
	screenH := screen.Bounds().Dy()
	minX := -ox
	minY := -oy
	maxX := screenW - ox - barWidth - 2*dx
	maxY := screenH - oy - barHeight - 2*dy
	if x < minX {
		x = minX
	} else if x > maxX {
		x = maxX
	}
	if y < minY {
		y = minY
	} else if y > maxY {
		y = maxY
	}

	drawBar := func(x, y int, cur, max int, clr color.RGBA) {
		alpha := uint8(255)
		frameClr := color.RGBA{0xff, 0xff, 0xff, alpha}
		pad := int(gs.GameScale)
		drawRect(x-pad, y-pad, barWidth+2*pad, pad, frameClr)
		drawRect(x-pad, y+barHeight, barWidth+2*pad, pad, frameClr)
		drawRect(x-pad, y, pad, barHeight, frameClr)
		drawRect(x+barWidth, y, pad, barHeight, frameClr)

		if max < cur {
			max = cur
		}

		total := 255
		if cur > total {
			cur = total
		}
		if max > total {
			max = total
		}

		wCur := barWidth * cur / total
		wMax := barWidth * max / total

		if wCur > 0 {
			base := clr
			if gs.BarColorByValue && max > 0 {
				ratio := float64(cur) / float64(max)
				switch {
				case ratio <= 0.33:
					base = color.RGBA{0xff, 0x00, 0x00, 0xff}
				case ratio <= 0.66:
					base = color.RGBA{0xff, 0xff, 0x00, 0xff}
				default:
					base = color.RGBA{0x00, 0xff, 0x00, 0xff}
				}
			}
			fillClr := color.RGBA{base.R, base.G, base.B, alpha}
			drawRect(x, y, wCur, barHeight, fillClr)
		}

		if wMax > wCur {
			greyClr := color.RGBA{0x80, 0x80, 0x80, alpha}
			drawRect(x+wCur, y, wMax-wCur, barHeight, greyClr)
		}

		if wMax < barWidth {
			yellowClr := color.RGBA{0x80, 0x80, 0x00, alpha}
			drawRect(x+wMax, y, barWidth-wMax, barHeight, yellowClr)
		}
	}

	hp := lerpBar(snap.prevHP, snap.hp, alpha)
	hpMax := lerpBar(snap.prevHPMax, snap.hpMax, alpha)
	drawBar(x, y, hp, hpMax, color.RGBA{0x00, 0xff, 0, 0xff})
	x += dx
	y += dy
	bal := lerpBar(snap.prevBalance, snap.balance, alpha)
	balMax := lerpBar(snap.prevBalanceMax, snap.balanceMax, alpha)
	drawBar(x, y, bal, balMax, color.RGBA{0x00, 0x00, 0xff, 0xff})
	x += dx
	y += dy
	sp := lerpBar(snap.prevSP, snap.sp, alpha)
	spMax := lerpBar(snap.prevSPMax, snap.spMax, alpha)
	drawBar(x, y, sp, spMax, color.RGBA{0xff, 0x00, 0x00, 0xff})
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

// drawInputOverlay renders the text entry box when chatting.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	scaledW, scaledH := eui.Layout(outsideWidth, outsideHeight)

	if uiReady && !windowsRestored {
		restoreWindowsAfterScale()
	}

	if outsideWidth > 512 && outsideHeight > 384 {
		if gs.WindowWidth != outsideWidth || gs.WindowHeight != outsideHeight {
			gs.WindowWidth = outsideWidth
			gs.WindowHeight = outsideHeight
			settingsDirty = true
		}
	}

	return scaledW, scaledH
}

func runGame(ctx context.Context) {
	gameCtx = ctx

	ebiten.SetScreenClearedEveryFrame(false)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// Ensure Update() TPS is synced with Draw FPS from the start.
	ebiten.SetTPS(ebiten.SyncWithFPS)
	w, h := ebiten.Monitor().Size()
	if w == 0 || h == 0 {
		w, h = initialWindowW, initialWindowH
	}
	if gameWin != nil {
		gameWin.SetSize(eui.Point{X: float32(w), Y: float32(h)})
	}
	if gs.Fullscreen {
		ebiten.SetFullscreen(true)
	}
	ebiten.SetWindowFloating(gs.Fullscreen || gs.AlwaysOnTop)

	op := &ebiten.RunGameOptions{ScreenTransparent: false}
	if err := ebiten.RunGameWithOptions(&Game{}, op); err != nil {
		log.Printf("ebiten: %v", err)
	}
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
	if gs.Style != "" {
		eui.LoadStyle(gs.Style)
	}
	initUI()
	updateDimmedScreenBG()
	updateCharacterButtons()

	close(gameStarted)
	go loadSpellcheck()
	go loadScripts()
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
	if !settingsLoaded {
		gameWin.SetZone(eui.HZoneCenter, eui.VZoneTop)
	}
	gameWin.Size = eui.Point{X: 8000, Y: 8000}
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
		layoutNotifications()
	}
	updateGameWindowSize()
	updateGameImageSize()
	layoutNotifications()
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
	layoutNotifications()
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

// altNetDelay calculates the current artificial network delay.
// It stays at 0 for the first two frames after login and then ramps
// linearly to the target over three seconds.
func altNetDelay(frame int, start time.Time, now time.Time, target time.Duration) (time.Duration, time.Time) {
	if frame <= 2 || target <= 0 {
		return 0, start
	}
	if start.IsZero() {
		start = now
	}
	elapsed := now.Sub(start)
	if elapsed >= 3*time.Second {
		return target, start
	}
	return time.Duration(float64(target) * elapsed.Seconds() / 3.0), start
}

func sendInputLoop(ctx context.Context, udpConn, tcpConn net.Conn) {
	// nextReliable determines when to send the next keep-alive packet via
	// the reliable channel to preserve NAT mappings.
	var nextReliable time.Time
	var rampStart time.Time
	var frameCount int
	for {
		select {
		case <-ctx.Done():
			return
		case <-frameCh:
		}
		frameCount++
		if gs.altNetMode {
			delay, rs := altNetDelay(frameCount, rampStart, time.Now(), time.Duration(gs.altNetDelay)*time.Millisecond)
			rampStart = rs
			if delay > 0 {
				time.Sleep(delay)
			}
		}
		frameMu.Lock()
		last := lastFrameTime
		frameMu.Unlock()
		if time.Since(last) > 2*time.Second || udpConn == nil {
			continue
		}
		inputMu.Lock()
		var s inputState
		if len(inputQueue) > 0 {
			s = inputQueue[0]
			latestInput = s
			inputQueue = inputQueue[1:]
			if keyStopFrames > 0 && len(inputQueue) == 0 && !s.mouseDown {
				s = inputState{mouseX: 0, mouseY: 0, mouseDown: true}
				keyStopFrames--
			}
		} else {
			s = latestInput
			if keyStopFrames > 0 {
				s = inputState{mouseX: 0, mouseY: 0, mouseDown: true}
				keyStopFrames--
			}
		}
		inputMu.Unlock()

		reliable := false
		now := time.Now()
		if now.After(nextReliable) && pendingCommand == "" && tcpConn != nil {
			reliable = true
			// next packet will be 3 to 5 minutes from now
			nextReliable = now.Add(3*time.Minute + time.Duration(rand.Intn(120))*time.Second)
		}

		var err error
		if reliable {
			err = sendPlayerInput(tcpConn, s.mouseX, s.mouseY, s.mouseDown, true)
		} else {
			err = sendPlayerInput(udpConn, s.mouseX, s.mouseY, s.mouseDown, false)
		}
		if err != nil {
			// ignore errors from dead connections
		}
	}
}

func udpReadLoop(ctx context.Context, conn net.Conn) {
	for {
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
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
			handleDisconnect()
			return
		}
		tag := binary.BigEndian.Uint16(m[:2])
		flags := frameFlags(m)
		// Arm-to-record behavior: collect pre-login blocks and start on first draw.
		if recorder == nil && recordingMovie {
			if tag == 2 {
				startRecording()
				recordingMovie = false
			} else {
				if flags&flagGameState != 0 {
					payload := append([]byte(nil), m[2:]...)
					parseGameState(payload, uint16(clVersion), uint16(movieRevision))
					loginGameState = payload
				}
				if flags&flagMobileData != 0 {
					payload := append([]byte(nil), m[2:]...)
					parseMobileTable(payload, 0, uint16(clVersion), uint16(movieRevision))
					loginMobileData = payload
				}
				if flags&flagPictureTable != 0 {
					payload := append([]byte(nil), m[2:]...)
					loginPictureTable = payload
				}
			}
		}
		if recorder != nil {
			if !wroteLoginBlocks {
				if tag == 2 { // first draw state
					if len(loginGameState) > 0 {
						l := len(loginGameState)
						recorder.AddBlock(gameStateBlock(0, 0, 0, l, l, l, loginGameState), flagGameState)
					}
					if len(loginMobileData) > 0 {
						if err := recorder.WriteBlock(loginMobileData, flagMobileData); err != nil {
							logError("record block: %v", err)
						}
					}
					if len(loginPictureTable) > 0 {
						if err := recorder.WriteBlock(loginPictureTable, flagPictureTable); err != nil {
							logError("record block: %v", err)
						}
					}
					wroteLoginBlocks = true
					if err := recorder.WriteFrame(m, flags); err != nil {
						logError("record frame: %v", err)
					}
				} else {
					if flags&flagGameState != 0 {
						payload := append([]byte(nil), m[2:]...)
						parseGameState(payload, uint16(clVersion), uint16(movieRevision))
						loginGameState = payload
					}
					if flags&flagMobileData != 0 {
						payload := append([]byte(nil), m[2:]...)
						parseMobileTable(payload, 0, uint16(clVersion), uint16(movieRevision))
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
				netJitter = 0
			} else {
				diff := rtt - netLatency
				if diff < 0 {
					diff = -diff
				}
				netJitter = (netJitter*7 + diff) / 8
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
			handleDisconnect()
			break
		}
		tag := binary.BigEndian.Uint16(m[:2])
		flags := frameFlags(m)
		if recorder == nil && recordingMovie {
			if tag == 2 {
				startRecording()
				recordingMovie = false
			} else {
				if flags&flagGameState != 0 {
					payload := append([]byte(nil), m[2:]...)
					parseGameState(payload, uint16(clVersion), uint16(movieRevision))
					loginGameState = payload
				}
				if flags&flagMobileData != 0 {
					payload := append([]byte(nil), m[2:]...)
					parseMobileTable(payload, 0, uint16(clVersion), uint16(movieRevision))
					loginMobileData = payload
				}
				if flags&flagPictureTable != 0 {
					payload := append([]byte(nil), m[2:]...)
					loginPictureTable = payload
				}
			}
		}
		if recorder != nil {
			if !wroteLoginBlocks {
				if tag == 2 { // first draw state
					if len(loginGameState) > 0 {
						l := len(loginGameState)
						recorder.AddBlock(gameStateBlock(0, 0, 0, l, l, l, loginGameState), flagGameState)
					}
					if len(loginMobileData) > 0 {
						if err := recorder.WriteBlock(loginMobileData, flagMobileData); err != nil {
							logError("record block: %v", err)
						}
					}
					if len(loginPictureTable) > 0 {
						if err := recorder.WriteBlock(loginPictureTable, flagPictureTable); err != nil {
							logError("record block: %v", err)
						}
					}
					wroteLoginBlocks = true
					if err := recorder.WriteFrame(m, flags); err != nil {
						logError("record frame: %v", err)
					}
				} else {
					if flags&flagGameState != 0 {
						payload := append([]byte(nil), m[2:]...)
						parseGameState(payload, uint16(clVersion), uint16(movieRevision))
						loginGameState = payload
					}
					if flags&flagMobileData != 0 {
						payload := append([]byte(nil), m[2:]...)
						parseMobileTable(payload, 0, uint16(clVersion), uint16(movieRevision))
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
	// Inspect the 2-byte message tag; only non-draw-state (tag != 2) messages
	// contribute pre-frame block flags. For draw-state frames, the movie file
	// flags should only reflect blocks we explicitly attach via AddBlock/WriteBlock.
	var tag uint16
	if len(m) >= 2 {
		tag = binary.BigEndian.Uint16(m[:2])
		m = m[2:]
	} else {
		m = nil
	}
	if tag != 2 {
		switch {
		case looksLikeGameState(m):
			flags |= flagGameState
		case looksLikeMobileData(m):
			flags |= flagMobileData
		case looksLikePictureTable(m):
			flags |= flagPictureTable
		}
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
