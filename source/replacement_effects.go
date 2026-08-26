package main

import (
	_ "embed"
	"image/color"
	"math"
	"os"
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
var replacementEffectsPreviewMask *ebiten.Image
var replacementEffectsPreviewCoinMask *ebiten.Image
var replacementEffectsShadersReady bool
var replacementEffectsShaderInitAttempted bool
var replacementEffectsShaderInitAfter time.Time

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
	pictID                   uint16
	kind                     replacementEffectKind
	left, top, width, height float64
	contentWidth, contentHeight float64
	alpha                    float32
	frame                    int
	coinDigits               [4]float32
	coinDigitX               [4]float64
	coinDigitCount           int
	coinGroupLeft             float64
	coinGroupTop              float64
	coinGroupRight            float64
	coinGroupBottom           float64
	started, lastSeen        time.Time
	seen                     bool
	mobileIndex              uint8
	hasMobileAnchor          bool
	mobileOffsetX            float64
	mobileOffsetY            float64
	mask                     *ebiten.Image
	hasMask                  bool
}

var replacementEffectDraws = make(map[uint64]replacementEffectDraw)
var replacementEffectNextKey uint64

const (
	replacementEffectFadeIn  = 180 * time.Millisecond
	replacementEffectFadeOut = 280 * time.Millisecond
)

// ReloadReplacementEffectsShader recompiles the replacement-effects shader
// from disk. The embedded source keeps release builds self-contained.
func ReloadReplacementEffectsShader() error {
	healingSource := healingBurstShaderSource
	if b, err := os.ReadFile("data/shaders/healing_burst.kage"); err == nil {
		healingSource = b
	}
	healingShader, err := ebiten.NewShader(healingSource)
	if err != nil {
		return err
	}
	wardSource := mysticWardShaderSource
	if b, err := os.ReadFile("data/shaders/mystic_ward.kage"); err == nil {
		wardSource = b
	}
	wardShader, err := ebiten.NewShader(wardSource)
	if err != nil {
		return err
	}
	fadeSource := mysticFadeShaderSource
	if b, err := os.ReadFile("data/shaders/mystic_fade.kage"); err == nil {
		fadeSource = b
	}
	fadeShader, err := ebiten.NewShader(fadeSource)
	if err != nil {
		return err
	}
	teleportSource := teleportBurstShaderSource
	if b, err := os.ReadFile("data/shaders/teleport_burst.kage"); err == nil {
		teleportSource = b
	}
	teleportShader, err := ebiten.NewShader(teleportSource)
	if err != nil {
		return err
	}
	stoneSource := stoneFormShaderSource
	if b, err := os.ReadFile("data/shaders/stone_form.kage"); err == nil {
		stoneSource = b
	}
	stoneShader, err := ebiten.NewShader(stoneSource)
	if err != nil {
		return err
	}
	coinSource := coinRewardShaderSource
	if b, err := os.ReadFile("data/shaders/coin_reward.kage"); err == nil {
		coinSource = b
	}
	coinShader, err := ebiten.NewShader(coinSource)
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
	return nil
}

// initializeReplacementEffectsAfterMenu delays the optional shader work until
// the menu has had time to settle, instead of blocking package startup or a
// draw pass while windows are being composed.
func initializeReplacementEffectsAfterMenu(now time.Time) {
	if !uiReady {
		return
	}
	if replacementEffectsShaderInitAfter.IsZero() {
		replacementEffectsShaderInitAfter = now.Add(750 * time.Millisecond)
		return
	}
	if replacementEffectsShadersReady || replacementEffectsShaderInitAttempted {
		return
	}
	if now.Before(replacementEffectsShaderInitAfter) {
		return
	}
	replacementEffectsShaderInitAttempted = true
	if err := ReloadReplacementEffectsShader(); err != nil {
		logError("replacement shader initialization failed: %v", err)
	}
}

func beginReplacementEffects() {
	for key, effect := range replacementEffectDraws {
		effect.seen = false
		replacementEffectDraws[key] = effect
	}
}

