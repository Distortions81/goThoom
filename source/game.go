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
	"runtime"
	"strings"
	"sync"
	"time"

	"gothoom/climg"
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
const initialWindowW, initialWindowH = 1920, 1080

const (
	nameTagHoverHold = 750 * time.Millisecond
	nameTagHoverFade = 1250 * time.Millisecond
)

type nameTagHoverReveal struct {
	name        string
	lastHovered time.Time
}

var (
	nameTagHoverRevealMu sync.Mutex
	nameTagHoverReveals  = make(map[uint8]nameTagHoverReveal)
)

var uiMouseDown bool

func playfieldBackgroundColor() color.RGBA {
	shade := uint8(0x88)
	if clImages != nil {
		shade = clImages.GammaCorrectChannel(shade)
	}
	return color.RGBA{R: shade, G: shade, B: shade, A: 0xff}
}

func fillOutsideWorldView(target *ebiten.Image, worldView image.Rectangle, fill color.Color) {
	if target == nil {
		return
	}
	bounds := target.Bounds()
	worldView = worldView.Intersect(bounds)
	if worldView.Empty() {
		target.Fill(fill)
		return
	}
	regions := [...]image.Rectangle{
		image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, worldView.Min.Y),
		image.Rect(bounds.Min.X, worldView.Max.Y, bounds.Max.X, bounds.Max.Y),
		image.Rect(bounds.Min.X, worldView.Min.Y, worldView.Min.X, worldView.Max.Y),
		image.Rect(worldView.Max.X, worldView.Min.Y, bounds.Max.X, worldView.Max.Y),
	}
	for _, region := range regions {
		if !region.Empty() {
			target.SubImage(region).(*ebiten.Image).Fill(fill)
		}
	}
}

func clearNameTagHoverReveals() {
	nameTagHoverRevealMu.Lock()
	clearMap(nameTagHoverReveals)
	nameTagHoverRevealMu.Unlock()
}

func nameTagHoverAlpha(index uint8, name string, hovered bool, now time.Time) float32 {
	nameTagHoverRevealMu.Lock()
	defer nameTagHoverRevealMu.Unlock()
	if hovered {
		nameTagHoverReveals[index] = nameTagHoverReveal{name: name, lastHovered: now}
		return 1
	}
	reveal, ok := nameTagHoverReveals[index]
	if !ok || reveal.name != name {
		return 0
	}
	elapsed := now.Sub(reveal.lastHovered)
	if elapsed <= nameTagHoverHold {
		return 1
	}
	if elapsed >= nameTagHoverHold+nameTagHoverFade {
		delete(nameTagHoverReveals, index)
		return 0
	}
	progress := float32(elapsed-nameTagHoverHold) / float32(nameTagHoverFade)
	return 1 - ease(progress)
}

// worldViewRect tracks the region of gameImage occupied by the rendered world.
var worldViewRect image.Rectangle

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

// updateGameImageSize ensures the game image item exists and matches the
// current inner content size of the game window.
func updateGameImageSize() {
	if gameWin == nil {
		return
	}
	size := gameWin.GetSize()
	pad := float64(2 * gameWin.Padding)
	title := float64(gameWin.GetTitleSize())
	pixelW := int(size.X) &^ 1
	pixelH := int(size.Y) &^ 1
	edgeInset := 2
	if gs.TiledWindows {
		// A tiled playfield has no standalone window frame, so its image should
		// occupy every assigned pixel. Rounding also avoids dropping the final
		// row or column when a normalized tile lands on an odd pixel boundary.
		pixelW = int(math.Round(float64(size.X)))
		pixelH = int(math.Round(float64(size.Y)))
		edgeInset = 0
	}
	// Inner content size (exclude titlebar and inside padding)
	cw := int(float64(pixelW) - pad)
	ch := int(float64(pixelH) - pad - title)
	w := cw - 2*edgeInset
	h := ch - 2*edgeInset
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
		gameImageItem.Position = eui.Point{X: float32(edgeInset) / s, Y: float32(edgeInset) / s}
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
		_, replacement := eui.NewImageFastItem(w, h)
		if gameImageBacking != nil {
			gameImageBacking.Deallocate()
		}
		gameImageBacking = replacement
		if gameWin != nil {
			gameWin.Dirty = true
		}
	}
	gameImage = gameImageBacking.SubImage(image.Rect(0, 0, w, h)).(*ebiten.Image)
	gameImageItem.Image = gameImage
	// Always update the item size/position even if we reuse a larger backing image.
	gameImageItem.Size = eui.Point{X: float32(w) / s, Y: float32(h) / s}
	gameImageItem.Position = eui.Point{X: float32(edgeInset) / s, Y: float32(edgeInset) / s}
}

func worldArtworkFilter() ebiten.Filter {
	// With the artwork scaler off, preserve hard pixel edges rather than
	// falling back to linear filtering. The scaler's other modes provide their
	// own reconstructed image, where Pixel Art Scaling still controls the final
	// sampling choice.
	if !artworkUpscaleEnabled() || gs.PixelArtScaling {
		return ebiten.FilterNearest
	}
	return ebiten.FilterLinear
}

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
)

// gameWin represents the main playfield window. Its size corresponds to the
// classic client field box (547×540) defined in old_mac_client/client/source/
// GameWin_cl.cp and Public_cl.h (Layout.layoFieldBox).
var gameWin *eui.WindowData
var gameWindowFreeformTitleHeight float32
var gameWindowFreeformPadding float32
var gameWindowFreeformMargin float32
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
	descriptors   map[uint8]frameDescriptor
	pictures      []framePicture
	prevPictures  []framePicture
	picShiftX     int
	picShiftY     int
	mobiles       map[uint8]frameMobile
	prevMobiles   map[uint8]frameMobile
	mobileScratch map[uint8]frameMobile
	prevDescs     map[uint8]frameDescriptor
	prevTime      time.Time
	curTime       time.Time

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
	logicalFrame                int

	// Prepared render caches populated only when a new game state arrives.
	// These avoid per-frame sorting and partitioning work in Draw.
	sortedPics []framePicture
	picsNeg    []framePicture
	picsZero   []framePicture
	picsPos    []framePicture
	liveMobs   []frameMobile
	deadMobs   []frameMobile
	nameMobs   []frameMobile
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
	markWorldStateChanged()
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
	// used by picture-shift and interpolation processing.
	state.sortedPics = append(state.sortedPics[:0], state.pictures...)
	sortPictures(state.sortedPics)
	state.picsNeg = state.picsNeg[:0]
	state.picsZero = state.picsZero[:0]
	state.picsPos = state.picsPos[:0]
	for _, p := range state.sortedPics {
		switch {
		case p.Plane < 0:
			state.picsNeg = append(state.picsNeg, p)
		case p.Plane == 0:
			state.picsZero = append(state.picsZero, p)
		default:
			state.picsPos = append(state.picsPos, p)
		}
	}
	markWorldStateChanged()
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
	worldGeneration             uint64
	descriptors                 map[uint8]frameDescriptor
	prevPicturePositions        map[picturePositionKey]struct{}
	prevPictureIndexGeneration  uint64
	prevPictureIndexValid       bool
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
	logicalFrame                int

	// Precomputed, sorted/partitioned data for rendering
	picsNeg  []framePicture
	picsZero []framePicture
	picsPos  []framePicture
	liveMobs []frameMobile
	deadMobs []frameMobile
}

