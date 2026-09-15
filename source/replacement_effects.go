package main

import (
	_ "embed"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Coin rewards are individual denomination sprites, not animation frames.
// 1842 is 0, 1843 is 1, through 1851 for 9.
const (
	coinRewardFirstPictID uint16 = 1842
	coinRewardLastPictID  uint16 = 1851
)

//go:embed data/shaders/healing_burst.kage
var healingBurstShaderSource []byte

//go:embed data/shaders/fire_plume.kage
var firePlumeShaderSource []byte

//go:embed data/shaders/blood_gush.kage
var bloodGushShaderSource []byte

//go:embed data/shaders/waving_flag.kage
var wavingFlagShaderSource []byte

//go:embed data/shaders/wall_torch.kage
var wallTorchShaderSource []byte

//go:embed data/shaders/hidden_path.kage
var hiddenPathShaderSource []byte

//go:embed data/shaders/mystic_ward.kage
var mysticWardShaderSource []byte

//go:embed data/shaders/mystic_orbit_ward.kage
var mysticOrbitWardShaderSource []byte

//go:embed data/shaders/mystic_fade.kage
var mysticFadeShaderSource []byte

//go:embed data/shaders/teleport_burst.kage
var teleportBurstShaderSource []byte

//go:embed data/shaders/stone_form.kage
var stoneFormShaderSource []byte

//go:embed data/shaders/lava_pool.kage
var lavaPoolShaderSource []byte

//go:embed data/shaders/magic_mote_ring.kage
var magicMoteRingShaderSource []byte

//go:embed data/shaders/town_puddle.kage
var townPuddleShaderSource []byte

//go:embed data/shaders/shore_wave.kage
var shoreWaveShaderSource []byte

//go:embed data/shaders/coin_reward.kage
var coinRewardShaderSource []byte

var healingBurstShader *ebiten.Shader
var firePlumeShader *ebiten.Shader
var bloodGushShader *ebiten.Shader
var wavingFlagShader *ebiten.Shader
var wallTorchShader *ebiten.Shader
var hiddenPathShader *ebiten.Shader
var mysticWardShader *ebiten.Shader
var mysticOrbitWardShader *ebiten.Shader
var mysticFadeShader *ebiten.Shader
var teleportBurstShader *ebiten.Shader
var stoneFormShader *ebiten.Shader
var lavaPoolShader *ebiten.Shader
var magicMoteRingShader *ebiten.Shader
var townPuddleShader *ebiten.Shader
var shoreWaveShader *ebiten.Shader
var coinRewardShader *ebiten.Shader
var replacementEffectsStarted = time.Now()
var replacementEffectsPreview bool

type replacementEffectPreviewMode uint8

const (
	replacementEffectPreviewNew replacementEffectPreviewMode = iota
	replacementEffectPreviewOriginal
	replacementEffectPreviewBoth
)

var replacementEffectsPreviewMode = replacementEffectPreviewNew
var replacementEffectsPreviewSelection = -1 // -1 displays the complete gallery.

type replacementEffectPreviewScale uint8

const (
	replacementEffectPreviewNativeSize replacementEffectPreviewScale = iota
	replacementEffectPreviewDoubleSize
	replacementEffectPreviewFullWindow
)

var replacementEffectsPreviewScale = replacementEffectPreviewFullWindow
var replacementEffectsPreviewUPS = 5
var replacementEffectsShadersReady bool
var replacementEffectsShaderInitAttempted bool
var replacementEffectsShaderInitIndex int
var replacementEffectsShaderInitMu sync.Mutex

// An empty directory resolves beside this source file. Tests can override it
// to exercise reload behavior without touching a user's data directory.
var replacementEffectsShaderSourceDir string

type replacementEffectKind uint8

const (
	replacementEffectHealing replacementEffectKind = iota + 1
	replacementEffectFirePlume
	replacementEffectBloodGush
	replacementEffectWavingFlag
	replacementEffectWallTorch
	replacementEffectHiddenPath
	replacementEffectMysticWard
	replacementEffectMysticOrbitWard
	replacementEffectMysticFade
	replacementEffectTeleportGold
	replacementEffectTeleportBlue
	replacementEffectTeleportPrismatic
	replacementEffectStoneForm
	replacementEffectLavaPool
	replacementEffectMagicMoteRing
	replacementEffectTownPuddle
	replacementEffectShoreWave
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
	drawnInline                 bool
	mobileIndex                 uint8
	hasMobileAnchor             bool
	mobileOffsetX               float64
	mobileOffsetY               float64
	maskImage                   *ebiten.Image
	maskOffsetX                 float32
	maskOffsetY                 float32
	maskInvScale                float32
	hasMask                     bool
	puddleFeet                  map[uint8]townPuddleFootTrack
	puddleRipples               [6]townPuddleRippleEvent
	puddleRippleNext            int
}

type townPuddleFootTrack struct {
	anchorX, anchorY float32
	logicalFrame     int
	lastSeen         time.Time
}

type townPuddleRippleEvent struct {
	x, y     float32
	strength float32
	seed     float32
	started  time.Time
}

var replacementEffectDraws = make(map[uint64]replacementEffectDraw)
var replacementEffectNextKey uint64

type replacementEffectShaderState struct {
	op           ebiten.DrawTrianglesShaderOptions
	uniforms     map[string]any
	size         [2]float32
	canvasSize   [2]float32
	maskOffset   [2]float32
	outlineSize  [2]float32
	coinDigits   [4]float32
	rippleFeet   [12]float32
	rippleMotion [6]float32
	rippleSeeds  [6]float32
	rippleAges   [6]float32
}

var replacementEffectShaderStates [replacementEffectCoinReward + 1]replacementEffectShaderState
var replacementEffectShaderIndices = []uint32{0, 1, 2, 1, 2, 3}

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
			"HasPoolOutline":  float32(0),
			"OutlineSize":     state.outlineSize[:],
			"FlagTheme":       float32(0),
			"FlagMirror":      float32(0),
			"TorchMirror":     float32(0),
			"PathVariant":     float32(0),
			"MagicTheme":      float32(0),
			"FireTheme":       float32(0),
			"PuddleVariant":   float32(0),
			"HasReflection":   float32(0),
			"WaveDirection":   float32(0),
		}
		if kind == replacementEffectTownPuddle {
			state.uniforms["RippleFeet"] = state.rippleFeet[:]
			state.uniforms["RippleMotion"] = state.rippleMotion[:]
			state.uniforms["RippleSeeds"] = state.rippleSeeds[:]
			state.uniforms["RippleAges"] = state.rippleAges[:]
			state.uniforms["RippleTime"] = float32(0)
		}
		if kind == replacementEffectFirePlume || kind == replacementEffectBloodGush || replacementEffectTeleportTheme(kind) >= 0 {
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
	screen.DrawTrianglesShader32(vertices[:], replacementEffectShaderIndices, shader, &state.op)
}

const (
	replacementEffectFadeIn  = 180 * time.Millisecond
	replacementEffectFadeOut = 280 * time.Millisecond
)