func replacementEffectReplacesPict(id uint16) bool {
	if !replacementEffectsShadersReady || !gs.ReplacementEffects {
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
	w, h := int(math.Ceil(effect.width)), int(math.Ceil(effect.height))
	if w <= 0 || h <= 0 {
		return
	}
	if effect.mask == nil || effect.mask.Bounds().Dx() != w || effect.mask.Bounds().Dy() != h {
		if effect.mask != nil {
			effect.mask.Deallocate()
		}
		effect.mask = newImage(w, h)
	}
	effect.mask.Clear()
	// The shader references image slot 0 even when HasMask is false, so Kage
	// still requires a bound texture for unmatched or unpinned effects.
	if mobileImg == nil || mobileSize <= 0 {
		return
	}
	bounds := mobileImg.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	op := acquireDrawOpts()
	op.Filter = worldArtworkFilter()
	op.DisableMipmaps = true
	scale := mobileSize / float64(bounds.Dx())
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(
		mobileX-mobileSize/2-effect.left,
		mobileY-mobileSize/2-effect.top,
	)
	effect.mask.DrawImage(mobileImg, op)
	releaseDrawOpts(op)
	effect.hasMask = true
}

func drawReplacementEffects(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64) {
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	for key, effect := range replacementEffectDraws {
		if effect.hasMobileAnchor {
			for _, mobile := range mobiles {
				if mobile.Index != effect.mobileIndex {
					continue
				}
				x, y := mobileScreenPosition(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxMobileInterpPixels)
				effect.left = float64(x) + effect.mobileOffsetX
				effect.top = float64(y) + effect.mobileOffsetY
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
				if effect.mask != nil {
					effect.mask.Deallocate()
				}
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
		phase := float32(time.Since(replacementEffectsStarted).Seconds())
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
		if effect.hasMask && effect.kind != replacementEffectCoinReward {
			lightOp := &ebiten.DrawRectShaderOptions{
				Blend: ebiten.BlendLighter,
				Uniforms: map[string]any{
					"Size":            []float32{float32(contentW), float32(contentH)},
					"Phase":           phase,
					"Alpha":           visualAlpha,
					"Energy":          energy,
					"HasMask":         hasMask,
					"SpriteLightOnly": float32(1),
				},
			}
			if theme := replacementEffectTeleportTheme(effect.kind); theme >= 0 {
				lightOp.Uniforms["TeleportTheme"] = theme
				lightOp.Uniforms["CanvasSize"] = []float32{float32(w), float32(h)}
			}
			if effect.kind == replacementEffectCoinReward {
				lightOp.Uniforms["CoinValue"] = float32(effect.frame % 10)
				lightOp.Uniforms["CoinDigits"] = effect.coinDigits[:]
				lightOp.Uniforms["CoinDigitCount"] = float32(effect.coinDigitCount)
			}
			lightOp.Images[0] = effect.mask
			lightOp.GeoM.Translate(effect.left, effect.top)
			screen.DrawRectShader(w, h, shader, lightOp)
		}
		op := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
			"Size":            []float32{float32(contentW), float32(contentH)},
			"Phase":           phase,
			"Alpha":           visualAlpha,
			"Energy":          energy,
			"HasMask":         hasMask,
			"SpriteLightOnly": float32(0),
		}}
		if theme := replacementEffectTeleportTheme(effect.kind); theme >= 0 {
			op.Uniforms["TeleportTheme"] = theme
			op.Uniforms["CanvasSize"] = []float32{float32(w), float32(h)}
		}
		if effect.kind == replacementEffectCoinReward {
			op.Uniforms["CoinValue"] = float32(effect.frame % 10)
			op.Uniforms["CoinDigits"] = effect.coinDigits[:]
			op.Uniforms["CoinDigitCount"] = float32(effect.coinDigitCount)
			op.Uniforms["CanvasSize"] = []float32{float32(w), float32(h)}
		}
		op.Images[0] = effect.mask
		op.GeoM.Translate(effect.left, effect.top)
		screen.DrawRectShader(w, h, shader, op)
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
	if !replacementEffectsShadersReady {
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
	if replacementEffectsPreviewMask == nil || replacementEffectsPreviewMask.Bounds().Dx() != effectW || replacementEffectsPreviewMask.Bounds().Dy() != effectH {
		if replacementEffectsPreviewMask != nil {
			replacementEffectsPreviewMask.Deallocate()
		}
		replacementEffectsPreviewMask = ebiten.NewImage(effectW, effectH)
	}
	coinCanvasW, coinCanvasH := roundToInt(float64(effectW)*1.70), roundToInt(float64(effectH)*1.70)
	if replacementEffectsPreviewCoinMask == nil || replacementEffectsPreviewCoinMask.Bounds().Dx() != coinCanvasW || replacementEffectsPreviewCoinMask.Bounds().Dy() != coinCanvasH {
		if replacementEffectsPreviewCoinMask != nil {
			replacementEffectsPreviewCoinMask.Deallocate()
		}
		replacementEffectsPreviewCoinMask = ebiten.NewImage(coinCanvasW, coinCanvasH)
	}
	elapsed := time.Since(replacementEffectsStarted).Seconds()
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
		mask := replacementEffectsPreviewMask
		op := &ebiten.DrawRectShaderOptions{Uniforms: map[string]any{
			"Size":            []float32{float32(effectW), float32(effectH)},
			"Phase":           phase,
			"Alpha":           float32(1),
			"Energy":          float32(1),
			"HasMask":         float32(0),
			"SpriteLightOnly": float32(0),
		}}
		if theme := replacementEffectTeleportTheme(preview.kind); theme >= 0 {
			op.Uniforms["TeleportTheme"] = theme
		}
		if replacementEffectNeedsOverscan(preview.kind) {
			drawW, drawH = coinCanvasW, coinCanvasH
			drawLeft -= float64(drawW-effectW) / 2
			drawTop -= float64(drawH-effectH) / 2
			mask = replacementEffectsPreviewCoinMask
			op.Uniforms["CanvasSize"] = []float32{float32(drawW), float32(drawH)}
		}
		if preview.kind == replacementEffectCoinReward {
			op.Uniforms["CoinValue"] = preview.coinDigits[0]
			op.Uniforms["CoinDigits"] = preview.coinDigits[:]
			op.Uniforms["CoinDigitCount"] = float32(preview.coinDigitCount)
		}
		op.Images[0] = mask
		op.GeoM.Translate(drawLeft, drawTop)
		screen.DrawRectShader(drawW, drawH, replacementEffectShader(preview.kind), op)

		labelOpts := acquireTextDrawOpts()
		labelOpts.GeoM.Translate(float64(bounds.Min.X)+float64(col)*cellW+8, float64(bounds.Min.Y)+float64(row)*cellH+8)
		labelOpts.ColorScale.ScaleWithColor(color.RGBA{R: 224, G: 234, B: 255, A: 255})
		text.Draw(screen, preview.label, mainFont, labelOpts)
		releaseTextDrawOpts(labelOpts)
	}
}