// captureDrawSnapshot copies the shared draw state into caller-owned storage.
// Draw calls this serially with the same destination so maps and slices can be
// reused without sharing mutable storage with the network goroutine.
func captureDrawSnapshot(snap *drawSnapshot) {
	stateMu.Lock()
	defer stateMu.Unlock()

	if snap.descriptors == nil {
		snap.descriptors = make(map[uint8]frameDescriptor, len(state.descriptors))
	} else {
		clear(snap.descriptors)
	}
	generation := worldStateGeneration.Load()
	snap.worldGeneration = generation
	if gs.ObjectPinning && gs.MotionSmoothing {
		if !snap.prevPictureIndexValid || snap.prevPictureIndexGeneration != generation {
			if snap.prevPicturePositions == nil {
				snap.prevPicturePositions = make(map[picturePositionKey]struct{}, len(state.prevPictures))
			} else {
				clear(snap.prevPicturePositions)
			}
			for _, p := range state.prevPictures {
				snap.prevPicturePositions[picturePositionKey{pictID: p.PictID, h: p.H, v: p.V}] = struct{}{}
			}
			snap.prevPictureIndexGeneration = generation
			snap.prevPictureIndexValid = true
		}
	} else {
		snap.prevPictureIndexValid = false
	}
	snap.picShiftX = state.picShiftX
	snap.picShiftY = state.picShiftY
	snap.mobiles = append(snap.mobiles[:0], state.nameMobs...)
	snap.prevTime = state.prevTime
	snap.curTime = state.curTime
	snap.hp = state.hp
	snap.hpMax = state.hpMax
	snap.sp = state.sp
	snap.spMax = state.spMax
	snap.balance = state.balance
	snap.balanceMax = state.balanceMax
	snap.prevHP = state.prevHP
	snap.prevHPMax = state.prevHPMax
	snap.prevSP = state.prevSP
	snap.prevSPMax = state.prevSPMax
	snap.prevBalance = state.prevBalance
	snap.prevBalanceMax = state.prevBalanceMax
	snap.ackCmd = state.ackCmd
	snap.lightingFlags = state.lightingFlags
	snap.dropped = state.dropped
	snap.logicalFrame = state.logicalFrame
	snap.picsNeg = append(snap.picsNeg[:0], state.picsNeg...)
	snap.picsZero = append(snap.picsZero[:0], state.picsZero...)
	snap.picsPos = append(snap.picsPos[:0], state.picsPos...)
	snap.liveMobs = append(snap.liveMobs[:0], state.liveMobs...)
	snap.deadMobs = append(snap.deadMobs[:0], state.deadMobs...)
	snap.bubbles = snap.bubbles[:0]

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
		var last [256]int
		for i, b := range kept {
			last[b.Index] = i + 1
		}
		dedup := kept[:0]
		for i, b := range kept {
			if last[b.Index] == i+1 {
				dedup = append(dedup, b)
			}
		}
		state.bubbles = dedup
		snap.bubbles = append(snap.bubbles, state.bubbles...)
	}
	if gs.MotionSmoothing {
		if snap.prevMobiles == nil {
			snap.prevMobiles = make(map[uint8]frameMobile, len(state.prevMobiles))
		} else {
			clear(snap.prevMobiles)
		}
		for idx, m := range state.prevMobiles {
			snap.prevMobiles[idx] = m
		}
	} else if snap.prevMobiles != nil {
		clear(snap.prevMobiles)
	}
	if mobileFrameBlendingEnabled() {
		if snap.prevDescs == nil {
			snap.prevDescs = make(map[uint8]frameDescriptor, len(state.prevDescs))
		} else {
			clear(snap.prevDescs)
		}
		for idx, d := range state.prevDescs {
			snap.prevDescs[idx] = d
		}
	} else if snap.prevDescs != nil {
		clear(snap.prevDescs)
	}
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
		logicalFrame:   src.logicalFrame,
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

func mobileFrameBlendingEnabled() bool {
	return gs.ShadersEnabled && gs.MotionSmoothing && gs.BlendMobiles
}