// ReloadReplacementEffectsShader recompiles replacement shaders from the
// checked-out source tree. Embedded source remains the release fallback.
func ReloadReplacementEffectsShader() error {
	replacementEffectsShaderInitMu.Lock()
	defer replacementEffectsShaderInitMu.Unlock()

	healingShader, err := compileReplacementEffectShaderForInit("healing_burst.kage", healingBurstShaderSource)
	if err != nil {
		return err
	}
	fireShader, err := compileReplacementEffectShaderForInit("fire_plume.kage", firePlumeShaderSource)
	if err != nil {
		return err
	}
	flagShader, err := compileReplacementEffectShaderForInit("waving_flag.kage", wavingFlagShaderSource)
	if err != nil {
		return err
	}
	torchShader, err := compileReplacementEffectShaderForInit("wall_torch.kage", wallTorchShaderSource)
	if err != nil {
		return err
	}
	pathShader, err := compileReplacementEffectShaderForInit("hidden_path.kage", hiddenPathShaderSource)
	if err != nil {
		return err
	}
	wardShader, err := compileReplacementEffectShaderForInit("mystic_ward.kage", mysticWardShaderSource)
	if err != nil {
		return err
	}
	orbitWardShader, err := compileReplacementEffectShaderForInit("mystic_orbit_ward.kage", mysticOrbitWardShaderSource)
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
	lavaShader, err := compileReplacementEffectShaderForInit("lava_pool.kage", lavaPoolShaderSource)
	if err != nil {
		return err
	}
	magicRingShader, err := compileReplacementEffectShaderForInit("magic_mote_ring.kage", magicMoteRingShaderSource)
	if err != nil {
		return err
	}
	puddleShader, err := compileReplacementEffectShaderForInit("town_puddle.kage", townPuddleShaderSource)
	if err != nil {
		return err
	}
	waveShader, err := compileReplacementEffectShaderForInit("shore_wave.kage", shoreWaveShaderSource)
	if err != nil {
		return err
	}
	coinShader, err := compileReplacementEffectShaderForInit("coin_reward.kage", coinRewardShaderSource)
	if err != nil {
		return err
	}
	bloodShader, err := compileReplacementEffectShaderForInit("blood_gush.kage", bloodGushShaderSource)
	if err != nil {
		return err
	}
	healingBurstShader = healingShader
	firePlumeShader = fireShader
	wavingFlagShader = flagShader
	wallTorchShader = torchShader
	hiddenPathShader = pathShader
	mysticWardShader = wardShader
	mysticOrbitWardShader = orbitWardShader
	mysticFadeShader = fadeShader
	teleportBurstShader = teleportShader
	stoneFormShader = stoneShader
	lavaPoolShader = lavaShader
	magicMoteRingShader = magicRingShader
	townPuddleShader = puddleShader
	shoreWaveShader = waveShader
	coinRewardShader = coinShader
	bloodGushShader = bloodShader
	replacementEffectsShadersReady = true
	replacementEffectsShaderInitAttempted = true
	replacementEffectsShaderInitIndex = replacementEffectsShaderCount
	return nil
}

const replacementEffectsShaderCount = 16

func replacementEffectShaderSourcePath(name string) string {
	dir := replacementEffectsShaderSourceDir
	if dir == "" {
		_, sourceFile, _, ok := runtime.Caller(0)
		if !ok {
			return ""
		}
		dir = filepath.Join(filepath.Dir(sourceFile), "data", "shaders")
	}
	return filepath.Join(dir, name)
}

func compileReplacementEffectShader(name string, embedded []byte) (*ebiten.Shader, error) {
	source := embedded
	// Shader reload is deliberately a source-tree development feature. Never
	// read user data here: user assets must not silently alter client rendering.
	if path := replacementEffectShaderSourcePath(name); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			source = b
		}
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
		shader, err = compileReplacementEffectShaderForInit("fire_plume.kage", firePlumeShaderSource)
		if err == nil {
			firePlumeShader = shader
		}
	case 2:
		shader, err = compileReplacementEffectShaderForInit("waving_flag.kage", wavingFlagShaderSource)
		if err == nil {
			wavingFlagShader = shader
		}
	case 3:
		shader, err = compileReplacementEffectShaderForInit("wall_torch.kage", wallTorchShaderSource)
		if err == nil {
			wallTorchShader = shader
		}
	case 4:
		shader, err = compileReplacementEffectShaderForInit("hidden_path.kage", hiddenPathShaderSource)
		if err == nil {
			hiddenPathShader = shader
		}
	case 5:
		shader, err = compileReplacementEffectShaderForInit("mystic_ward.kage", mysticWardShaderSource)
		if err == nil {
			mysticWardShader = shader
		}
	case 6:
		shader, err = compileReplacementEffectShaderForInit("mystic_orbit_ward.kage", mysticOrbitWardShaderSource)
		if err == nil {
			mysticOrbitWardShader = shader
		}
	case 7:
		shader, err = compileReplacementEffectShaderForInit("mystic_fade.kage", mysticFadeShaderSource)
		if err == nil {
			mysticFadeShader = shader
		}
	case 8:
		shader, err = compileReplacementEffectShaderForInit("teleport_burst.kage", teleportBurstShaderSource)
		if err == nil {
			teleportBurstShader = shader
		}
	case 9:
		shader, err = compileReplacementEffectShaderForInit("stone_form.kage", stoneFormShaderSource)
		if err == nil {
			stoneFormShader = shader
		}
	case 10:
		shader, err = compileReplacementEffectShaderForInit("lava_pool.kage", lavaPoolShaderSource)
		if err == nil {
			lavaPoolShader = shader
		}
	case 11:
		shader, err = compileReplacementEffectShaderForInit("magic_mote_ring.kage", magicMoteRingShaderSource)
		if err == nil {
			magicMoteRingShader = shader
		}
	case 12:
		shader, err = compileReplacementEffectShaderForInit("town_puddle.kage", townPuddleShaderSource)
		if err == nil {
			townPuddleShader = shader
		}
	case 13:
		shader, err = compileReplacementEffectShaderForInit("shore_wave.kage", shoreWaveShaderSource)
		if err == nil {
			shoreWaveShader = shader
		}
	case 14:
		shader, err = compileReplacementEffectShaderForInit("coin_reward.kage", coinRewardShaderSource)
		if err == nil {
			coinRewardShader = shader
		}
	case 15:
		shader, err = compileReplacementEffectShaderForInit("blood_gush.kage", bloodGushShaderSource)
		if err == nil {
			bloodGushShader = shader
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
		effect.drawnInline = false
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
	switch id {
	case 1759, 1760:
		return replacementEffectHealing, true
	case 481, 482, 572, 1739, 1740, 1741:
		return replacementEffectFirePlume, true
	case 33:
		return replacementEffectBloodGush, true
	case 885, 886, 887, 5645, 5646, 5647:
		return replacementEffectWavingFlag, true
	case 330, 331:
		return replacementEffectWallTorch, true
	case 445, 446:
		return replacementEffectHiddenPath, true
	case 1286:
		return replacementEffectMysticWard, true
	case 1587:
		return replacementEffectMysticOrbitWard, true
	case 597, 598:
		return replacementEffectLavaPool, true
	case 1039, 1040, 1041, 1042, 1043, 1044:
		return replacementEffectMagicMoteRing, true
	case 888, 889:
		return replacementEffectTownPuddle, true
	case 3568, 3569, 3570, 3571:
		return replacementEffectShoreWave, true
	case 2976:
		return replacementEffectTeleportGold, true
	case 2977:
		return replacementEffectTeleportBlue, true
	case 2978:
		return replacementEffectTeleportPrismatic, true
	case 3125:
		return replacementEffectStoneForm, true
	case coinRewardFirstPictID, 1843, 1844, 1845, 1846, 1847, 1848, 1849, 1850, coinRewardLastPictID:
		return replacementEffectCoinReward, true
	}
	return 0, false
}

