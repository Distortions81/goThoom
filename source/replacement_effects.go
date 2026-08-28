package main

import (
	_ "embed"
	"image/color"
	"math"
	"os"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var healingBurstPictIDs = map[uint16]struct{}{
	1759: {},
	1760: {},
}

var mysticWardPictIDs = map[uint16]struct{}{
	1286: {},
}

var mysticFadePictIDs = map[uint16]struct{}{
	445: {},
}

var teleportGoldPictIDs = map[uint16]struct{}{
	2976: {},
}

var teleportBluePictIDs = map[uint16]struct{}{
	2977: {},
}

var teleportPrismaticPictIDs = map[uint16]struct{}{
	2978: {},
}

var stoneFormPictIDs = map[uint16]struct{}{
	3125: {},
}

// Coin rewards are individual denomination sprites, not animation frames.
// 1842 is 0, 1843 is 1, through 1851 for 9.
var coinRewardPictIDs = map[uint16]int{
	1842: 0,
	1843: 1,
	1844: 2,
	1845: 3,
	1846: 4,
	1847: 5,
	1848: 6,
	1849: 7,
	1850: 8,
	1851: 9,
}

//go:embed data/shaders/healing_burst.kage
var healingBurstShaderSource []byte

//go:embed data/shaders/mystic_ward.kage
var mysticWardShaderSource []byte

//go:embed data/shaders/mystic_fade.kage
var mysticFadeShaderSource []byte

//go:embed data/shaders/teleport_burst.kage
var teleportBurstShaderSource []byte

//go:embed data/shaders/stone_form.kage
var stoneFormShaderSource []byte

//go:embed data/shaders/coin_reward.kage
var coinRewardShaderSource []byte

var healingBurstShader *ebiten.Shader
var mysticWardShader *ebiten.Shader
var mysticFadeShader *ebiten.Shader
var teleportBurstShader *ebiten.Shader
var stoneFormShader *ebiten.Shader
var coinRewardShader *ebiten.Shader
var replacementEffectsStarted = time.Now()
var replacementEffectsPreview bool
var replacementEffectsShadersReady bool
var replacementEffectsShaderInitAttempted bool
var replacementEffectsShaderInitIndex int
var replacementEffectsShaderInitMu sync.Mutex

type replacementEffectKind uint8

const (
	replacementEffectHealing replacementEffectKind = iota + 1
	replacementEffectMysticWard
	replacementEffectMysticFade
	replacementEffectTeleportGold
	replacementEffectTeleportBlue
	replacementEffectTeleportPrismatic
	replacementEffectStoneForm
	replacementEffectCoinReward
)

type replacementEffectDraw struct {
	pictID                      uint16
	kind                        replacementEffectKind
	left, top, width, height    float64
	contentWidth, contentHeight float64
	alpha                       float32
	frame                       int
	coinDigits                  [4]float32
	coinDigitX                  [4]float64
	coinDigitCount              int
	coinGroupLeft               float64
	coinGroupTop                float64
	coinGroupRight              float64
	coinGroupBottom             float64
	started, lastSeen           time.Time
	seen                        bool
	mobileIndex                 uint8
	hasMobileAnchor             bool
	mobileOffsetX               float64
	mobileOffsetY               float64
	maskImage                   *ebiten.Image
	maskOffsetX                 float32
	maskOffsetY                 float32
	maskInvScale                float32
	hasMask                     bool
}

var replacementEffectDraws = make(map[uint64]replacementEffectDraw)
var replacementEffectNextKey uint64

type replacementEffectShaderState struct {
	op         ebiten.DrawTrianglesShaderOptions
	uniforms   map[string]any
	size       [2]float32
	canvasSize [2]float32
	maskOffset [2]float32
	coinDigits [4]float32
}

var replacementEffectShaderStates [replacementEffectCoinReward + 1]replacementEffectShaderState
var replacementEffectShaderIndices = []uint16{0, 1, 2, 1, 2, 3}

func init() {
	for kind := replacementEffectHealing; kind <= replacementEffectCoinReward; kind++ {
		state := &replacementEffectShaderStates[kind]
		state.uniforms = map[string]any{
			"Size":            state.size[:],
			"Phase":           float32(0),
			"Alpha":           float32(0),
			"Energy":          float32(0),
			"HasMask":         float32(0),
			"MaskOffset":      state.maskOffset[:],
			"MaskInvScale":    float32(1),
			"SpriteLightOnly": float32(0),
		}
		if replacementEffectTeleportTheme(kind) >= 0 {
			state.uniforms["CanvasSize"] = state.canvasSize[:]
			state.uniforms["TeleportTheme"] = float32(0)
		}
		if kind == replacementEffectCoinReward {
			state.uniforms["CanvasSize"] = state.canvasSize[:]
			state.uniforms["CoinValue"] = float32(0)
			state.uniforms["CoinDigits"] = state.coinDigits[:]
			state.uniforms["CoinDigitCount"] = float32(0)
		}
		state.op.Uniforms = state.uniforms
	}
}

func drawReplacementEffectShader(screen *ebiten.Image, left, top float64, width, height int, shader *ebiten.Shader, state *replacementEffectShaderState) {
	sourceLeft, sourceTop := float32(0), float32(0)
	if source := state.op.Images[0]; source != nil {
		sourceLeft = float32(source.Bounds().Min.X)
		sourceTop = float32(source.Bounds().Min.Y)
	}
	right := float32(left) + float32(width)
	bottom := float32(top) + float32(height)
	sourceRight := sourceLeft + float32(width)
	sourceBottom := sourceTop + float32(height)
	vertices := [...]ebiten.Vertex{
		{DstX: float32(left), DstY: float32(top), SrcX: sourceLeft, SrcY: sourceTop},
		{DstX: right, DstY: float32(top), SrcX: sourceRight, SrcY: sourceTop},
		{DstX: float32(left), DstY: bottom, SrcX: sourceLeft, SrcY: sourceBottom},
		{DstX: right, DstY: bottom, SrcX: sourceRight, SrcY: sourceBottom},
	}
	for index := range vertices {
		vertices[index].ColorR = 1
		vertices[index].ColorG = 1
		vertices[index].ColorB = 1
		vertices[index].ColorA = 1
	}
	screen.DrawTrianglesShader(vertices[:], replacementEffectShaderIndices, shader, &state.op)
}

const (
	replacementEffectFadeIn  = 180 * time.Millisecond
	replacementEffectFadeOut = 280 * time.Millisecond
)

// ReloadReplacementEffectsShader recompiles the replacement-effects shader
// from disk. The embedded source keeps release builds self-contained.
func ReloadReplacementEffectsShader() error {
	replacementEffectsShaderInitMu.Lock()
	defer replacementEffectsShaderInitMu.Unlock()

	healingShader, err := compileReplacementEffectShaderForInit("healing_burst.kage", healingBurstShaderSource)
	if err != nil {
		return err
	}
	wardShader, err := compileReplacementEffectShaderForInit("mystic_ward.kage", mysticWardShaderSource)
	if err != nil {
		return err
	}
	fadeShader, err := compileReplacementEffectShaderForInit("mystic_fade.kage", mysticFadeShaderSource)
	if err != nil {
		return err
	}
	teleportShader, err := compileReplacementEffectShaderForInit("teleport_burst.kage", teleportBurstShaderSource)
	if err != nil {
		return err
	}
	stoneShader, err := compileReplacementEffectShaderForInit("stone_form.kage", stoneFormShaderSource)
	if err != nil {
		return err
	}
	coinShader, err := compileReplacementEffectShaderForInit("coin_reward.kage", coinRewardShaderSource)
	if err != nil {
		return err
	}
	healingBurstShader = healingShader
	mysticWardShader = wardShader
	mysticFadeShader = fadeShader
	teleportBurstShader = teleportShader
	stoneFormShader = stoneShader
	coinRewardShader = coinShader
	replacementEffectsShadersReady = true
	replacementEffectsShaderInitAttempted = true
	replacementEffectsShaderInitIndex = replacementEffectsShaderCount
	return nil
}

const replacementEffectsShaderCount = 6

func compileReplacementEffectShader(name string, embedded []byte) (*ebiten.Shader, error) {
	source := embedded
	if b, err := os.ReadFile("data/shaders/" + name); err == nil {
		source = b
	}
	return ebiten.NewShader(source)
}

var compileReplacementEffectShaderForInit = compileReplacementEffectShader

func replacementEffectsShaderInitializationPending() bool {
	replacementEffectsShaderInitMu.Lock()
	defer replacementEffectsShaderInitMu.Unlock()
	return !replacementEffectsShadersReady && !replacementEffectsShaderInitAttempted
}

// loadNextReplacementEffectShader compiles one shader so the event loop can
// present a frame between each expensive Kage compilation.
func loadNextReplacementEffectShader() error {
	replacementEffectsShaderInitMu.Lock()
	defer replacementEffectsShaderInitMu.Unlock()

	if replacementEffectsShadersReady || replacementEffectsShaderInitAttempted {
		return nil
	}

	var (
		shader *ebiten.Shader
		err    error
	)
	switch replacementEffectsShaderInitIndex {
	case 0:
		shader, err = compileReplacementEffectShaderForInit("healing_burst.kage", healingBurstShaderSource)
		if err == nil {
			healingBurstShader = shader
		}
	case 1:
		shader, err = compileReplacementEffectShaderForInit("mystic_ward.kage", mysticWardShaderSource)
		if err == nil {
			mysticWardShader = shader
		}
	case 2:
		shader, err = compileReplacementEffectShaderForInit("mystic_fade.kage", mysticFadeShaderSource)
		if err == nil {
			mysticFadeShader = shader
		}
	case 3:
		shader, err = compileReplacementEffectShaderForInit("teleport_burst.kage", teleportBurstShaderSource)
		if err == nil {
			teleportBurstShader = shader
		}
	case 4:
		shader, err = compileReplacementEffectShaderForInit("stone_form.kage", stoneFormShaderSource)
		if err == nil {
			stoneFormShader = shader
		}
	case 5:
		shader, err = compileReplacementEffectShaderForInit("coin_reward.kage", coinRewardShaderSource)
		if err == nil {
			coinRewardShader = shader
		}
	default:
		replacementEffectsShadersReady = true
		replacementEffectsShaderInitAttempted = true
		return nil
	}
	if err != nil {
		replacementEffectsShaderInitAttempted = true
		return err
	}
	replacementEffectsShaderInitIndex++
	if replacementEffectsShaderInitIndex == replacementEffectsShaderCount {
		replacementEffectsShadersReady = true
		replacementEffectsShaderInitAttempted = true
	}
	return nil
}

func beginReplacementEffects() {
	if !replacementEffectsEnabled() {
		clear(replacementEffectDraws)
		return
	}
	for key, effect := range replacementEffectDraws {
		effect.seen = false
		replacementEffectDraws[key] = effect
	}
}

func replacementEffectReplacesPict(id uint16) bool {
	if !replacementEffectsShadersReady || !replacementEffectsEnabled() {
		return false
	}
	_, ok := replacementEffectKindForPict(id)
	return ok
}

func replacementEffectKindForPict(id uint16) (replacementEffectKind, bool) {
	if _, ok := healingBurstPictIDs[id]; ok {
		return replacementEffectHealing, true
	}
	if _, ok := mysticWardPictIDs[id]; ok {
		return replacementEffectMysticWard, true
	}
	if _, ok := mysticFadePictIDs[id]; ok {
		return replacementEffectMysticFade, true
	}
	if _, ok := teleportGoldPictIDs[id]; ok {
		return replacementEffectTeleportGold, true
	}
	if _, ok := teleportBluePictIDs[id]; ok {
		return replacementEffectTeleportBlue, true
	}
	if _, ok := teleportPrismaticPictIDs[id]; ok {
		return replacementEffectTeleportPrismatic, true
	}
	if _, ok := stoneFormPictIDs[id]; ok {
		return replacementEffectStoneForm, true
	}
	if _, ok := coinRewardPictIDs[id]; ok {
		return replacementEffectCoinReward, true
	}
	return 0, false
}

func replacementEffectShader(kind replacementEffectKind) *ebiten.Shader {
	if kind == replacementEffectMysticWard {
		return mysticWardShader
	}
	if kind == replacementEffectMysticFade {
		return mysticFadeShader
	}
	if replacementEffectTeleportTheme(kind) >= 0 {
		return teleportBurstShader
	}
	if kind == replacementEffectStoneForm {
		return stoneFormShader
	}
	if kind == replacementEffectCoinReward {
		return coinRewardShader
	}
	return healingBurstShader
}

func replacementEffectIsOneShot(kind replacementEffectKind) bool {
	return kind == replacementEffectMysticWard || kind == replacementEffectMysticFade || replacementEffectTeleportTheme(kind) >= 0
}

func replacementEffectTeleportTheme(kind replacementEffectKind) float32 {
	switch kind {
	case replacementEffectTeleportGold:
		return 0
	case replacementEffectTeleportBlue:
		return 1
	case replacementEffectTeleportPrismatic:
		return 2
	default:
		return -1
	}
}

func replacementEffectNeedsOverscan(kind replacementEffectKind) bool {
	return kind == replacementEffectCoinReward || replacementEffectTeleportTheme(kind) >= 0
}

// queueReplacementPictureEffect preserves the legacy effect's world anchor
// while deferring the visual to a full-bright pass after scene lighting.
func queueReplacementPictureEffect(pictID uint16, frame int, h, v int16, instanceKey uint64, left, top, width, height float64, alpha float32, mobileImg *ebiten.Image, mobileX, mobileY, mobileSize float64) bool {
	if !replacementEffectReplacesPict(pictID) || width <= 0 || height <= 0 {
		return false
	}
	kind, _ := replacementEffectKindForPict(pictID)
	if kind == replacementEffectCoinReward {
		frame = coinRewardPictIDs[pictID]
	}
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	// 1759 and 1760 are two legacy representations of the same healing
	// family. Prefer the pinned mobile identity so movement does not restart
	// the effect; unpinned effects fall back to their world position.
	var key uint64
	if kind == replacementEffectCoinReward {
		key = replacementCoinClusterKey(left, top, width, height, now)
	} else {
		key = instanceKey
		if key != 0 {
			key |= uint64(kind) << 56
		} else {
			key = uint64(uint16(h))<<16 | uint64(uint16(v))
			key |= uint64(kind) << 48
		}
	}
	effect, ok := replacementEffectDraws[key]
	if !ok || now.Sub(effect.lastSeen) > replacementEffectFadeOut {
		effect = replacementEffectDraw{pictID: pictID, kind: kind, started: now}
	}
	if kind == replacementEffectCoinReward {
		if !effect.seen {
			effect.coinDigitCount = 0
			effect.coinGroupLeft, effect.coinGroupTop = left, top
			effect.coinGroupRight, effect.coinGroupBottom = left+width, top+height
		} else {
			effect.coinGroupLeft = math.Min(effect.coinGroupLeft, left)
			effect.coinGroupTop = math.Min(effect.coinGroupTop, top)
			effect.coinGroupRight = math.Max(effect.coinGroupRight, left+width)
			effect.coinGroupBottom = math.Max(effect.coinGroupBottom, top+height)
		}
		if effect.coinDigitCount < len(effect.coinDigits) {
			i := effect.coinDigitCount
			effect.coinDigits[i] = float32(frame)
			effect.coinDigitX[i] = left + width/2
			effect.coinDigitCount++
			// The legacy sprites are laid out left-to-right. Preserve that order
			// while combining them into one readable reward number.
			for i > 0 && effect.coinDigitX[i] < effect.coinDigitX[i-1] {
				effect.coinDigits[i], effect.coinDigits[i-1] = effect.coinDigits[i-1], effect.coinDigits[i]
				effect.coinDigitX[i], effect.coinDigitX[i-1] = effect.coinDigitX[i-1], effect.coinDigitX[i]
				i--
			}
		}
		// Center a generous round coin on the original sprite group, rather
		// than pinning three tiny, independently fading replacements.
		groupWidth := effect.coinGroupRight - effect.coinGroupLeft
		groupHeight := effect.coinGroupBottom - effect.coinGroupTop
		centerX := effect.coinGroupLeft + groupWidth/2
		centerY := effect.coinGroupTop + groupHeight/2
		// Give multi-digit payouts enough face area for the full number: each
		// added digit grows the coin instead of cramming more glyphs into the
		// original single-denomination sprite size.
		minimumSize := 56 + 28*float64(max(0, effect.coinDigitCount-1))
		contentSize := 0.75 * math.Max(minimumSize, math.Max(groupWidth+22, groupHeight+18))
		canvasPadding := contentSize * 0.35
		effect.contentWidth, effect.contentHeight = contentSize, contentSize
		effect.width, effect.height = contentSize+2*canvasPadding, contentSize+2*canvasPadding
		effect.left, effect.top = centerX-effect.width/2, centerY-effect.height/2
	} else {
		effect.left, effect.top = left, top
		effect.width, effect.height = width, height
		effect.contentWidth, effect.contentHeight = width, height
		if replacementEffectNeedsOverscan(kind) {
			padding := 0.35 * math.Max(width, height)
			effect.left, effect.top = left-padding, top-padding
			effect.width, effect.height = width+2*padding, height+2*padding
		}
	}
	effect.alpha = alpha
	effect.frame = frame
	effect.lastSeen = now
	effect.seen = true
	// Coin rewards can appear in open world space as well as over a mobile.
	// Keep their native group position stable instead of letting a transient
	// nearest-mobile choice split or move the digits between updates.
	effect.hasMobileAnchor = kind != replacementEffectCoinReward && instanceKey != 0
	if effect.hasMobileAnchor {
		effect.mobileIndex = uint8(instanceKey)
		effect.mobileOffsetX = effect.left - mobileX
		effect.mobileOffsetY = effect.top - mobileY
	}
	updateReplacementEffectMask(&effect, mobileImg, mobileX, mobileY, mobileSize)
	replacementEffectDraws[key] = effect
	return true
}

// replacementCoinClusterKey keeps a reward together while its legacy digit
// sprites wobble slightly between updates. The distance is measured after the
// normal world transform so it naturally follows game scaling.
func replacementCoinClusterKey(left, top, width, height float64, now time.Time) uint64 {
	cx, cy := left+width/2, top+height/2
	radius := math.Max(24, 24*gs.GameScale)
	bestKey := uint64(0)
	bestDistance := radius * radius
	for key, effect := range replacementEffectDraws {
		if effect.kind != replacementEffectCoinReward || now.Sub(effect.lastSeen) > replacementEffectFadeOut {
			continue
		}
		ex, ey := effect.coinGroupLeft+(effect.coinGroupRight-effect.coinGroupLeft)/2, effect.coinGroupTop+(effect.coinGroupBottom-effect.coinGroupTop)/2
		dx, dy := cx-ex, cy-ey
		distance := dx*dx + dy*dy
		if distance <= bestDistance {
			bestKey, bestDistance = key, distance
		}
	}
	if bestKey != 0 {
		return bestKey
	}
	replacementEffectNextKey++
	return uint64(replacementEffectCoinReward)<<56 | replacementEffectNextKey
}

func updateReplacementEffectMask(effect *replacementEffectDraw, mobileImg *ebiten.Image, mobileX, mobileY, mobileSize float64) {
	effect.hasMask = false
	effect.maskImage = whiteImage
	effect.maskOffsetX = 0
	effect.maskOffsetY = 0
	effect.maskInvScale = 1
	if mobileImg == nil || mobileSize <= 0 {
		return
	}
	bounds := mobileImg.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	scale := mobileSize / float64(bounds.Dx())
	if scale <= 0 {
		return
	}
	effect.maskImage = mobileImg
	effect.maskOffsetX = float32(mobileX - mobileSize/2 - effect.left)
	effect.maskOffsetY = float32(mobileY - mobileSize/2 - effect.top)
	effect.maskInvScale = float32(1 / scale)
	effect.hasMask = true
}

func drawReplacementEffects(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64) {
	if !replacementEffectsEnabled() {
		return
	}
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	globalPhase := float32(now.Sub(replacementEffectsStarted).Seconds())
	for key, effect := range replacementEffectDraws {
		if effect.hasMobileAnchor {
			for _, mobile := range mobiles {
				if mobile.Index != effect.mobileIndex {
					continue
				}
				x, y := mobileScreenPositionFloat(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxMobileInterpPixels)
				effect.left = x + effect.mobileOffsetX
				effect.top = y + effect.mobileOffsetY
				break
			}
		}
		fadeIn := replacementEffectEase(float32(now.Sub(effect.started)) / float32(replacementEffectFadeIn))
		if effect.kind == replacementEffectCoinReward {
			// Reward digits may exist for only one server update. Draw them at
			// full strength immediately; the normal fade-out still softens exit.
			fadeIn = 1
		}
		fadeOut := float32(1)
		if !effect.seen {
			age := now.Sub(effect.lastSeen)
			if age >= replacementEffectFadeOut {
				delete(replacementEffectDraws, key)
				continue
			}
			fadeOut = 1 - replacementEffectEase(float32(age)/float32(replacementEffectFadeOut))
		}
		energy := fadeIn * fadeOut
		if energy <= 0 {
			continue
		}
		visualAlpha := effect.alpha * energy
		if effect.kind == replacementEffectCoinReward {
			// The legacy reward picture can be obscured or dimmed by its target.
			// A payout marker needs to remain readable above the world; only its
			// own exit fade should affect opacity.
			visualAlpha = 0.80 * energy
		}
		// Keep the rectangle identical to the alpha-mask texture. Coin rewards
		// reserve an oversized canvas so their stars can extend beyond the face.
		w, h := int(math.Ceil(effect.width)), int(math.Ceil(effect.height))
		if w <= 0 || h <= 0 {
			continue
		}
		phase := globalPhase
		if replacementEffectIsOneShot(effect.kind) {
			// One-shot effects start their particle sequence when this sprite
			// appears instead of inheriting the global shader phase.
			phase = float32(now.Sub(effect.started).Seconds())
		}
		shader := replacementEffectShader(effect.kind)
		contentW, contentH := effect.contentWidth, effect.contentHeight
		if contentW <= 0 || contentH <= 0 {
			contentW, contentH = effect.width, effect.height
		}
		hasMask := float32(0)
		if effect.hasMask && effect.kind != replacementEffectCoinReward {
			hasMask = 1
		}
		// Light the mobile itself in a separate additive pass. This preserves
		// the original sprite detail while lifting its opaque pixels out of the
		// dusk-darkened scene with the burst's blue light.
		state := &replacementEffectShaderStates[effect.kind]
		state.size = [2]float32{float32(contentW), float32(contentH)}
		state.canvasSize = [2]float32{float32(w), float32(h)}
		state.maskOffset = [2]float32{effect.maskOffsetX, effect.maskOffsetY}
		state.coinDigits = effect.coinDigits
		state.uniforms["Phase"] = phase
		state.uniforms["Alpha"] = visualAlpha
		state.uniforms["Energy"] = energy
		state.uniforms["HasMask"] = hasMask
		state.uniforms["MaskInvScale"] = effect.maskInvScale
		if theme := replacementEffectTeleportTheme(effect.kind); theme >= 0 {
			state.uniforms["TeleportTheme"] = theme
		}
		if effect.kind == replacementEffectCoinReward {
			state.uniforms["CoinValue"] = float32(effect.frame % 10)
			state.uniforms["CoinDigitCount"] = float32(effect.coinDigitCount)
		}
		state.op.Images[0] = effect.maskImage
		if state.op.Images[0] == nil {
			state.op.Images[0] = whiteImage
		}
		if effect.hasMask && effect.kind != replacementEffectCoinReward {
			state.op.Blend = ebiten.BlendLighter
			state.uniforms["SpriteLightOnly"] = float32(1)
			drawReplacementEffectShader(screen, effect.left, effect.top, w, h, shader, state)
		}
		state.op.Blend = ebiten.Blend{}
		state.uniforms["SpriteLightOnly"] = float32(0)
		drawReplacementEffectShader(screen, effect.left, effect.top, w, h, shader, state)
	}
}

func replacementEffectEase(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

type replacementEffectPreview struct {
	kind           replacementEffectKind
	label          string
	coinDigits     [4]float32
	coinDigitCount int
}

var replacementEffectsPreviews = []replacementEffectPreview{
	{kind: replacementEffectHealing, label: "Healing"},
	{kind: replacementEffectMysticWard, label: "Mystic Ward"},
	{kind: replacementEffectMysticFade, label: "Ward Fading"},
	{kind: replacementEffectTeleportGold, label: "Gold Teleport"},
	{kind: replacementEffectTeleportBlue, label: "Blue Teleport"},
	{kind: replacementEffectTeleportPrismatic, label: "Prismatic Teleport"},
	{kind: replacementEffectStoneForm, label: "Stone Form"},
	{kind: replacementEffectCoinReward, label: "Coin 123", coinDigits: [4]float32{1, 2, 3}, coinDigitCount: 3},
}

// drawReplacementEffectsPreview renders the actual effect shaders in a
// looping gallery, so visual tuning does not require finding a movie event.
func drawReplacementEffectsPreview(screen *ebiten.Image) {
	if !replacementEffectsShadersReady || !gs.ShadersEnabled {
		return
	}
	bounds := screen.Bounds()
	if bounds.Dx() < 240 || bounds.Dy() < 180 {
		return
	}
	vector.FillRect(screen, float32(bounds.Min.X), float32(bounds.Min.Y), float32(bounds.Dx()), float32(bounds.Dy()), color.Black, false)

	const columns = 3
	cellW := float64(bounds.Dx()) / columns
	rows := (len(replacementEffectsPreviews) + columns - 1) / columns
	cellH := float64(bounds.Dy()) / float64(rows)
	effectW := max(56, roundToInt(cellW*0.68))
	effectH := max(56, roundToInt(cellH*0.66))
	coinCanvasW, coinCanvasH := roundToInt(float64(effectW)*1.70), roundToInt(float64(effectH)*1.70)
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	elapsed := now.Sub(replacementEffectsStarted).Seconds()
	for i, preview := range replacementEffectsPreviews {
		col, row := i%columns, i/columns
		left := float64(bounds.Min.X) + float64(col)*cellW + (cellW-float64(effectW))/2
		top := float64(bounds.Min.Y) + float64(row)*cellH + 28
		phase := float32(elapsed)
		if replacementEffectIsOneShot(preview.kind) {
			phase = float32(math.Mod(elapsed+float64(i)*0.23, 1.35))
		}
		drawW, drawH := effectW, effectH
		drawLeft, drawTop := left, top
		state := &replacementEffectShaderStates[preview.kind]
		state.size = [2]float32{float32(effectW), float32(effectH)}
		state.canvasSize = [2]float32{float32(effectW), float32(effectH)}
		state.maskOffset = [2]float32{}
		state.coinDigits = preview.coinDigits
		state.uniforms["Phase"] = phase
		state.uniforms["Alpha"] = float32(1)
		state.uniforms["Energy"] = float32(1)
		state.uniforms["HasMask"] = float32(0)
		state.uniforms["MaskInvScale"] = float32(1)
		state.uniforms["SpriteLightOnly"] = float32(0)
		if theme := replacementEffectTeleportTheme(preview.kind); theme >= 0 {
			state.uniforms["TeleportTheme"] = theme
		}
		if replacementEffectNeedsOverscan(preview.kind) {
			drawW, drawH = coinCanvasW, coinCanvasH
			drawLeft -= float64(drawW-effectW) / 2
			drawTop -= float64(drawH-effectH) / 2
			state.canvasSize = [2]float32{float32(drawW), float32(drawH)}
		}
		if preview.kind == replacementEffectCoinReward {
			state.uniforms["CoinValue"] = preview.coinDigits[0]
			state.uniforms["CoinDigitCount"] = float32(preview.coinDigitCount)
		}
		state.op.Blend = ebiten.Blend{}
		state.op.Images[0] = whiteImage
		drawReplacementEffectShader(screen, drawLeft, drawTop, drawW, drawH, replacementEffectShader(preview.kind), state)

		labelOpts := acquireTextDrawOpts()
		labelOpts.GeoM.Translate(float64(bounds.Min.X)+float64(col)*cellW+8, float64(bounds.Min.Y)+float64(row)*cellH+8)
		labelOpts.ColorScale.ScaleWithColor(color.RGBA{R: 224, G: 234, B: 255, A: 255})
		text.Draw(screen, preview.label, mainFont, labelOpts)
		releaseTextDrawOpts(labelOpts)
	}
}