func pictureFrameBlendingEnabled() bool {
	return gs.ShadersEnabled && gs.MotionSmoothing && gs.BlendPicts
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
	if gs.MotionSmoothing && !curTime.IsZero() && curTime.After(prevTime) {
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
		if mobileFrameBlendingEnabled() {
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
		if pictureFrameBlendingEnabled() {
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

type Game struct {
	drawSnapshot drawSnapshot
}

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
	if updateStartupLoading() {
		return nil
	}
	drainMainThreadDispatcher()
	if assetDumpMode() {
		assetDumpOnce.Do(exportAssets)
		return nil
	}
	drainScriptDispatcher()
	processMusicRequests()

	if classicSplashFilterPending && gs.ShowClanLordSplashImage {
		prepareClassicSplash()
	}

	if inputFlow != nil && len(inputFlow.Contents) > 0 {
		eui.ClearFocus(inputFlow.Contents[0])
		inputFlow.Contents[0].Focused = false
	}
	legacyMacroBeginInputFrame()
	keyboardTestBeginInputFrame()
	eui.SetKeyboardInputCaptured(keyboardTestFrameActive)
	eui.Update() //We really need this to return eaten clicks
	ensureToolbarAccessible()
	updateSetupWizardGraphicsDetection()
	// Advance script tick waiters once per frame
	scriptAdvanceTick()
	pollScriptChangeEvents()
	advanceLegacyMacros(int64(ackFrame))
	if legacyMacroLibraryWin != nil && legacyMacroLibraryWin.IsOpen() {
		legacyMacroLibraryRefreshErrorsButton()
	}
	typingElsewhere := typingInUI()
	if inputActive && inputFlow != nil && len(inputFlow.Contents) > 0 {
		item := inputFlow.Contents[0]
		inputPos = plainCursorPos(item.Text, item.CursorPos)
		plain := strings.ReplaceAll(item.Text, "\n", "")
		inputText = []rune(plain)
	}
	updateKeyboardTest()
	legacyMacroPollKeyboard(int64(ackFrame), typingElsewhere)
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
	updateHotkeyRecording()
	consumedScriptInput := checkHotkeys()

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

	if !keyboardTestSuppressingInput() && (inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) || joyClick2) &&
		!scriptInputConsumesButton(consumedScriptInput, "RightClick") {
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
		updateToolbarHands()
		inventoryDirty = false
	}

	if !nextRecentPlayersExpiry.IsZero() && !now.Before(nextRecentPlayersExpiry) {
		playersDirty = true
		nextRecentPlayersExpiry = time.Time{}
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
		ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
		if newChars := ebiten.AppendInputChars(nil); len(newChars) > 0 &&
			!legacyMacroSuppressesTypedInput() && !scriptInputConsumesText(consumedScriptInput) {
			if inputPos < 0 {
				inputPos = 0
			}
			if inputPos > len(inputText) {
				inputPos = len(inputText)
			}
			for _, char := range newChars {
				if !ctrl && legacyMacroReplacementBoundary(char) {
					if updated, pos, handled := legacyMacroTriggerReplacement(string(inputText), inputPos); handled {
						inputText = []rune(updated)
						inputPos = pos
						changedInput = true
						textChanged = true
					}
				}
				inputText = append(inputText, 0)
				copy(inputText[inputPos+1:], inputText[inputPos:])
				inputText[inputPos] = char
				inputPos++
				changedInput = true
				textChanged = true
			}
		}
		if ctrl && !legacyMacroKeyConsumed(ebiten.KeyV) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyV) && inpututil.IsKeyJustPressed(ebiten.KeyV) {
			if txt, err := clipboard.Read(context.Background(), clipboard.FmtText); err == nil && len(txt) > 0 {
				runes := []rune(string(txt))
				inputText = append(inputText[:inputPos], append(runes, inputText[inputPos:]...)...)
				inputPos += len(runes)
				changedInput = true
				textChanged = true
			}
		}
		if ctrl && !legacyMacroKeyConsumed(ebiten.KeyC) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyC) && inpututil.IsKeyJustPressed(ebiten.KeyC) {
			_, _ = clipboard.Write(context.Background(), clipboard.FmtText, []byte(string(inputText)))
		}
		if !legacyMacroKeyConsumed(ebiten.KeyArrowLeft) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyArrowLeft) && inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			if inputPos > 0 {
				inputPos--
				changedInput = true
			}
		}
		if !legacyMacroKeyConsumed(ebiten.KeyArrowRight) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyArrowRight) && inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			if inputPos < len(inputText) {
				inputPos++
				changedInput = true
			}
		}
		if !legacyMacroKeyConsumed(ebiten.KeyArrowUp) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyArrowUp) && inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
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
		if !legacyMacroKeyConsumed(ebiten.KeyArrowDown) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyArrowDown) && inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
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
		if len(inputText) > 0 && !legacyMacroKeyConsumed(ebiten.KeyBackspace) &&
			!scriptInputConsumesKey(consumedScriptInput, ebiten.KeyBackspace) && now.Sub(lastBackpace) > time.Millisecond*keyRepeatRate {
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
		if !legacyMacroKeyConsumed(ebiten.KeyEnter) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyEnter) && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if !ctrl {
				if updated, pos, handled := legacyMacroTriggerReplacement(string(inputText), inputPos); handled {
					inputText = []rune(updated)
					inputPos = pos
					changedInput = true
					textChanged = true
				}
			}
			orig := string(inputText)
			if legacyMacroHasExpression(orig) {
				if entry := strings.TrimSpace(orig); entry != "" {
					inputHistory = append(inputHistory, entry)
				}
				if gs.InputBarAlwaysOpen {
					inputActive = true
				} else {
					inputActive = false
				}
				inputText = inputText[:0]
				inputPos = 0
				historyPos = len(inputHistory)
				legacyMacroTriggerExpression(orig, int64(ackFrame))
			} else {
				txt := expandShortcut(orig)
				txt = strings.TrimSpace(txt)
				if txt == "" {
					// If handlers removed the text, fall back to the user's
					// original entry so it's still sent.
					txt = strings.TrimSpace(orig)
				}
				if txt != "" {
					if !dispatchLocalCommand(txt) {
						enqueueCommand(txt)
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
			}
			changedInput = true
			textChanged = true
		}
		if !legacyMacroKeyConsumed(ebiten.KeyEscape) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyEscape) && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			inputActive = false
			inputText = inputText[:0]
			inputPos = 0
			historyPos = len(inputHistory)
			changedInput = true
			textChanged = true
		}
	} else if !typingElsewhere {
		if !legacyMacroKeyConsumed(ebiten.KeyEnter) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyEnter) && inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
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
		if (ebiten.IsKeyPressed(ebiten.KeyArrowLeft) && !legacyMacroKeyConsumed(ebiten.KeyArrowLeft) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyArrowLeft)) ||
			(ebiten.IsKeyPressed(ebiten.KeyA) && !legacyMacroKeyConsumed(ebiten.KeyA) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyA)) {
			dx--
		}
		if (ebiten.IsKeyPressed(ebiten.KeyArrowRight) && !legacyMacroKeyConsumed(ebiten.KeyArrowRight) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyArrowRight)) ||
			(ebiten.IsKeyPressed(ebiten.KeyD) && !legacyMacroKeyConsumed(ebiten.KeyD) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyD)) {
			dx++
		}
		if (ebiten.IsKeyPressed(ebiten.KeyArrowUp) && !legacyMacroKeyConsumed(ebiten.KeyArrowUp) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyArrowUp)) ||
			(ebiten.IsKeyPressed(ebiten.KeyW) && !legacyMacroKeyConsumed(ebiten.KeyW) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyW)) {
			dy--
		}
		if (ebiten.IsKeyPressed(ebiten.KeyArrowDown) && !legacyMacroKeyConsumed(ebiten.KeyArrowDown) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyArrowDown)) ||
			(ebiten.IsKeyPressed(ebiten.KeyS) && !legacyMacroKeyConsumed(ebiten.KeyS) && !scriptInputConsumesKey(consumedScriptInput, ebiten.KeyS)) {
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
	if focused && inGame && !typingElsewhere && !pointInUI(mx, my) && !keyboardTestSuppressingInput() {
		wheelX, wheelY := ebiten.Wheel()
		wheelName, wheelModifiers := legacyMacroWheelInput(wheelX, wheelY, legacyMacroCurrentModifiers(false))
		if wheelName != "" && !scriptInputConsumesButton(consumedScriptInput, scriptWheelButtonName(wheelX, wheelY)) {
			if started, allowDefault := legacyMacroTriggerWheel(wheelName, wheelModifiers, int64(ackFrame)); started && !allowDefault {
				legacyMacroMarkInputConsumed(wheelName)
			}
		}
	}
	// Map mouse to world coordinates accounting for current draw scale/offset.
	baseX := int16(float64(mx-worldOriginX)/worldScale - float64(fieldCenterX))
	baseY := int16(float64(my-worldOriginY)/worldScale - float64(fieldCenterY))
	heldTime := inpututil.MouseButtonPressDuration(ebiten.MouseButtonLeft)
	mouseClick := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !scriptInputConsumesButton(consumedScriptInput, "LeftClick")
	click := mouseClick || joyClick1
	rightClick := (inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) || joyClick2) && !scriptInputConsumesButton(consumedScriptInput, "RightClick")
	middleClick := (inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) || joyClick3) && !scriptInputConsumesButton(consumedScriptInput, "MiddleClick")

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
	if keyboardTestSuppressingInput() {
		click = false
		rightClick = false
		middleClick = false
		heldTime = 0
	}

	stopWalkIfOutside(click, inGame)
	inputMu.Lock()
	prev := latestInput
	inputMu.Unlock()
	uiOwnsClick := pointInUI(mx, my)
	if mouseClick {
		uiOwnsClick = uiOwnsPointerPress(uiOwnsClick, eui.PointerPressWindow(), eui.PointerPressHandled())
	}
	if click && uiOwnsClick {
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
		info := handleWorldClick(baseX, baseY, ebiten.MouseButtonLeft)
		event := legacyMacroWorldClickEvent(info, 1, legacyMacroMouseChord(1))
		if started, allowDefault := legacyMacroTriggerClick(event, int64(ackFrame)); started && !allowDefault {
			click = false
			heldTime = 0
			legacyMacroMarkMouseConsumed(ebiten.MouseButtonLeft, "click")
		} else if info.OnPlayer && legacyMacroHandlePlayerModifierClick(info.Mobile.Name, event.Modifiers) {
			click = false
			heldTime = 0
		}
	}
	if rightClick && inGame && !pointInUI(mx, my) {
		info := handleWorldClick(baseX, baseY, ebiten.MouseButtonRight)
		event := legacyMacroWorldClickEvent(info, 2, legacyMacroMouseChord(2))
		if started, allowDefault := legacyMacroTriggerClick(event, int64(ackFrame)); started && !allowDefault {
			rightClick = false
			legacyMacroMarkMouseConsumed(ebiten.MouseButtonRight, "click2")
		} else if info.OnPlayer && legacyMacroHandlePlayerModifierClick(info.Mobile.Name, event.Modifiers) {
			rightClick = false
		}
	}
	if middleClick && inGame && !pointInUI(mx, my) {
		info := handleWorldClick(baseX, baseY, ebiten.MouseButtonMiddle)
		event := legacyMacroWorldClickEvent(info, 3, legacyMacroMouseChord(3))
		if started, allowDefault := legacyMacroTriggerClick(event, int64(ackFrame)); started && !allowDefault {
			middleClick = false
			legacyMacroMarkMouseConsumed(ebiten.MouseButtonMiddle, "click3")
		} else if info.OnPlayer && legacyMacroHandlePlayerModifierClick(info.Mobile.Name, event.Modifiers) {
			middleClick = false
		}
	}
	for _, extra := range []struct {
		button ebiten.MouseButton
		name   string
		number int
	}{
		{button: ebiten.MouseButton3, name: "click4", number: 4},
		{button: ebiten.MouseButton4, name: "click5", number: 5},
	} {
		if keyboardTestSuppressingInput() || !focused || !inWindow || !inGame || pointInUI(mx, my) ||
			scriptInputConsumesButton(consumedScriptInput, mouseButtonName(extra.button)) || !inpututil.IsMouseButtonJustPressed(extra.button) {
			continue
		}
		info := handleWorldClick(baseX, baseY, extra.button)
		event := legacyMacroWorldClickEvent(info, extra.number, legacyMacroMouseChord(extra.number))
		if started, allowDefault := legacyMacroTriggerClick(event, int64(ackFrame)); started && !allowDefault {
			legacyMacroMarkMouseConsumed(extra.button, extra.name)
		} else if info.OnPlayer && legacyMacroHandlePlayerModifierClick(info.Mobile.Name, event.Modifiers) {
			legacyMacroMarkMouseConsumed(extra.button, extra.name)
		}
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
		} else if !legacyMacroMouseConsumed(ebiten.MouseButtonLeft) && continueHeldWalk(prev, inGame, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), heldTime, click) {
			walk = true
			walkToggled = false
		}
	}
	if !focused {
		walk = false
	}

	/* Change Cursor. EUI runs earlier in this frame, so preserve any cursor it
	selected for a divider, window edge, link, or text control. */
	if !eui.PointerCursorClaimed() {
		if walk && !keyWalk {
			ebiten.SetCursorShape(ebiten.CursorShapeCrosshair)
		} else {
			ebiten.SetCursorShape(ebiten.CursorShapeDefault)
		}
	}

	// If the pointer is outside the game window and we're not walking,
	// keep the last target so idle mouse movement doesn't jitter the server
	// input. When walking, continue tracking the live pointer position even
	// outside the window as requested.
	if !inGame && !walk {
		x, y = prev.mouseX, prev.mouseY
	}

	if !legacyMacroMovedThisFrame() {
		queueInput(inputState{mouseX: x, mouseY: y, mouseDown: walk})
	}

	// Warn about poor performance and suggest disabling shaders.
	// Suppress this while intentionally lowering FPS due to power saving
	// (background/unfocused or always-on power save).
	if tcpConn != nil && perFrameShaderEffectsEnabled() && gs.PromptDisableShaders && !shaderWarnShown {
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

	return nil
}