func replacementEffectShader(kind replacementEffectKind) *ebiten.Shader {
	if kind == replacementEffectFirePlume {
		return firePlumeShader
	}
	if kind == replacementEffectBloodGush {
		return bloodGushShader
	}
	if kind == replacementEffectWavingFlag {
		return wavingFlagShader
	}
	if kind == replacementEffectWallTorch {
		return wallTorchShader
	}
	if kind == replacementEffectHiddenPath {
		return hiddenPathShader
	}
	if kind == replacementEffectMysticWard {
		return mysticWardShader
	}
	if kind == replacementEffectMysticOrbitWard {
		return mysticOrbitWardShader
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
	if kind == replacementEffectLavaPool {
		return lavaPoolShader
	}
	if kind == replacementEffectMagicMoteRing {
		return magicMoteRingShader
	}
	if kind == replacementEffectTownPuddle {
		return townPuddleShader
	}
	if kind == replacementEffectShoreWave {
		return shoreWaveShader
	}
	if kind == replacementEffectCoinReward {
		return coinRewardShader
	}
	return healingBurstShader
}

func replacementEffectIsOneShot(kind replacementEffectKind) bool {
	return kind == replacementEffectFirePlume || kind == replacementEffectBloodGush || kind == replacementEffectMysticWard || kind == replacementEffectMysticFade || replacementEffectTeleportTheme(kind) >= 0
}

// Persistent overlays, such as flags carried by a player, are visible for as
// long as their source picture is present. Unlike a spell burst, they should
// neither ease in nor linger at a stale position after that picture is gone.
func replacementEffectIsPersistent(kind replacementEffectKind) bool {
	return kind == replacementEffectWavingFlag || kind == replacementEffectWallTorch || kind == replacementEffectMysticOrbitWard || kind == replacementEffectLavaPool || kind == replacementEffectMagicMoteRing || kind == replacementEffectTownPuddle || kind == replacementEffectShoreWave
}

func replacementEffectUsesPlayerMask(kind replacementEffectKind) bool {
	return kind != replacementEffectLavaPool && kind != replacementEffectTownPuddle && kind != replacementEffectShoreWave
}

func replacementEffectDrawsBelowMobiles(kind replacementEffectKind) bool {
	return kind == replacementEffectLavaPool || kind == replacementEffectTownPuddle || kind == replacementEffectShoreWave
}

func replacementEffectPictureDrawsBelowMobiles(pictID uint16) bool {
	if !replacementEffectReplacesPict(pictID) {
		return false
	}
	kind, _ := replacementEffectKindForPict(pictID)
	return replacementEffectDrawsBelowMobiles(kind)
}

// replacementEffectFramePhase maps a legacy sprite frame to a position in the
// procedural effect's authored timeline. It is used once, when an effect is
// created, to seed a smooth animation at the matching source-frame phase.
func replacementEffectFramePhase(frame, frames int, duration float32) float32 {
	if frames < 2 || duration <= 0 {
		return 0
	}
	frame %= frames
	if frame < 0 {
		frame += frames
	}
	return duration * float32(frame) / float32(frames-1)
}

func replacementEffectSequenceDuration(kind replacementEffectKind) float32 {
	if kind == replacementEffectFirePlume {
		return 1.35
	}
	if kind == replacementEffectMysticOrbitWard || kind == replacementEffectLavaPool || kind == replacementEffectMagicMoteRing || kind == replacementEffectTownPuddle || kind == replacementEffectShoreWave {
		if kind == replacementEffectTownPuddle {
			return 3.2
		}
		if kind == replacementEffectShoreWave {
			return 12.8
		}
		if kind == replacementEffectLavaPool {
			return 4.8
		}
		return 0.8
	}
	return 1
}

func replacementEffectFrameStartOffset(kind replacementEffectKind, pictID uint16, frame int) time.Duration {
	if kind == replacementEffectWavingFlag || kind == replacementEffectWallTorch || kind == replacementEffectShoreWave || kind == replacementEffectCoinReward {
		return 0
	}
	frames := 1
	if clImages != nil {
		frames = clImages.NumFrames(uint32(pictID))
	}
	if frames < 2 {
		return 0
	}
	duration := replacementEffectSequenceDuration(kind)
	phase := replacementEffectFramePhase(frame, frames, duration)
	if kind == replacementEffectMysticOrbitWard || kind == replacementEffectLavaPool || kind == replacementEffectMagicMoteRing || kind == replacementEffectTownPuddle || kind == replacementEffectShoreWave {
		frame %= frames
		if frame < 0 {
			frame += frames
		}
		phase = duration * float32(frame) / float32(frames)
	}
	return time.Duration(float64(phase) * float64(time.Second))
}

func replacementEffectInstanceStartOffset(kind replacementEffectKind, key uint64) time.Duration {
	if kind != replacementEffectWallTorch {
		return 0
	}
	// Mix the stable picture/mobile key into a broad phase range. A deterministic
	// offset avoids synchronized flames without making them jump after redraws.
	value := key + 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	value ^= value >> 31
	fraction := float64(uint32(value>>32)) / float64(^uint32(0))
	return time.Duration(fraction * float64(4*time.Second))
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
	return kind == replacementEffectFirePlume || kind == replacementEffectBloodGush || kind == replacementEffectCoinReward || replacementEffectTeleportTheme(kind) >= 0
}

func replacementEffectFlagTheme(id uint16) float32 {
	switch id {
	case 885:
		return 0 // aqua
	case 886:
		return 1 // red
	case 887:
		return 2 // yellow
	case 5645:
		return 3 // violet
	case 5646:
		return 4 // white
	case 5647:
		return 5 // gold
	default:
		return 0
	}
}

func replacementEffectFlagMirror(id uint16) float32 {
	if id == 5645 || id == 5646 || id == 5647 {
		return 1
	}
	return 0
}

func replacementEffectWallTorchMirror(id uint16) float32 {
	if id == 330 {
		return 1
	}
	return 0
}

func replacementEffectPathVariant(id uint16) float32 {
	if id == 446 {
		return 1
	}
	return 0
}

func replacementEffectFireTheme(id uint16) float32 {
	switch id {
	case 1739:
		return 1
	case 1740:
		return 2
	case 1741:
		return 3
	default:
		return 0
	}
}

func replacementEffectMagicTheme(id uint16) float32 {
	if id >= 1039 && id <= 1044 {
		return float32(id - 1039)
	}
	return 0
}

func replacementEffectPuddleVariant(id uint16) float32 {
	if id == 889 {
		return 1
	}
	return 0
}

func replacementEffectGroundPictureInstanceKey(p framePicture) uint64 {
	// The picture matcher carries lightKey from frame to frame, including
	// retained edge pictures. Keep different puddle shapes distinct even when
	// their world anchors overlap.
	return uint64(p.PictID)<<33 | pictureLightInstanceKey(p)
}

func replacementEffectWorldKey(kind replacementEffectKind, h, v int16, instanceKey uint64) uint64 {
	if instanceKey != 0 {
		return instanceKey | uint64(kind)<<56
	}
	return uint64(uint16(h))<<16 | uint64(uint16(v)) | uint64(kind)<<48
}

func replacementEffectWaveDirection(id uint16) float32 {
	switch id {
	case 3571:
		return 1 // south
	case 3568:
		return 2 // east
	case 3569:
		return 3 // west
	default:
		return 0 // north
	}
}

// queueReplacementPictureEffect preserves the legacy effect's world anchor
// while deferring the visual to the procedural-effects world pass.
func queueReplacementPictureEffect(pictID uint16, frame int, h, v int16, instanceKey uint64, left, top, width, height float64, alpha float32, mobileImg *ebiten.Image, mobileX, mobileY, mobileSize float64) bool {
	if !replacementEffectReplacesPict(pictID) || width <= 0 || height <= 0 {
		return false
	}
	kind, _ := replacementEffectKindForPict(pictID)
	if kind == replacementEffectCoinReward {
		frame = int(pictID - coinRewardFirstPictID)
	}
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	// 1759/1760 healing and 481/482/572 fire are legacy alternate IDs for the
	// same families. Prefer the pinned mobile identity so an ID handoff or
	// movement does not restart the procedural effect; unpinned effects fall
	// back to their world position.
	var key uint64
	if kind == replacementEffectCoinReward {
		key = replacementCoinClusterKey(left, top, width, height, now)
	} else {
		key = replacementEffectWorldKey(kind, h, v, instanceKey)
	}
	effect, ok := replacementEffectDraws[key]
	if !ok || (!replacementEffectIsPersistent(kind) && now.Sub(effect.lastSeen) > replacementEffectFadeOut) {
		startOffset := replacementEffectFrameStartOffset(kind, pictID, frame) + replacementEffectInstanceStartOffset(kind, key)
		effect = replacementEffectDraw{pictID: pictID, kind: kind, started: now.Add(-startOffset)}
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
	effect.drawnInline = false
	// Coin rewards can appear in open world space as well as over a mobile.
	// Keep their native group position stable instead of letting a transient
	// nearest-mobile choice split or move the digits between updates.
	effect.hasMobileAnchor = replacementEffectUsesPlayerMask(kind) && kind != replacementEffectCoinReward && kind != replacementEffectWallTorch && instanceKey != 0
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
	drawReplacementEffectsLayer(screen, ox, oy, mobiles, prevMobiles, shiftX, shiftY, alpha, false)
}

func drawReplacementEffectsBelowMobiles(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, descriptors map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, logicalFrame int, viewport *viewportRenderState) {
	drawReplacementEffectsLayerWithPuddleReflections(screen, ox, oy, mobiles, descriptors, prevMobiles, shiftX, shiftY, alpha, logicalFrame, true, viewport)
}

func drawReplacementEffectsLayer(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, belowMobiles bool) {
	drawReplacementEffectsLayerWithPuddleReflections(screen, ox, oy, mobiles, nil, prevMobiles, shiftX, shiftY, alpha, 0, belowMobiles, nil)
}

func drawReplacementEffectsLayerWithPuddleReflections(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, descriptors map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, logicalFrame int, belowMobiles bool, viewport *viewportRenderState) {
	drawReplacementEffectsLayerSelected(screen, ox, oy, mobiles, descriptors, prevMobiles, shiftX, shiftY, alpha, logicalFrame, belowMobiles, viewport, 0)
}

// Negative-plane floor pictures draw at their exact place among scenery. The
// later floor pass must not paint the same shader a second time over pictures
// that followed it in the source order.
func drawReplacementEffectInlineGround(screen *ebiten.Image, key uint64, ox, oy int, mobiles []frameMobile, descriptors map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, logicalFrame int, viewport *viewportRenderState) {
	drawReplacementEffectsLayerSelected(screen, ox, oy, mobiles, descriptors, prevMobiles, shiftX, shiftY, alpha, logicalFrame, true, viewport, key)
}

func drawReplacementEffectsLayerSelected(screen *ebiten.Image, ox, oy int, mobiles []frameMobile, descriptors map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, logicalFrame int, belowMobiles bool, viewport *viewportRenderState, onlyKey uint64) {
	if !replacementEffectsEnabled() {
		return
	}
	if belowMobiles && !gs.hideMobiles && len(descriptors) != 0 {
		maxWidth, maxHeight := 0, 0
		if onlyKey != 0 {
			effect := replacementEffectDraws[onlyKey]
			if effect.kind == replacementEffectTownPuddle && effect.seen {
				maxWidth = int(math.Ceil(effect.width))
				maxHeight = int(math.Ceil(effect.height))
			}
		} else {
			for _, effect := range replacementEffectDraws {
				if effect.kind != replacementEffectTownPuddle || !effect.seen {
					continue
				}
				maxWidth = max(maxWidth, int(math.Ceil(effect.width)))
				maxHeight = max(maxHeight, int(math.Ceil(effect.height)))
			}
		}
		if maxWidth > 0 && maxHeight > 0 {
			scratch := &townPuddleReflectionTmp
			if viewport != nil {
				scratch = &viewport.puddleReflectionTmp
			}
			ensureTownPuddleReflectionScratch(scratch, maxWidth, maxHeight)
		}
	}
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	globalPhase := float32(now.Sub(replacementEffectsStarted).Seconds())
	drawEffect := func(key uint64, effect replacementEffectDraw) {
		if onlyKey == 0 && effect.drawnInline {
			return
		}
		if replacementEffectDrawsBelowMobiles(effect.kind) != belowMobiles {
			return
		}
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
		if replacementEffectIsPersistent(effect.kind) {
			fadeIn = 1
		}
		if effect.kind == replacementEffectCoinReward {
			// Reward digits may exist for only one server update. Draw them at
			// full strength immediately; the normal fade-out still softens exit.
			fadeIn = 1
		}
		fadeOut := float32(1)
		if !effect.seen {
			if replacementEffectIsPersistent(effect.kind) {
				delete(replacementEffectDraws, key)
				return
			}
			age := now.Sub(effect.lastSeen)
			if age >= replacementEffectFadeOut {
				delete(replacementEffectDraws, key)
				return
			}
			fadeOut = 1 - replacementEffectEase(float32(age)/float32(replacementEffectFadeOut))
		}
		energy := fadeIn * fadeOut
		if energy <= 0 {
			return
		}
		if onlyKey != 0 {
			effect.drawnInline = true
			replacementEffectDraws[key] = effect
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
			return
		}
		phase := globalPhase
		if effect.kind != replacementEffectWavingFlag && effect.kind != replacementEffectShoreWave && effect.kind != replacementEffectCoinReward {
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
		state.uniforms["HasReflection"] = float32(0)
		state.uniforms["HasPoolOutline"] = float32(0)
		state.uniforms["MaskInvScale"] = effect.maskInvScale
		if effect.kind == replacementEffectWavingFlag {
			state.uniforms["FlagTheme"] = replacementEffectFlagTheme(effect.pictID)
			state.uniforms["FlagMirror"] = replacementEffectFlagMirror(effect.pictID)
		}
		if effect.kind == replacementEffectWallTorch {
			state.uniforms["TorchMirror"] = replacementEffectWallTorchMirror(effect.pictID)
		}
		if effect.kind == replacementEffectHiddenPath {
			state.uniforms["PathVariant"] = replacementEffectPathVariant(effect.pictID)
		}
		if effect.kind == replacementEffectFirePlume {
			state.uniforms["FireTheme"] = replacementEffectFireTheme(effect.pictID)
		}
		if effect.kind == replacementEffectTownPuddle {
			state.uniforms["PuddleVariant"] = replacementEffectPuddleVariant(effect.pictID)
			state.rippleFeet = [12]float32{}
			state.rippleMotion = [6]float32{}
			state.rippleSeeds = [6]float32{}
			state.rippleAges = [6]float32{}
			state.uniforms["RippleTime"] = globalPhase
		}
		if effect.kind == replacementEffectShoreWave {
			state.uniforms["WaveDirection"] = replacementEffectWaveDirection(effect.pictID)
		}
		if effect.kind == replacementEffectMagicMoteRing {
			state.uniforms["MagicTheme"] = replacementEffectMagicTheme(effect.pictID)
		}
		if theme := replacementEffectTeleportTheme(effect.kind); theme >= 0 {
			state.uniforms["TeleportTheme"] = theme
		}
		if effect.kind == replacementEffectCoinReward {
			state.uniforms["CoinValue"] = float32(effect.frame % 10)
			state.uniforms["CoinDigitCount"] = float32(effect.coinDigitCount)
		}
		state.op.Images[0], state.op.Images[1] = effect.maskImage, nil
		if effect.kind == replacementEffectLavaPool {
			if outline := loadImageFrameOriginal(effect.pictID, 0); outline != nil {
				bounds := outline.Bounds()
				state.outlineSize = [2]float32{float32(bounds.Dx()), float32(bounds.Dy())}
				state.uniforms["HasPoolOutline"] = float32(1)
				state.op.Images[0] = outline
			}
		}
		if effect.kind == replacementEffectTownPuddle && !gs.hideMobiles && len(descriptors) != 0 {
			surface := drawTownPuddleMobileReflections(&effect, w, h, ox, oy, mobiles, descriptors, prevMobiles, shiftX, shiftY, alpha, logicalFrame, now, viewport)
			populateTownPuddleRippleUniforms(&effect, now, state)
			replacementEffectDraws[key] = effect
			if reflection := surface.image; reflection != nil {
				// Kage pixel-unit shaders require every bound source to have the same
				// size. The puddle uses image 0 only for local coordinates, so the
				// reflection texture can safely occupy both slots.
				state.op.Images[0], state.op.Images[1] = reflection, reflection
				state.uniforms["HasReflection"] = float32(1)
			}
		}
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
	if onlyKey != 0 {
		if effect, ok := replacementEffectDraws[onlyKey]; ok {
			drawEffect(onlyKey, effect)
		}
		return
	}
	for key, effect := range replacementEffectDraws {
		drawEffect(key, effect)
	}
}

var townPuddleReflectionTmp *ebiten.Image

type townPuddleMobileSurface struct {
	image *ebiten.Image
}

func drawTownPuddleMobileReflections(effect *replacementEffectDraw, width, height, ox, oy int, mobiles []frameMobile, descriptors map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64, logicalFrame int, now time.Time, viewport *viewportRenderState) townPuddleMobileSurface {
	var surface townPuddleMobileSurface
	if width <= 0 || height <= 0 || len(mobiles) == 0 {
		return surface
	}
	var scratch **ebiten.Image = &townPuddleReflectionTmp
	if viewport != nil {
		scratch = &viewport.puddleReflectionTmp
	}
	reflection := *scratch
	if reflection == nil {
		return surface
	}
	reflection.Clear()
	reflectionCount := 0
	for _, mobile := range mobiles {
		desc, ok := descriptors[mobile.Index]
		if !ok || desc.PictID == 0 || mobile.State == poseDead {
			continue
		}
		mobileX, mobileY := mobileScreenPositionFloat(ox, oy, mobile, prevMobiles, shiftX, shiftY, alpha, maxMobileInterpPixels)
		size := mobileSize(desc.PictID)
		if size <= 0 {
			continue
		}
		drawSize := float64(roundToInt(float64(size) * gs.GameScale))
		if drawSize <= 0 {
			continue
		}
		centerX := effect.left + effect.width/2
		centerY := effect.top + effect.height/2
		// Before loading the pose, reject mobiles whose approximate contact
		// point cannot reach the puddle. The exact opaque foot row follows.
		approximateFootY := mobileY + drawSize*0.40
		if math.Abs(mobileX-centerX) >= effect.width/2+drawSize*0.25 || math.Abs(approximateFootY-centerY) >= effect.height/2+drawSize*0.65 {
			continue
		}
		pose, ok := loadMobileReflectionPose(desc, mobile.State)
		if !ok {
			continue
		}
		footFraction := pose.footRow
		footY := mobileY - drawSize/2 + drawSize*footFraction
		proximity := townPuddleReflectionProximity(mobileX-centerX, footY-centerY, effect.width, effect.height, drawSize)
		if proximity <= 0 {
			continue
		}
		localFootX, localFootY := float32(mobileX-effect.left), float32(footY-effect.top)
		localAnchorX, localAnchorY := float32(mobileX-effect.left), float32(mobileY-effect.top)
		effect.recordTownPuddleFoot(mobile.Index, logicalFrame, localAnchorX, localAnchorY, localFootX, localFootY, float32(proximity), townPuddleMobileMotion(mobile, prevMobiles, shiftX, shiftY), now)
		if reflectionCount == 3 {
			continue
		}
		// The camera looks down onto a ground plane. The mobile's ground point
		// moves the reflection in the same screen-space direction; only its
		// artwork is flipped and foreshortened into the shallow puddle. The
		// shared reflection draw trims and softens the join at the foot row.
		options := frameBlendDrawOptions{
			Left:   mobileX - drawSize/2 - effect.left,
			Top:    townPuddleReflectionVerticalTop(footY, effect.top, float64(height), footFraction) - mobileReflectionFootOverlap(drawSize, float64(height)*0.82),
			ScaleX: drawSize / float64(pose.image.Bounds().Dx()),
			ScaleY: -float64(height) * 0.82 / float64(pose.image.Bounds().Dy()),
			Red:    1, Green: 1, Blue: 1, Alpha: float32(0.80 * proximity),
			Linear: worldArtworkFilter() == ebiten.FilterLinear,
		}
		drawMobileReflectionSprite(reflection, pose, options)
		reflectionCount++
	}
	if reflectionCount > 0 {
		surface.image = reflection
	}
	return surface
}

func (effect *replacementEffectDraw) addTownPuddleRipple(x, y, strength float32, mobileIndex uint8, now time.Time) {
	index := effect.puddleRippleNext % len(effect.puddleRipples)
	effect.puddleRipples[index] = townPuddleRippleEvent{
		x: x, y: y, strength: strength,
		seed:    float32(mobileIndex) * 0.6180339,
		started: now,
	}
	effect.puddleRippleNext++
}

func (effect *replacementEffectDraw) recordTownPuddleFoot(mobileIndex uint8, logicalFrame int, anchorX, anchorY, footX, footY, proximity, frameMotion float32, now time.Time) {
	if effect.puddleFeet == nil {
		effect.puddleFeet = make(map[uint8]townPuddleFootTrack)
	}
	previous, hadPrevious := effect.puddleFeet[mobileIndex]
	if hadPrevious && now.Sub(previous.lastSeen) <= 350*time.Millisecond && previous.logicalFrame != logicalFrame {
		dx, dy := float64(anchorX-previous.anchorX), float64(anchorY-previous.anchorY)
		distance := math.Hypot(dx, dy)
		scale := max(1, gs.GameScale)
		if (frameMotion > 0 || distance >= 0.9*scale) && distance <= maxMobileInterpPixels*scale {
			strength := max(0.75, frameMotion, float32(math.Min(1, distance/(8*scale)))) * proximity
			effect.addTownPuddleRipple(footX, footY, strength, mobileIndex, now)
		}
	} else if (!hadPrevious || now.Sub(previous.lastSeen) > 350*time.Millisecond) && frameMotion > 0 {
		effect.addTownPuddleRipple(footX, footY, max(0.75, frameMotion)*proximity, mobileIndex, now)
	}
	effect.puddleFeet[mobileIndex] = townPuddleFootTrack{
		anchorX: anchorX, anchorY: anchorY, logicalFrame: logicalFrame, lastSeen: now,
	}
}

func populateTownPuddleRippleUniforms(effect *replacementEffectDraw, now time.Time, state *replacementEffectShaderState) {
	for i, ripple := range effect.puddleRipples {
		age := now.Sub(ripple.started)
		if ripple.started.IsZero() || age < 0 || age >= time.Second {
			continue
		}
		state.rippleFeet[i*2], state.rippleFeet[i*2+1] = ripple.x, ripple.y
		state.rippleMotion[i] = ripple.strength
		state.rippleSeeds[i] = ripple.seed
		state.rippleAges[i] = float32(age.Seconds())
	}
}

func townPuddleMobileMotion(mobile frameMobile, previous map[uint8]frameMobile, shiftX, shiftY int) float32 {
	prev, ok := previous[mobile.Index]
	if !ok {
		return 0
	}
	dh := int(mobile.H) - int(prev.H) - shiftX
	dv := int(mobile.V) - int(prev.V) - shiftY
	distanceSquared := dh*dh + dv*dv
	if distanceSquared == 0 || distanceSquared > maxMobileInterpPixels*maxMobileInterpPixels {
		return 0
	}
	return float32(math.Min(1, math.Sqrt(float64(distanceSquared))/12))
}

func ensureTownPuddleReflectionScratch(scratch **ebiten.Image, width, height int) *ebiten.Image {
	if *scratch == nil || (*scratch).Bounds().Dx() < width || (*scratch).Bounds().Dy() < height {
		if *scratch != nil {
			(*scratch).Deallocate()
		}
		*scratch = ebiten.NewImageWithOptions(image.Rect(0, 0, width, height), &ebiten.NewImageOptions{Unmanaged: true})
	}
	return *scratch
}

func townPuddleReflectionProximity(dx, dy, puddleWidth, puddleHeight, mobileSize float64) float64 {
	if puddleWidth <= 0 || puddleHeight <= 0 || mobileSize <= 0 {
		return 0
	}
	xReach := puddleWidth/2 + mobileSize*0.25
	yReach := puddleHeight/2 + mobileSize*0.45
	return math.Max(0, 1-math.Max(math.Abs(dx)/xReach, math.Abs(dy)/yReach))
}

func townPuddleReflectionVerticalTop(footY, puddleTop, puddleHeight, footFraction float64) float64 {
	reflectedHeight := puddleHeight * 0.82
	// With the source flipped, its opaque foot row lands at Top minus this
	// amount. Put that row exactly at the upright sprite's contact point.
	return mobileReflectionTop(footY, reflectedHeight, footFraction) - puddleTop
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
	pictID         uint16
	coinDigits     [4]float32
	coinDigitCount int
}

var replacementEffectsPreviews = []replacementEffectPreview{
	{kind: replacementEffectHealing, label: "Healing", pictID: 1759},
	{kind: replacementEffectFirePlume, label: "Fire Plume", pictID: 481},
	{kind: replacementEffectFirePlume, label: "Violet Fire Plume", pictID: 1739},
	{kind: replacementEffectFirePlume, label: "Green Fire Plume", pictID: 1740},
	{kind: replacementEffectFirePlume, label: "Blue Fire Plume", pictID: 1741},
	{kind: replacementEffectBloodGush, label: "Critical Blood Gush", pictID: 33},
	{kind: replacementEffectWavingFlag, label: "Aqua Flag", pictID: 885},
	{kind: replacementEffectWavingFlag, label: "Red Flag", pictID: 886},
	{kind: replacementEffectWavingFlag, label: "Yellow Flag", pictID: 887},
	{kind: replacementEffectWavingFlag, label: "Violet Flag", pictID: 5645},
	{kind: replacementEffectWavingFlag, label: "White Flag", pictID: 5646},
	{kind: replacementEffectWavingFlag, label: "Gold Flag", pictID: 5647},
	{kind: replacementEffectWallTorch, label: "Wall Torch", pictID: 330},
	{kind: replacementEffectWallTorch, label: "Wall Torch (mirrored)", pictID: 331},
	{kind: replacementEffectHiddenPath, label: "Hidden Path Sparkles", pictID: 446},
	{kind: replacementEffectHiddenPath, label: "Hidden Path Fading", pictID: 445},
	{kind: replacementEffectMysticWard, label: "Mystic Ward", pictID: 1286},
	{kind: replacementEffectMysticOrbitWard, label: "Amber Orbit Ward", pictID: 1587},
	{kind: replacementEffectLavaPool, label: "Lava Pool (compact)", pictID: 597},
	{kind: replacementEffectLavaPool, label: "Lava Pool (large)", pictID: 598},
	{kind: replacementEffectMagicMoteRing, label: "Neutral Magic Ring", pictID: 1039},
	{kind: replacementEffectMagicMoteRing, label: "Green Magic Ring", pictID: 1040},
	{kind: replacementEffectMagicMoteRing, label: "Red Magic Ring", pictID: 1041},
	{kind: replacementEffectMagicMoteRing, label: "Magenta Magic Ring", pictID: 1042},
	{kind: replacementEffectMagicMoteRing, label: "Yellow Magic Ring", pictID: 1043},
	{kind: replacementEffectMagicMoteRing, label: "Cyan Magic Ring", pictID: 1044},
	{kind: replacementEffectTownPuddle, label: "Town Puddle (small)", pictID: 888},
	{kind: replacementEffectTownPuddle, label: "Town Puddle (large)", pictID: 889},
	{kind: replacementEffectShoreWave, label: "Shore Foam (east)", pictID: 3568},
	{kind: replacementEffectShoreWave, label: "Shore Foam (west)", pictID: 3569},
	{kind: replacementEffectShoreWave, label: "Shore Foam (north)", pictID: 3570},
	{kind: replacementEffectShoreWave, label: "Shore Foam (south)", pictID: 3571},
	{kind: replacementEffectMysticFade, label: "Ward Fading", pictID: 445},
	{kind: replacementEffectTeleportGold, label: "Gold Teleport", pictID: 2976},
	{kind: replacementEffectTeleportBlue, label: "Blue Teleport", pictID: 2977},
	{kind: replacementEffectTeleportPrismatic, label: "Prismatic Teleport", pictID: 2978},
	{kind: replacementEffectStoneForm, label: "Stone Form", pictID: 3125},
	{kind: replacementEffectCoinReward, label: "Coin 123", pictID: coinRewardFirstPictID, coinDigits: [4]float32{1, 2, 3}, coinDigitCount: 3},
}

func replacementEffectPreviewLabel(mode replacementEffectPreviewMode) string {
	switch mode {
	case replacementEffectPreviewOriginal:
		return "Original animation"
	case replacementEffectPreviewBoth:
		return "Original + new effect"
	default:
		return "New effect"
	}
}

// One-shot shaders consume Phase as their age and deliberately fade to
// transparent. In the preview they need a local loop rather than application
// uptime, otherwise opening the gallery after startup displays only their
// already-expired final state.
func replacementEffectPreviewPhase(kind replacementEffectKind, elapsed float64) float32 {
	switch kind {
	case replacementEffectHealing, replacementEffectWavingFlag, replacementEffectWallTorch, replacementEffectMysticOrbitWard, replacementEffectStoneForm, replacementEffectLavaPool, replacementEffectMagicMoteRing, replacementEffectTownPuddle, replacementEffectShoreWave, replacementEffectCoinReward:
		return float32(elapsed)
	default:
		return float32(math.Mod(elapsed, float64(replacementEffectSequenceDuration(kind))))
	}
}

func replacementEffectPreviewNativeDimensions(preview replacementEffectPreview) (int, int) {
	if preview.pictID != 0 && clImages != nil {
		width, height := clImages.Size(uint32(preview.pictID))
		frames := max(1, clImages.NumFrames(uint32(preview.pictID)))
		height /= frames
		if width > 0 && height > 0 {
			return width, height
		}
	}
	return 64, 64
}

func replacementEffectPreviewLavaBounds(left, top float64, width, height, nativeW, nativeH int) (float64, float64, int, int) {
	scale := math.Min(float64(width)/float64(nativeW), float64(height)/float64(nativeH))
	drawW := max(1, roundToInt(float64(nativeW)*scale))
	drawH := max(1, roundToInt(float64(nativeH)*scale))
	return left + float64(width-drawW)/2, top + float64(height-drawH)/2, drawW, drawH
}

func replacementEffectPreviewItems() []replacementEffectPreview {
	if replacementEffectsPreviewSelection >= 0 && replacementEffectsPreviewSelection < len(replacementEffectsPreviews) {
		return replacementEffectsPreviews[replacementEffectsPreviewSelection : replacementEffectsPreviewSelection+1]
	}
	return replacementEffectsPreviews
}

func drawReplacementEffectOriginalPreview(screen *ebiten.Image, preview replacementEffectPreview, left, top float64, width, height int, elapsed float64) {
	if preview.pictID == 0 || clImages == nil {
		return
	}
	frames := max(1, clImages.NumFrames(uint32(preview.pictID)))
	frame := int(elapsed*float64(max(1, replacementEffectsPreviewUPS))) % frames
	image := loadImageFrameOriginal(preview.pictID, frame)
	if image == nil {
		return
	}
	bounds := image.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	scale := math.Min(float64(width)/float64(bounds.Dx()), float64(height)/float64(bounds.Dy()))
	if scale <= 0 {
		return
	}
	op := acquireDrawOpts()
	op.Filter = ebiten.FilterNearest
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(left+(float64(width)-float64(bounds.Dx())*scale)/2, top+(float64(height)-float64(bounds.Dy())*scale)/2)
	screen.DrawImage(image, op)
	releaseDrawOpts(op)
}

// replacementEffectsPreviewGrassPattern is deliberately coordinate-based rather
// than random so the preview backdrop remains still while an effect animates.
func replacementEffectsPreviewGrassPattern(column, row int) uint32 {
	value := uint32(column)*0x9e3779b9 ^ uint32(row)*0x85ebca6b
	value ^= value >> 16
	value *= 0x7feb352d
	value ^= value >> 15
	return value
}

func drawReplacementEffectsPreviewGrass(screen *ebiten.Image, bounds image.Rectangle) {
	const tileSize = 20
	screen.Fill(color.RGBA{R: 45, G: 91, B: 48, A: 255})
	for top := bounds.Min.Y; top < bounds.Max.Y; top += tileSize {
		for left := bounds.Min.X; left < bounds.Max.X; left += tileSize {
			pattern := replacementEffectsPreviewGrassPattern(left/tileSize, top/tileSize)
			shade := uint8(pattern & 7)
			base := color.RGBA{R: 42 + shade, G: 94 + shade*2, B: 46 + shade, A: 255}
			vector.FillRect(screen, float32(left), float32(top), tileSize, tileSize, base, false)

			bladeX := left + 2 + int((pattern>>4)%15)
			bladeY := top + 4 + int((pattern>>8)%11)
			bladeHeight := 5 + int((pattern>>12)%7)
			blade := color.RGBA{R: 74 + shade, G: 132 + shade*2, B: 63 + shade, A: 170}
			vector.FillRect(screen, float32(bladeX), float32(bladeY), 1, float32(bladeHeight), blade, false)
			vector.FillRect(screen, float32(bladeX+3), float32(bladeY+3), 1, float32(max(3, bladeHeight-3)), blade, false)
		}
	}
}

// drawReplacementEffectsPreview renders either the full shader gallery or one
// enlarged effect. The selected display mode lets artists compare it directly
// with the source sprite without changing live world replacement behavior.
func drawReplacementEffectsPreview(screen *ebiten.Image) {
	if !replacementEffectsShadersReady {
		return
	}
	bounds := screen.Bounds()
	if bounds.Dx() < 240 || bounds.Dy() < 180 {
		return
	}
	drawReplacementEffectsPreviewGrass(screen, bounds)

	previews := replacementEffectPreviewItems()
	if len(previews) == 0 {
		return
	}
	columns := 3
	if len(previews) == 1 {
		columns = 1
	}
	cellW := float64(bounds.Dx()) / float64(columns)
	rows := (len(previews) + columns - 1) / columns
	cellH := float64(bounds.Dy()) / float64(rows)
	effectW := max(56, roundToInt(cellW*0.68))
	effectH := max(56, roundToInt(cellH*0.66))
	if len(previews) == 1 {
		switch replacementEffectsPreviewScale {
		case replacementEffectPreviewNativeSize:
			effectW, effectH = replacementEffectPreviewNativeDimensions(previews[0])
		case replacementEffectPreviewDoubleSize:
			effectW, effectH = replacementEffectPreviewNativeDimensions(previews[0])
			effectW *= 2
			effectH *= 2
		default:
			// Leave room for the preview title while otherwise using the game view.
			effectW = max(56, bounds.Dx()-32)
			effectH = max(56, bounds.Dy()-64)
		}
	}
	coinCanvasW, coinCanvasH := roundToInt(float64(effectW)*1.70), roundToInt(float64(effectH)*1.70)
	now := drawFrameNow
	if now.IsZero() {
		now = time.Now()
	}
	elapsed := now.Sub(replacementEffectsStarted).Seconds()
	for i, preview := range previews {
		col, row := i%columns, i/columns
		left := float64(bounds.Min.X) + float64(col)*cellW + (cellW-float64(effectW))/2
		top := float64(bounds.Min.Y) + float64(row)*cellH + 28
		phase := replacementEffectPreviewPhase(preview.kind, elapsed)
		drawW, drawH := effectW, effectH
		drawLeft, drawTop := left, top
		if preview.kind == replacementEffectLavaPool {
			nativeW, nativeH := replacementEffectPreviewNativeDimensions(preview)
			drawLeft, drawTop, drawW, drawH = replacementEffectPreviewLavaBounds(left, top, effectW, effectH, nativeW, nativeH)
		}
		if replacementEffectsPreviewMode != replacementEffectPreviewNew {
			drawReplacementEffectOriginalPreview(screen, preview, left, top, effectW, effectH, elapsed)
		}
		state := &replacementEffectShaderStates[preview.kind]
		state.size = [2]float32{float32(drawW), float32(drawH)}
		state.canvasSize = [2]float32{float32(drawW), float32(drawH)}
		state.maskOffset = [2]float32{}
		state.coinDigits = preview.coinDigits
		state.uniforms["Phase"] = phase
		state.uniforms["Alpha"] = float32(1)
		state.uniforms["Energy"] = float32(1)
		state.uniforms["HasMask"] = float32(0)
		state.uniforms["HasReflection"] = float32(0)
		state.uniforms["HasPoolOutline"] = float32(0)
		state.uniforms["MaskInvScale"] = float32(1)
		state.uniforms["SpriteLightOnly"] = float32(0)
		if preview.kind == replacementEffectWavingFlag {
			state.uniforms["FlagTheme"] = replacementEffectFlagTheme(preview.pictID)
			state.uniforms["FlagMirror"] = replacementEffectFlagMirror(preview.pictID)
		}
		if preview.kind == replacementEffectWallTorch {
			state.uniforms["TorchMirror"] = replacementEffectWallTorchMirror(preview.pictID)
		}
		if preview.kind == replacementEffectHiddenPath {
			state.uniforms["PathVariant"] = replacementEffectPathVariant(preview.pictID)
		}
		if preview.kind == replacementEffectFirePlume {
			state.uniforms["FireTheme"] = replacementEffectFireTheme(preview.pictID)
		}
		if preview.kind == replacementEffectTownPuddle {
			state.uniforms["PuddleVariant"] = replacementEffectPuddleVariant(preview.pictID)
			state.rippleFeet = [12]float32{}
			state.rippleMotion = [6]float32{}
			state.rippleSeeds = [6]float32{}
			state.rippleAges = [6]float32{}
			state.uniforms["RippleTime"] = float32(elapsed)
			// The gallery has no moving mobile; demonstrate one footfall so
			// puddle movement ripples can still be judged in effectsPreview.
			previewRippleAge := math.Mod(elapsed+float64(preview.pictID-888)*0.35, 1.25)
			if previewRippleAge < 1 {
				state.rippleFeet[0], state.rippleFeet[1] = float32(effectW)*0.5, float32(effectH)*0.5
				state.rippleMotion[0] = 0.9
				state.rippleAges[0] = float32(previewRippleAge)
			}
		}
		if preview.kind == replacementEffectShoreWave {
			state.uniforms["WaveDirection"] = replacementEffectWaveDirection(preview.pictID)
		}
		if preview.kind == replacementEffectMagicMoteRing {
			state.uniforms["MagicTheme"] = replacementEffectMagicTheme(preview.pictID)
		}
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
		if replacementEffectsPreviewMode != replacementEffectPreviewOriginal {
			state.op.Blend = ebiten.Blend{}
			state.op.Images[0], state.op.Images[1] = whiteImage, nil
			if preview.kind == replacementEffectLavaPool {
				if outline := loadImageFrameOriginal(preview.pictID, 0); outline != nil {
					outlineBounds := outline.Bounds()
					state.outlineSize = [2]float32{float32(outlineBounds.Dx()), float32(outlineBounds.Dy())}
					state.uniforms["HasPoolOutline"] = float32(1)
					state.op.Images[0] = outline
				}
			}
			drawReplacementEffectShader(screen, drawLeft, drawTop, drawW, drawH, replacementEffectShader(preview.kind), state)
		}

		labelOpts := acquireTextDrawOpts()
		labelOpts.GeoM.Translate(float64(bounds.Min.X)+float64(col)*cellW+8, float64(bounds.Min.Y)+float64(row)*cellH+8)
		labelOpts.ColorScale.ScaleWithColor(color.RGBA{R: 224, G: 234, B: 255, A: 255})
		label := preview.label
		if len(previews) == 1 {
			label += " — " + replacementEffectPreviewLabel(replacementEffectsPreviewMode)
		}
		text.Draw(screen, label, mainFont, labelOpts)
		releaseTextDrawOpts(labelOpts)
	}
}