// dispatchLocalCommand runs commands owned by the client. It is shared by
// typed input and legacy macro output so both paths have identical behavior.
// The return value reports whether the command was consumed locally.
func dispatchLocalCommand(txt string) bool {
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
		return true
	}
	if !strings.HasPrefix(txt, "/") {
		return false
	}

	lower := strings.ToLower(txt)
	if strings.HasPrefix(lower, "/testhooks") {
		consoleMessage("> " + txt)
		arg := strings.TrimSpace(txt[len("/testhooks"):])
		testScriptHooks(arg)
		return true
	}

	parts := strings.SplitN(strings.TrimPrefix(txt, "/"), " ", 2)
	name := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}
	handler, ok := scriptCommands[name]
	if !ok || handler == nil {
		return false
	}
	owner := scriptCommandOwners[name]
	if scriptDisabled[owner] {
		return false
	}
	consoleMessage("> " + txt)
	scriptLogEvent(owner, "Command", args)
	handler(args)
	return true
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
	pixelW := int(size.X) &^ 1
	pixelH := int(size.Y) &^ 1
	edgeInset := 2
	if gs.TiledWindows {
		pixelW = int(math.Round(float64(size.X)))
		pixelH = int(math.Round(float64(size.Y)))
		edgeInset = 0
	}
	cw := int(float64(pixelW) - pad)         // content width
	ch := int(float64(pixelH) - pad - title) // content height
	bufW := cw - 2*edgeInset
	bufH := ch - 2*edgeInset
	if bufW <= 0 || bufH <= 0 {
		if gs.GameScale <= 0 {
			return gx, gy, 1.0
		}
		return gx, gy, gs.GameScale
	}

	viewRect, scale := fittedWorldView(bufW, bufH)

	originX := gx + edgeInset + viewRect.Min.X
	originY := gy + edgeInset + viewRect.Min.Y
	return originX, originY, scale
}

func fittedWorldView(bufW, bufH int) (image.Rectangle, float64) {
	if bufW < 1 || bufH < 1 {
		return image.Rectangle{}, 1
	}
	scale := math.Min(float64(bufW)/float64(gameAreaSizeX), float64(bufH)/float64(gameAreaSizeY))
	if scale <= 0 {
		scale = 1
	}
	w := max(1, roundToInt(float64(gameAreaSizeX)*scale))
	h := max(1, roundToInt(float64(gameAreaSizeY)*scale))
	left := (bufW - w) / 2
	top := (bufH - h) / 2
	return image.Rect(left, top, left+w, top+h), scale
}

func (g *Game) Draw(screen *ebiten.Image) {
	now := time.Now()
	drawFrameNow = now
	defer func() { drawFrameNow = time.Time{} }()
	defer shaderCompilationFrameDrawn()
	if shouldDrawStartupLoadingScreen() {
		drawStartupLoadingScreen(screen, startupLoadingLabel())
		return
	}
	loadToolbarHands()

	// Power-save throttling: measure draw duration and sleep remaining time
	// to achieve the requested FPS when active.
	if gs.PowerSaveAlways || (!ebiten.IsFocused() && gs.PowerSaveBackground) {
		frameStart := now
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
		eui.Draw(screen)
		return
	}

	bufW := gameImage.Bounds().Dx()
	bufH := gameImage.Bounds().Dy()
	viewRect, renderScale := fittedWorldView(bufW, bufH)
	worldViewRect = viewRect
	worldView := gameImage.SubImage(viewRect).(*ebiten.Image)

	// Render the world directly at its final game-window resolution.
	var snap drawSnapshot
	var alpha float64
	var haveSnap bool
	if !setupWizardPreviewActive && clmov == "" && !playingMovie && tcpConn == nil && pcapPath == "" && !fake {
		gameImage.Fill(playfieldBackgroundColor())
		prev := gs.GameScale
		gs.GameScale = renderScale
		drawSplash(worldView, 0, 0)
		gs.GameScale = prev
	} else {
		captureDrawSnapshot(&g.drawSnapshot)
		if setupWizardPreviewActive {
			prepareSetupWizardSceneSnapshot(&g.drawSnapshot, now)
		}
		snap = g.drawSnapshot
		var mobileFade, pictFade float32
		alpha, mobileFade, pictFade = computeInterpolation(now, snap.prevTime, snap.curTime, gs.MobileBlendAmount, gs.BlendAmount)
		if prepareSceneArtworkFrame(snap) {
			// Keep the last completed world frame visible while Ebitengine submits
			// the prepared upload batch. A small indicator communicates the pause
			// without replacing normal play with a flashing loading screen.
			noteClientActivity(clientActivityGPU)
		} else {
			prev := gs.GameScale
			gs.GameScale = renderScale
			useLighting := shaderLightingEnabled() && lightingShader != nil
			useComposite := useLighting && sceneMayNeedLighting(snap)
			sceneTarget := worldView
			if useComposite {
				fillOutsideWorldView(gameImage, viewRect, playfieldBackgroundColor())
				sceneTarget = ensureLightingTmp(worldView.Bounds())
				sceneTarget.Fill(playfieldBackgroundColor())
			} else {
				gameImage.Fill(playfieldBackgroundColor())
			}
			drawScene(sceneTarget, 0, 0, snap, alpha, mobileFade, pictFade)
			if useLighting {
				// Use shader-based night darkening with inverse-square falloff.
				addNightDarkSources(sceneTarget.Bounds(), float32(alpha))
			} else {
				// Classic overlay path when shader is off.
				//drawNightAmbient(worldView, 0, 0)
				drawNightOverlay(sceneTarget, 0, 0)
			}
			if useComposite {
				applyWorldComposite(worldView, sceneTarget, frameLights, frameDarks, float32(alpha), useLighting)
			} else if useLighting && (len(frameLights) != 0 || len(frameDarks) != 0) {
				// Conservative fallback if a newly supported light source was not
				// recognized by sceneMayNeedLighting.
				applyLightingShader(worldView, frameLights, frameDarks, float32(alpha))
			} else {
				applyDetailedCharacterShadow(worldView)
			}
			drawReplacementEffects(worldView, worldView.Bounds().Min.X, worldView.Bounds().Min.Y, snap.mobiles, snap.prevMobiles, snap.picShiftX, snap.picShiftY, alpha)
			drawStatusBars(worldView, 0, 0, snap, alpha)
			gs.GameScale = prev
			haveSnap = true
		}
	}
	if replacementEffectsPreview {
		drawReplacementEffectsPreview(worldView)
	}

	var finalScale float64
	if haveSnap {
		prev := gs.GameScale
		finalScale = renderScale
		if finalScale <= 0 {
			finalScale = worldScale
		}
		if finalScale <= 0 {
			finalScale = 1
		}
		windowScale := speechBubbleWindowScale(finalScale)
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
	drawSetupWizardFPS(gameImage)
	drawClientActivityIndicators(worldView, takeClientActivity())

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
var drawFrameNow time.Time

func drawRecPlayBadge(dst *ebiten.Image) {
	// Only show when actively recording/armed or playing back.
	showRec := recorder != nil || recordingMovie
	showPlay := !showRec && playingMovie && !setupWizardPreviewActive
	if !showRec && !showPlay {
		return
	}
	// Pulse alpha between ~0.5 and 1.0
	t := float64(drawFrameNow.UnixNano()) / 1e9
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
	origin := dst.Bounds().Min
	pad := float32(6)
	cx := float32(origin.X + 10)
	cy := float32(origin.Y + 10)
	r := float32(6)
	vector.FillCircle(dst, cx+pad, cy+pad, r, col, false)
	// Text to the right
	op := acquireTextDrawOpts()
	op.GeoM.Translate(float64(origin.X)+float64(2*pad+r*2), float64(origin.Y)+float64(4+pad))
	op.ColorScale.Scale(1, 1, 1, float32(alpha))
	text.Draw(dst, label, mainFontBold, op)
	releaseTextDrawOpts(op)
}

func prepareSceneArtworkFrame(snap drawSnapshot) bool {
	if !gs.BatchArtworkLoading {
		return false
	}
	preparedSheets := prepareSceneArtwork(snap)
	if preparedSheets <= 1 {
		return false
	}
	// Artwork preparation creates large temporary RGBA pose and upscale
	// buffers. Collect them during the deliberate loading hitch instead of
	// letting the next automatic GC stretch recovery across visible frames.
	runtime.GC()
	return tcpConn != nil && clmov == "" && !playingMovie && pcapPath == "" && !fake && !setupWizardPreviewActive
}

// drawScene renders all world objects for the current frame.
func drawScene(screen *ebiten.Image, ox, oy int, snap drawSnapshot, alpha float64, mobileFade, pictFade float32) {
	frameDetailedShadowMask = nil
	frameDetailedShadowBounds = image.Rectangle{}
	// Ebitengine subimages retain their parent-space bounds.
	ox += screen.Bounds().Min.X
	oy += screen.Bounds().Min.Y
	if shaderLightingEnabled() {
		frameLights = frameLights[:0]
		frameDarks = frameDarks[:0]
		frameLightCasters = frameLightCasters[:0]
	}
	frameContactShadowLights = frameContactShadowLights[:0]
	beginReplacementEffects()

	// Use cached descriptor map directly; no need to rebuild/sort it per frame.
	descMap := snap.descriptors
	mobileLimit := maxMobileInterpPixels * (snap.dropped + 1)

	// Use precomputed, sorted partitions
	negPics := snap.picsNeg
	zeroPics := snap.picsZero
	posPics := snap.picsPos
	live := snap.liveMobs
	dead := snap.deadMobs
	shadowAlpha, _, shadowKind := currentCharacterShadowRenderState()
	if !gs.hideMobiles && shadowKind == characterShadowContact {
		collectContactShadowLights(ox, oy, snap, alpha)
	}

	for _, p := range negPics {
		drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, descMap, snap.prevMobiles, snap.prevPicturePositions, snap.picShiftX, snap.picShiftY, snap.logicalFrame)
	}

	if gs.hideMobiles {
		for _, p := range zeroPics {
			drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, descMap, snap.prevMobiles, snap.prevPicturePositions, snap.picShiftX, snap.picShiftY, snap.logicalFrame)
		}
	} else {
		if shadowKind == characterShadowDirectional {
			drawMobileShadows(screen, ox, oy, snap.mobiles, descMap, snap.prevMobiles, snap.picShiftX, snap.picShiftY, alpha, mobileLimit)
		}
		for _, m := range dead {
			drawMobileImmediateShadow(screen, ox, oy, m, descMap, snap.prevMobiles, snap.picShiftX, snap.picShiftY, alpha, mobileLimit, shadowAlpha, shadowKind)
			drawMobile(screen, ox, oy, m, descMap, snap.prevMobiles, snap.prevDescs, snap.picShiftX, snap.picShiftY, alpha, mobileFade, mobileLimit, snap.logicalFrame)
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
					drawMobileImmediateShadow(screen, ox, oy, live[i], descMap, snap.prevMobiles, snap.picShiftX, snap.picShiftY, alpha, mobileLimit, shadowAlpha, shadowKind)
					drawMobile(screen, ox, oy, live[i], descMap, snap.prevMobiles, snap.prevDescs, snap.picShiftX, snap.picShiftY, alpha, mobileFade, mobileLimit, snap.logicalFrame)
					drawMobileNameTag(screen, snap, live[i], alpha)
				}
				i++
			} else {
				drawPicture(screen, ox, oy, zeroPics[j], alpha, pictFade, snap.mobiles, descMap, snap.prevMobiles, snap.prevPicturePositions, snap.picShiftX, snap.picShiftY, snap.logicalFrame)
				j++
			}
		}
	}

	for _, p := range posPics {
		drawPicture(screen, ox, oy, p, alpha, pictFade, snap.mobiles, descMap, snap.prevMobiles, snap.prevPicturePositions, snap.picShiftX, snap.picShiftY, snap.logicalFrame)
	}
}

type sceneArtworkRequestCache struct {
	sync.Mutex
	keys            []sheetKey
	scratch         []sheetKey
	worldGeneration uint64
	cacheGeneration uint64
	upscaleFactor   int
	upscaleMode     int
	upscaleEnabled  bool
	frameBlend      bool
	archive         *climg.CLImages
	valid           bool
}

var sceneArtworkRequests sceneArtworkRequestCache

func equalSheetKeys(a, b []sheetKey) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func prepareSceneArtwork(snap drawSnapshot) int {
	if clImages == nil {
		return 0
	}
	factor := screenCappedArtworkUpscaleFactor()
	mode := artworkUpscaleMode()
	upscale := artworkUpscaleEnabled()
	frameBlend := mobileFrameBlendingEnabled()
	cacheGeneration := artworkCacheGeneration.Load()

	sceneArtworkRequests.Lock()
	defer sceneArtworkRequests.Unlock()
	if sceneArtworkRequests.valid &&
		sceneArtworkRequests.worldGeneration == snap.worldGeneration &&
		sceneArtworkRequests.cacheGeneration == cacheGeneration &&
		sceneArtworkRequests.upscaleFactor == factor &&
		sceneArtworkRequests.upscaleMode == mode &&
		sceneArtworkRequests.upscaleEnabled == upscale &&
		sceneArtworkRequests.frameBlend == frameBlend &&
		sceneArtworkRequests.archive == clImages {
		return 0
	}

	keys := sceneArtworkRequests.scratch[:0]
	addPictures := func(pictures []framePicture) {
		for _, picture := range pictures {
			keys = append(keys, makeSheetKey(picture.PictID, nil, false))
		}
	}
	addPictures(snap.picsNeg)
	addPictures(snap.picsZero)
	addPictures(snap.picsPos)
	for _, mobile := range snap.mobiles {
		if descriptor, ok := snap.descriptors[mobile.Index]; ok {
			keys = append(keys, makeSheetKey(descriptor.PictID, playerColorsForDescriptor(descriptor), true))
		}
		if !frameBlend {
			continue
		}
		if _, ok := snap.prevMobiles[mobile.Index]; ok {
			descriptor := snap.descriptors[mobile.Index]
			if previousDescriptor, ok := snap.prevDescs[mobile.Index]; ok {
				descriptor = previousDescriptor
			}
			keys = append(keys, makeSheetKey(descriptor.PictID, playerColorsForDescriptor(descriptor), true))
		}
	}
	if sceneArtworkRequests.valid &&
		sceneArtworkRequests.cacheGeneration == cacheGeneration &&
		sceneArtworkRequests.upscaleFactor == factor &&
		sceneArtworkRequests.upscaleMode == mode &&
		sceneArtworkRequests.upscaleEnabled == upscale &&
		sceneArtworkRequests.frameBlend == frameBlend &&
		sceneArtworkRequests.archive == clImages &&
		equalSheetKeys(sceneArtworkRequests.keys, keys) {
		sceneArtworkRequests.worldGeneration = snap.worldGeneration
		sceneArtworkRequests.scratch = keys
		return 0
	}
	prepared := prepareArtworkSheets(keys)
	sceneArtworkRequests.scratch = sceneArtworkRequests.keys[:0]
	sceneArtworkRequests.keys = keys
	sceneArtworkRequests.worldGeneration = snap.worldGeneration
	sceneArtworkRequests.cacheGeneration = cacheGeneration
	sceneArtworkRequests.upscaleFactor = factor
	sceneArtworkRequests.upscaleMode = mode
	sceneArtworkRequests.upscaleEnabled = upscale
	sceneArtworkRequests.frameBlend = frameBlend
	sceneArtworkRequests.archive = clImages
	sceneArtworkRequests.valid = true
	return prepared
}

func collectContactShadowLights(ox, oy int, snap drawSnapshot, alpha float64) {
	if clImages == nil {
		return
	}
	for _, mobile := range snap.mobiles {
		desc, ok := snap.descriptors[mobile.Index]
		if !ok {
			continue
		}
		size := mobileSize(desc.PictID)
		if size <= 0 {
			continue
		}
		x, y := mobileScreenPosition(ox, oy, mobile, snap.prevMobiles, snap.picShiftX, snap.picShiftY, alpha, maxMobileInterpPixels*(snap.dropped+1))
		addMobileContactShadowLight(uint32(desc.PictID), mobile.State, mobile.Index, float64(x), float64(y), size)
	}
	collectPictures := func(pictures []framePicture) {
		for _, picture := range pictures {
			if gs.hideMoving && picture.Moving {
				continue
			}
			w, h := clImages.Size(uint32(picture.PictID))
			if frames := clImages.NumFrames(uint32(picture.PictID)); frames > 1 {
				h /= frames
			}
			x, y := pictureScreenPosition(ox, oy, picture, alpha, snap.mobiles, snap.prevMobiles, snap.prevPicturePositions, snap.picShiftX, snap.picShiftY, w, h)
			addPictureContactShadowLight(uint32(picture.PictID), float64(x), float64(y), w, h)
		}
	}
	collectPictures(snap.picsNeg)
	collectPictures(snap.picsZero)
	collectPictures(snap.picsPos)
}

func mobileScreenPosition(ox, oy int, m frameMobile, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, maxDist int) (int, int) {
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
	x := roundToInt((h+float64(fieldCenterX))*gs.GameScale) + ox
	y := roundToInt((v+float64(fieldCenterY))*gs.GameScale) + oy
	return x, y
}

// drawMobile renders a single mobile object with optional interpolation and onion skinning.
// When a mobile lacks history but the world shifts, a pseudo-previous position
// derived from picShift provides a one-frame interpolation. maxDist sets the
// maximum allowed pixel delta for interpolation.
func drawMobile(screen *ebiten.Image, ox, oy int, m frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, prevDescs map[uint8]frameDescriptor, shiftX, shiftY int, alpha float64, fade float32, maxDist, logicalFrame int) {
	x, y := mobileScreenPosition(ox, oy, m, prevMobiles, shiftX, shiftY, alpha, maxDist)
	var img *ebiten.Image
	plane := 0
	var d frameDescriptor
	var colors []byte
	var state uint8
	if desc, ok := descMap[m.Index]; ok {
		d = desc
		colors = playerColorsForDescriptor(d)
		state = m.State
		img = loadMobileFrame(d.PictID, state, colors)
		plane = d.Plane
	}
	curKey := makeMobileKey(d.PictID, state, colors)
	img = getScaledMobileFrame(curKey, img)
	var prevImg *ebiten.Image
	var prevColors []byte
	if mobileFrameBlendingEnabled() {
		if pm, ok := prevMobiles[m.Index]; ok {
			pd := descMap[m.Index]
			if d, ok := prevDescs[m.Index]; ok {
				pd = d
			}
			prevColors = playerColorsForDescriptor(pd)
			prevImg = loadMobileFrame(pd.PictID, pm.State, prevColors)
			prevKey := makeMobileKey(pd.PictID, pm.State, prevColors)
			prevImg = getScaledMobileFrame(prevKey, prevImg)
		}
	}
	if img != nil {
		size := mobileSize(d.PictID)
		if size == 0 {
			size = img.Bounds().Dx()
		}
		addMobileLightCaster(float64(x), float64(y), size, mobileSpriteMetricsFor(curKey, img))
		addMobileLightSource(uint32(d.PictID), m.State, m.Index, float64(x), float64(y), size, logicalFrame, alpha, screen.Bounds())
		blend := mobileFrameBlendingEnabled() && prevImg != nil && fade > 0 && fade < 1
		var src *ebiten.Image
		drawSize := img.Bounds().Dx()
		if blend {
			drawSize = max(prevImg.Bounds().Dx(), img.Bounds().Dx())
		} else if mobileFrameBlendingEnabled() && prevImg != nil {
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
		tx := float64(x) - scaled/2
		ty := float64(y) - scaled/2
		drawn := false
		if blend {
			drawn = drawFrameBlend(screen, prevImg, img, frameBlendDrawOptions{
				Left: tx, Top: ty, ScaleX: scale, ScaleY: scale, Fade: fade,
				Red: 1, Green: 1, Blue: 1, Alpha: 1,
				Linear: worldArtworkFilter() == ebiten.FilterLinear,
			})
		}
		if !drawn {
			if src == nil {
				src = img
			}
			op := acquireDrawOpts()
			op.Filter = worldArtworkFilter()
			op.DisableMipmaps = true
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(tx, ty)
			screen.DrawImage(src, op)
			releaseDrawOpts(op)
		}
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

func pictureMobileBoundsOverlap(pictH, pictV int16, pictW, pictHeight int, mob frameMobile, mobSize int) bool {
	picL := int(pictH) - pictW/2
	picT := int(pictV) - pictHeight/2
	mobL := int(mob.H) - mobSize/2
	mobT := int(mob.V) - mobSize/2
	return picL < mobL+mobSize && mobL < picL+pictW &&
		picT < mobT+mobSize && mobT < picT+pictHeight
}

func pictureEligibleForObscuring(p framePicture) bool {
	return p.Plane >= 0 && !pictureSemiTransparent(p.PictID)
}

const obscuringBlockSize = 128

type obscuringBlockKey struct {
	x int
	y int
}

type obscuringMobileCandidate struct {
	current    frameMobile
	previous   frameMobile
	descriptor frameDescriptor
	size       int
}

type pictureObscuringScratch struct {
	candidates     []obscuringMobileCandidate
	currentBlocks  map[obscuringBlockKey][]int
	previousBlocks map[obscuringBlockKey][]int
	currentUsed    []obscuringBlockKey
	previousUsed   []obscuringBlockKey
	seen           []uint32
	visit          uint32
}

func newPictureObscuringScratch() *pictureObscuringScratch {
	return &pictureObscuringScratch{
		currentBlocks:  make(map[obscuringBlockKey][]int),
		previousBlocks: make(map[obscuringBlockKey][]int),
	}
}

func (s *pictureObscuringScratch) reset() {
	s.candidates = s.candidates[:0]
	for _, key := range s.currentUsed {
		s.currentBlocks[key] = s.currentBlocks[key][:0]
	}
	for _, key := range s.previousUsed {
		s.previousBlocks[key] = s.previousBlocks[key][:0]
	}
	s.currentUsed = s.currentUsed[:0]
	s.previousUsed = s.previousUsed[:0]
}

func (s *pictureObscuringScratch) addBlock(blocks map[obscuringBlockKey][]int, used *[]obscuringBlockKey, key obscuringBlockKey, candidateIndex int) {
	entries := blocks[key]
	if len(entries) == 0 {
		*used = append(*used, key)
	}
	blocks[key] = append(entries, candidateIndex)
}

func (s *pictureObscuringScratch) prepareSeen(count int) {
	if cap(s.seen) < count {
		s.seen = make([]uint32, count)
	} else {
		s.seen = s.seen[:count]
	}
}

func (s *pictureObscuringScratch) nextVisit() uint32 {
	s.visit++
	if s.visit == 0 {
		clear(s.seen[:cap(s.seen)])
		s.visit = 1
	}
	return s.visit
}

var pictureObscuringScratchPool = sync.Pool{
	New: func() any { return newPictureObscuringScratch() },
}

func obscuringBlockCoordinate(v int) int {
	if v < 0 {
		return -((-v + obscuringBlockSize - 1) / obscuringBlockSize)
	}
	return v / obscuringBlockSize
}

func obscuringBlockRange(h, v int16, width, height int) (minX, maxX, minY, maxY int) {
	left := int(h) - width/2
	top := int(v) - height/2
	return obscuringBlockCoordinate(left), obscuringBlockCoordinate(left + width),
		obscuringBlockCoordinate(top), obscuringBlockCoordinate(top + height)
}

func cachePictureObscuring(pictures []framePicture, mobiles []frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, logicalFrame int) {
	if clImages == nil {
		return
	}
	scratch := pictureObscuringScratchPool.Get().(*pictureObscuringScratch)
	scratch.reset()
	defer pictureObscuringScratchPool.Put(scratch)

	for _, m := range mobiles {
		d, ok := descMap[m.Index]
		if !ok {
			continue
		}
		size := mobileSize(d.PictID)
		if size <= 0 {
			continue
		}
		previous := m
		if pm, ok := prevMobiles[m.Index]; ok {
			previous = pm
		}
		candidateIndex := len(scratch.candidates)
		scratch.candidates = append(scratch.candidates, obscuringMobileCandidate{current: m, previous: previous, descriptor: d, size: size})
		minX, maxX, minY, maxY := obscuringBlockRange(m.H, m.V, size, size)
		for blockY := minY; blockY <= maxY; blockY++ {
			for blockX := minX; blockX <= maxX; blockX++ {
				key := obscuringBlockKey{blockX, blockY}
				scratch.addBlock(scratch.currentBlocks, &scratch.currentUsed, key, candidateIndex)
			}
		}
		minX, maxX, minY, maxY = obscuringBlockRange(previous.H, previous.V, size, size)
		for blockY := minY; blockY <= maxY; blockY++ {
			for blockX := minX; blockX <= maxX; blockX++ {
				key := obscuringBlockKey{blockX, blockY}
				scratch.addBlock(scratch.previousBlocks, &scratch.previousUsed, key, candidateIndex)
			}
		}
	}
	scratch.prepareSeen(len(scratch.candidates))
	for i := range pictures {
		p := &pictures[i]
		p.obscuredPrev = false
		p.obscuredNow = false
		if !pictureEligibleForObscuring(*p) {
			continue
		}
		width, height := clImages.Size(uint32(p.PictID))
		if width <= 0 || height <= 0 {
			continue
		}
		if frames := clImages.NumFrames(uint32(p.PictID)); frames > 1 {
			height /= frames
		}
		frame := clImages.FrameIndexForInstance(uint32(p.PictID), logicalFrame, pictureAnimationInstanceKey(p.H, p.V))
		previousFrame := clImages.FrameIndexForInstance(uint32(p.PictID), logicalFrame-1, pictureAnimationInstanceKey(p.PrevH, p.PrevV))
		prevMinX, prevMaxX, prevMinY, prevMaxY := obscuringBlockRange(p.PrevH, p.PrevV, width, height)
		visit := scratch.nextVisit()
		for blockY := prevMinY; blockY <= prevMaxY && !p.obscuredPrev; blockY++ {
			for blockX := prevMinX; blockX <= prevMaxX && !p.obscuredPrev; blockX++ {
				for _, candidateIndex := range scratch.previousBlocks[obscuringBlockKey{blockX, blockY}] {
					if scratch.seen[candidateIndex] == visit {
						continue
					}
					scratch.seen[candidateIndex] = visit
					c := scratch.candidates[candidateIndex]
					if pictureDrawsAfterMobileAt(*p, p.PrevH, p.PrevV, c.previous.H, c.previous.V, c.descriptor.Plane) &&
						pictureMobileBoundsOverlap(p.PrevH, p.PrevV, width, height, c.previous, c.size) {
						p.obscuredPrev = pictureObscuresMobileAt(p.PictID, previousFrame, p.PrevH, p.PrevV, c.previous, c.descriptor)
						if p.obscuredPrev {
							break
						}
					}
				}
			}
		}
		currentMinX, currentMaxX, currentMinY, currentMaxY := obscuringBlockRange(p.H, p.V, width, height)
		visit = scratch.nextVisit()
		for blockY := currentMinY; blockY <= currentMaxY && !p.obscuredNow; blockY++ {
			for blockX := currentMinX; blockX <= currentMaxX && !p.obscuredNow; blockX++ {
				for _, candidateIndex := range scratch.currentBlocks[obscuringBlockKey{blockX, blockY}] {
					if scratch.seen[candidateIndex] == visit {
						continue
					}
					scratch.seen[candidateIndex] = visit
					c := scratch.candidates[candidateIndex]
					if pictureDrawsAfterMobileAt(*p, p.H, p.V, c.current.H, c.current.V, c.descriptor.Plane) &&
						pictureMobileBoundsOverlap(p.H, p.V, width, height, c.current, c.size) {
						p.obscuredNow = pictureObscuresMobileAt(p.PictID, frame, p.H, p.V, c.current, c.descriptor)
						if p.obscuredNow {
							break
						}
					}
				}
			}
		}
	}
}

func pictureObscuringFadeAlpha(obscuredPrev, obscuredNow bool, opacity, fade float32) float32 {
	prevAlpha := float32(1)
	if obscuredPrev {
		prevAlpha = opacity
	}
	targetAlpha := float32(1)
	if obscuredNow {
		targetAlpha = opacity
	}
	return prevAlpha + (targetAlpha-prevAlpha)*fade
}

func pictureCanPinToMobile(p framePicture, width, height int) bool {
	return gs.ObjectPinning && gs.MotionSmoothing && p.Moving && !p.Background && width <= 500 && height <= 500
}

// drawPicture renders a single picture sprite.
func drawPicture(screen *ebiten.Image, ox, oy int, p framePicture, alpha float64, fade float32, mobiles []frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, prevPicturePositions map[picturePositionKey]struct{}, shiftX, shiftY, logicalFrame int) {
	if gs.hideMoving && p.Moving {
		return
	}
	plane := p.Plane

	w, h := 0, 0
	if clImages != nil {
		w, h = clImages.Size(uint32(p.PictID))
		if frames := clImages.NumFrames(uint32(p.PictID)); frames > 1 {
			h /= frames
		}
	}

	fx, fy := pictureScreenPositionFloat(ox, oy, p, alpha, mobiles, prevMobiles, prevPicturePositions, shiftX, shiftY, w, h)
	x, y := roundToInt(fx), roundToInt(fy)
	filter := worldArtworkFilter()
	left, right := filteredSpriteSpan(fx, w, gs.GameScale, filter)
	top, bottom := filteredSpriteSpan(fy, h, gs.GameScale, filter)
	lightX := (left + right) / 2
	lightY := (top + bottom) / 2
	addPictureLightSource(uint32(p.PictID), p.H, p.V, lightX, lightY, w, h, logicalFrame, alpha, screen.Bounds())
	fadeAlpha := float32(1.0)
	if gs.FadeObscuringPictures {
		fadeAlpha = pictureObscuringFadeAlpha(p.obscuredPrev, p.obscuredNow, float32(gs.ObscuringPictureOpacity), fade)
	}
	effectFrame := 0
	if clImages != nil {
		effectFrame = clImages.FrameIndexForInstance(uint32(p.PictID), logicalFrame, pictureAnimationInstanceKey(p.H, p.V))
	}
	mobileImg, mobileX, mobileY, mobileTargetSize, effectInstanceKey := replacementEffectPlayerMask(ox, oy, p, mobiles, descMap, prevMobiles, shiftX, shiftY, alpha)
	if queueReplacementPictureEffect(p.PictID, effectFrame, p.H, p.V, effectInstanceKey, left, top, right-left, bottom-top, fadeAlpha, mobileImg, mobileX, mobileY, mobileTargetSize) {
		return
	}

	frame := effectFrame
	prevFrame := 0
	if clImages != nil {
		prevInstanceKey := pictureAnimationInstanceKey(p.PrevH, p.PrevV)
		prevFrame = clImages.FrameIndexForInstance(uint32(p.PictID), logicalFrame-1, prevInstanceKey)
	}

	img := loadImageFrame(p.PictID, frame)
	img = getScaledPictureFrame(p.PictID, frame, img)
	var prevImg *ebiten.Image
	if pictureFrameBlendingEnabled() && clImages != nil {
		if prevFrame != frame {
			prevImg = loadImageFrame(p.PictID, prevFrame)
			prevImg = getScaledPictureFrame(p.PictID, prevFrame, prevImg)
		}
	}

	if img != nil {
		drawW, drawH := w, h
		blend := pictureFrameBlendingEnabled() && prevImg != nil && fade > 0 && fade < 1
		var src *ebiten.Image
		if blend {
			drawW = max(prevImg.Bounds().Dx(), img.Bounds().Dx())
			drawH = max(prevImg.Bounds().Dy(), img.Bounds().Dy())
		} else if pictureFrameBlendingEnabled() && prevImg != nil {
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
		targetW := right - left
		targetH := bottom - top
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
		red, green, blue := float32(1), float32(1), float32(1)
		if gs.pictAgainDebug && p.Again {
			red, green, blue = 0, 0, 1
		} else if src == img && gs.smoothingDebug && p.Moving {
			red, green, blue = 1, 0, 0
		}
		drawn := false
		if blend {
			drawn = drawFrameBlend(screen, prevImg, img, frameBlendDrawOptions{
				Left: left, Top: top, ScaleX: sx, ScaleY: sy, Fade: fade,
				Red: red, Green: green, Blue: blue, Alpha: fadeAlpha,
				Linear: filter == ebiten.FilterLinear,
			})
		}
		if !drawn {
			if src == nil {
				src = img
			}
			red, green, blue, drawAlpha := premultipliedDrawColor(red, green, blue, fadeAlpha)
			op := acquireDrawOpts()
			op.Filter = filter
			op.DisableMipmaps = true
			op.GeoM.Scale(sx, sy)
			op.GeoM.Translate(left, top)
			op.ColorScale.Scale(red, green, blue, drawAlpha)
			screen.DrawImage(src, op)
			releaseDrawOpts(op)
		}

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

func replacementEffectPlayerMask(ox, oy int, p framePicture, mobiles []frameMobile, descMap map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64) (*ebiten.Image, float64, float64, float64, uint64) {
	if !replacementEffectReplacesPict(p.PictID) {
		return nil, 0, 0, 0, 0
	}
	best := -1
	bestDist := 65 * 65
	for i, mobile := range mobiles {
		desc, ok := descMap[mobile.Index]
		if !ok || desc.Type == kDescNPC {
			continue
		}
		dh := int(p.H) - int(mobile.H)
		dv := int(p.V) - int(mobile.V)
		dist := dh*dh + dv*dv
		if dist < bestDist {
			best, bestDist = i, dist
		}
	}
	if best < 0 {
		return nil, 0, 0, 0, 0
	}
	mobile := mobiles[best]
	desc := descMap[mobile.Index]
	instanceKey := uint64(1)<<63 | uint64(mobile.Index)
	x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxMobileInterpPixels)
	scaledSize := float64(roundToInt(float64(mobileSize(desc.PictID)) * gs.GameScale))
	if scaledSize <= 0 {
		return nil, float64(x), float64(y), 0, instanceKey
	}
	colors := playerColorsForDescriptor(desc)
	img := loadMobileFrame(desc.PictID, mobile.State, colors)
	img = getScaledMobileFrame(makeMobileKey(desc.PictID, mobile.State, colors), img)
	if img == nil {
		return nil, float64(x), float64(y), scaledSize, instanceKey
	}
	return img, float64(x), float64(y), scaledSize, instanceKey
}

func pictureScreenPosition(ox, oy int, p framePicture, alpha float64, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, prevPicturePositions map[picturePositionKey]struct{}, shiftX, shiftY, width, height int) (int, int) {
	x, y := pictureScreenPositionFloat(ox, oy, p, alpha, mobiles, prevMobiles, prevPicturePositions, shiftX, shiftY, width, height)
	return roundToInt(x), roundToInt(y)
}

func pictureScreenPositionFloat(ox, oy int, p framePicture, alpha float64, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, prevPicturePositions map[picturePositionKey]struct{}, shiftX, shiftY, width, height int) (float64, float64) {
	offX := float64(int(p.PrevH)-int(p.H)) * (1 - alpha)
	offY := float64(int(p.PrevV)-int(p.V)) * (1 - alpha)
	if p.Moving && !gs.smoothMoving && !pictureCloudMotionEnabled(p) {
		if int(p.PrevH) != int(p.H)-shiftX || int(p.PrevV) != int(p.V)-shiftY {
			offX = 0
			offY = 0
		}
	}

	var mobileX, mobileY float64
	// Only independently moving, non-background pictures can be attached to a
	// mobile. Ground sprites may not always be explicitly marked Background.
	if pictureCanPinToMobile(p, width, height) {
		if dx, dy, ok := pictureMobileOffset(p, mobiles, prevMobiles, prevPicturePositions, alpha); ok {
			mobileX, mobileY = dx, dy
			offX = 0
			offY = 0
		}
	}

	x := ((float64(p.H) + offX + mobileX) + float64(fieldCenterX)) * gs.GameScale
	y := ((float64(p.V) + offY + mobileY) + float64(fieldCenterY)) * gs.GameScale
	return x + float64(ox), y + float64(oy)
}

func scaledSpriteSpan(center float64, size int, scale float64) (int, int) {
	half := float64(size) * scale / 2
	return roundToInt(center - half), roundToInt(center + half)
}

func filteredSpriteSpan(center float64, size int, scale float64, filter ebiten.Filter) (float64, float64) {
	start, end := scaledSpriteSpan(center, size, scale)
	if filter == ebiten.FilterLinear {
		return float64(start) - 0.5, float64(end) + 0.5
	}
	return float64(start), float64(end)
}

func pictureAnimationInstanceKey(h, v int16) uint64 {
	return uint64(uint16(h))<<16 | uint64(uint16(v))
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
type picturePositionKey struct {
	pictID uint16
	h, v   int16
}

func hasPreviousPicture(positions map[picturePositionKey]struct{}, pictID uint16, h, v int) bool {
	if h < -32768 || h > 32767 || v < -32768 || v > 32767 {
		return false
	}
	_, ok := positions[picturePositionKey{pictID: pictID, h: int16(h), v: int16(v)}]
	return ok
}

func pictureMobileOffset(p framePicture, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, prevPicturePositions map[picturePositionKey]struct{}, alpha float64) (float64, float64, bool) {
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
		if hasPreviousPicture(prevPicturePositions, p.PictID, expPrevH, expPrevV) {
			h := float64(pm.H)*(1-alpha) + float64(m.H)*alpha
			v := float64(pm.V)*(1-alpha) + float64(m.V)*alpha
			return h - float64(m.H), v - float64(m.V), true
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
		if !hasPreviousPicture(prevPicturePositions, p.PictID, expPrevH, expPrevV) {
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
// It respects motion smoothing and rasterizes name tags at the final display
// scale so cached text is drawn 1:1 rather than resampled with the artwork.
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
	x := screen.Bounds().Min.X + roundToInt((h+float64(fieldCenterX))*gs.GameScale)
	y := screen.Bounds().Min.Y + roundToInt((v+float64(fieldCenterY))*gs.GameScale)
	if d, ok := snap.descriptors[m.Index]; ok {
		if gs.HideSelfNameTag && strings.EqualFold(d.Name, playerName) {
			return
		}
		showName := d.Name != ""
		nameRevealAlpha := float32(1)
		if showName && gs.NameTagsOnHoverOnly {
			lastHoverMu.Lock()
			hovered := lastHover.OnMobile && lastHover.Mobile.Index == m.Index
			lastHoverMu.Unlock()
			nameRevealAlpha = nameTagHoverAlpha(m.Index, d.Name, hovered, drawFrameNow)
			showName = nameRevealAlpha > 0
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
			dead := m.State == poseDead
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
			key := makeNameTagKey(d.Name, m.Colors, d.Type, nameAlpha, style, dead, gs.GameScale)
			img, iw, ih := m.nameTag, m.nameTagW, m.nameTagH
			if img == nil || m.nameTagKey != key {
				img, iw, ih = sharedNameTagImage(key)
				if img != nil {
					// Update shared cache so next frames reuse this image.
					stateMu.Lock()
					if sm, ok := state.mobiles[m.Index]; ok {
						sm.nameTag = img
						sm.nameTagW = iw
						sm.nameTagH = ih
						sm.nameTagKey = key
						state.mobiles[m.Index] = sm
					}
					stateMu.Unlock()
				}
			}
			if img != nil {
				scaledWidth := max(1, iw)
				scaledHeight := max(1, ih)
				top := y + int(offset)
				left := x - scaledWidth/2
				barHeight := 0
				barClr, showHealthBar := mobileHealthBarColor(m.Colors, d.Type)
				if gs.NameHealthBarModern && showHealthBar {
					barHeight = max(1, gs.NameHealthBarThickness)
				}
				nameY, barY := nameHealthBarOffsets(scaledHeight, barHeight, gs.NameHealthBarAbove)
				if barHeight > 0 {
					barClr.A = uint8(float32(barClr.A) * nameRevealAlpha)
					vector.FillRect(screen, float32(left+1), float32(top+barY), float32(max(1, scaledWidth-2)), float32(barHeight), barClr, false)
				}
				op := acquireDrawOpts()
				op.Filter = ebiten.FilterNearest
				op.DisableMipmaps = true
				op.ColorScale.ScaleAlpha(nameRevealAlpha)
				op.GeoM.Translate(float64(left), float64(top+nameY))
				screen.DrawImage(img, op)
				releaseDrawOpts(op)
			}
		} else {
			drawGenericBar()
		}
	}
}

func relativeNameTagScale(worldScale, fontRasterScale float64) float64 {
	if worldScale <= 0 || fontRasterScale <= 0 {
		return 1
	}
	return worldScale / fontRasterScale
}

const speechBubbleReferenceScale = 3.0

// speechBubbleWindowScale follows the physical world size without depending
// on the default supersampling setting. Changing the artwork render default
// must not silently resize chat text for existing configurations.
func speechBubbleWindowScale(finalScale float64) float64 {
	if finalScale <= 0 {
		return 1
	}
	return finalScale / speechBubbleReferenceScale
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
		x := screen.Bounds().Min.X + roundToInt((hpos+float64(fieldCenterX))*gs.GameScale)
		y := screen.Bounds().Min.Y + roundToInt((vpos+float64(fieldCenterY))*gs.GameScale)
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
	bounds := screen.Bounds()
	ox += bounds.Min.X
	oy += bounds.Min.Y
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

	minX := bounds.Min.X - ox
	minY := bounds.Min.Y - oy
	maxX := bounds.Max.X - ox - barWidth - 2*dx
	maxY := bounds.Max.Y - oy - barHeight - 2*dy
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

	if uiReady {
		if !windowsRestored {
			restoreWindowsAfterScale()
		} else if (gs.TiledWindows || gs.AutoResizeWindows) && managedWindowLayoutChanged() {
			applyManagedWindowLayout()
		}
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
	applyVSyncSetting()
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

	go loadSpellcheck()
}

func makeGameWindow() {
	if gameWin != nil {
		return
	}
	gameWin = eui.NewWindow()
	gameWindowFreeformTitleHeight = gameWin.GetRawTitleSize()
	gameWindowFreeformPadding = gameWin.Padding
	gameWindowFreeformMargin = gameWin.Margin
	updateGameWindowTitle()
	gameWin.Closable = false
	gameWin.Resizable = !gs.TiledWindows
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
	gameWin.Maximizable = !gs.TiledWindows
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

func updateGameWindowTitle() {
	if gameWin == nil {
		return
	}
	gameWin.Title = "Clan Lord"
	if playerName != "" {
		gameWin.Title += " -- " + playerName
	}
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
	// The tiled layout owns the outer window geometry. Constraining that
	// rectangle to the playfield aspect ratio shrinks the managed tile and
	// leaves an unused strip (most noticeably with the titlebar hidden).
	// Keep the assigned tile intact and fit the rendered game within it.
	if gs.TiledWindows {
		updateGameImageSize()
		layoutNotifications()
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
		if gs.AltNetMode {
			delay, rs := altNetDelay(frameCount, rampStart, time.Now(), time.Duration(gs.AltNetDelay)*time.Millisecond)
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
		if now.After(nextReliable) && commandQueueIsIdle() && tcpConn != nil {
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
		alreadyProcessed := recordIncomingMovieMessage(m)
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
		if !alreadyProcessed {
			processServerMessage(m)
		}
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
		if !recordIncomingMovieMessage(m) {
			processServerMessage(m)
		}
		// Allow maintenance queues to issue commands even when the
		// player isn't moving; this keeps /be-info and /be-who flowing
		// during idle periods on live connections.
		if commandQueueIsIdle() {
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

// recordIncomingMovieMessage is shared by TCP and UDP so both transports use
// the same existing clMov state-block encoding.
func recordIncomingMovieMessage(m []byte) bool {
	if len(m) < 2 {
		return false
	}
	tag := binary.BigEndian.Uint16(m[:2])
	if recorder == nil && recordingMovie && tag == 2 {
		// Apply the first complete draw before taking the initial snapshot.
		// This avoids an empty baseline when recording was armed pre-login.
		processServerMessage(m)
		startRecording()
		if recorder != nil {
			recordingMovie = false
		}
		return true
	}
	if recorder == nil {
		return false
	}
	if err := recorder.WriteNetworkMessage(m, frameFlags(m)); err != nil {
		logError("record frame: %v", err)
	}
	return false
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
