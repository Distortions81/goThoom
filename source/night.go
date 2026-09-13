package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"gothoom/climg"
	"image"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed data/images/night.png

var nightImage []byte

type NightInfo struct {
	mu              sync.Mutex
	BaseLevel       int
	Azimuth         int
	Cloudy          bool
	Flags           uint
	Level           int
	Shadows         int
	oldAzimuth      int
	redshift        float64
	startOfTwilight int
	generation      uint64
}

type nightRenderState struct {
	baseLevel, azimuth, level, shadows int
	cloudy                             bool
	flags                              uint
	redshift                           float64
	generation                         uint64
}

// resetNightState discards time-of-day information owned by the current live
// session or movie. A subsequent source must establish its own lighting state.
func resetNightState() {
	primarySession.night.reset()
}

func (n *NightInfo) reset() {
	if n == nil {
		return
	}
	n.mu.Lock()
	n.BaseLevel = 0
	n.Azimuth = 0
	n.Cloudy = false
	n.Flags = 0
	n.Level = 0
	n.Shadows = 0
	n.oldAzimuth = 0
	n.redshift = 0
	n.startOfTwilight = 0
	n.generation++
	n.mu.Unlock()
}

func (n *NightInfo) snapshot() nightRenderState {
	if n == nil {
		return nightRenderState{}
	}
	n.mu.Lock()
	state := nightRenderState{
		baseLevel: n.BaseLevel, azimuth: n.Azimuth, cloudy: n.Cloudy,
		flags: n.Flags, level: n.Level, shadows: n.Shadows,
		redshift: n.redshift, generation: n.generation,
	}
	n.mu.Unlock()
	return state
}

func (n *NightInfo) generationSnapshot() uint64 {
	if n == nil {
		return 0
	}
	n.mu.Lock()
	generation := n.generation
	n.mu.Unlock()
	return generation
}

var (
	nightImg *ebiten.Image
)

var blackImg *ebiten.Image

var nightRE = regexp.MustCompile(`^/nt ([0-9]+) /sa ([-0-9]+) /cl ([01])`)

func (n *NightInfo) calcCurLevel() {
	delta := 0
	if n.Flags&kLightNoNightMods != 0 {
		n.Level = 0
	} else {
		if n.Flags&kLightAdjust25Pct != 0 {
			delta += 25
		}
		if n.Flags&kLightAdjust50Pct != 0 {
			delta += 50
		}
		if n.Flags&kLightAreaIsDarker != 0 {
			delta = -delta
		}
		n.Level = n.BaseLevel - delta
	}
	if n.Level < 0 {
		n.Level = 0
	} else if n.Level > 100 {
		n.Level = 100
	}

	if n.Flags&kLightNoShadows != 0 {
		n.Shadows = 0
	} else {
		n.Shadows = 50 - n.Level
		if n.Shadows < 0 {
			n.Shadows = 0
		}
		if n.Cloudy && n.Shadows > 25 {
			n.Shadows = 25
		}
	}
}

func (n *NightInfo) calcRedshift(frame int) {
	const ticksPerGameSecond = 60.0 / 4.09
	const twilightLength = 30 * 60 * ticksPerGameSecond
	const maxRedshift = 1.25

	if n.oldAzimuth != n.Azimuth {
		if (n.oldAzimuth == -2 && n.Azimuth == -1) || (n.oldAzimuth == 179 && n.Azimuth == 180) {
			n.startOfTwilight = frame
		} else {
			n.startOfTwilight = 0
		}
		n.oldAzimuth = n.Azimuth
	}

	if n.Azimuth != -1 && n.Azimuth != 180 {
		n.startOfTwilight = 0
	}

	if n.startOfTwilight != 0 {
		shift := float64(frame-n.startOfTwilight) / twilightLength
		if shift < 0 {
			shift = 0
		} else if shift > 1 {
			shift = 1
		}
		if shift < 0.5 {
			n.redshift = 1 + shift*2*(maxRedshift-1)
		} else {
			n.redshift = 1 + (1-shift)*2*(maxRedshift-1)
		}
	} else {
		n.redshift = 1
	}
}

func (n *NightInfo) SetFlags(f uint) {
	n.setFlags(f, 0)
}

func (n *NightInfo) setFlags(f uint, frame int) {
	n.mu.Lock()
	n.Flags = f
	n.calcCurLevel()
	n.calcRedshift(frame)
	n.generation++
	n.mu.Unlock()
}

// currentNightLevel computes the effective night percentage (0..100) after
// applying client preferences and server flags.
func currentNightLevel() int {
	return effectiveNightLevel(primarySession.night.snapshot())
}

func effectiveNightLevel(night nightRenderState) int {
	lvl := night.level
	flags := night.flags
	limit := gs.MaxNightLevel
	if flags&kLightForce100Pct != 0 {
		limit = 100
	}
	if gs.forceNightLevel >= 0 {
		lvl = gs.forceNightLevel
	}
	if lvl > limit {
		lvl = limit
	}
	if lvl < 0 {
		lvl = 0
	}
	return lvl
}

func explicitShadowPictureAlpha(flags uint32) (bool, float32) {
	return explicitShadowPictureAlphaForNight(flags, primarySession.night.snapshot())
}

func explicitShadowPictureAlphaForNight(flags uint32, night nightRenderState) (bool, float32) {
	if flags&climg.PictDefIsShadow == 0 {
		return true, 1
	}
	shadows := night.shadows
	rawLevel := night.level
	lightFlags := night.flags
	if shadows == 0 {
		return false, 0
	}
	limit := gs.MaxNightLevel
	if lightFlags&kLightForce100Pct != 0 {
		limit = 100
	}
	if rawLevel > 33 && limit > 33 {
		return true, 0.25
	}
	return true, 1
}

func parseNightCommand(s string) bool {
	return parseNightCommandForSession(primarySession, s)
}

func parseNightCommandForSession(session *Session, s string) bool {
	if session == nil || session.night == nil {
		return false
	}
	frame := 0
	if session.draw != nil {
		session.draw.mu.Lock()
		frame = session.draw.frame
		session.draw.mu.Unlock()
	}
	night := session.night
	if m := nightRE.FindStringSubmatch(s); m != nil {
		lvl, _ := strconv.Atoi(m[1])
		sa, _ := strconv.Atoi(m[2])
		cloudy := m[3] != "0"
		night.mu.Lock()
		night.BaseLevel = lvl
		night.Level = lvl
		night.Azimuth = sa
		night.Cloudy = cloudy
		night.calcCurLevel()
		night.calcRedshift(frame)
		night.generation++
		night.mu.Unlock()
		return true
	}
	const prefix = "/nt "
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	rest := s[len(prefix):]
	var nightLevel, shadowLevel, sunAngle, declination int
	if n, err := fmt.Sscanf(rest, "%d %d %d %d", &nightLevel, &shadowLevel, &sunAngle, &declination); err == nil && n >= 3 {
		night.mu.Lock()
		night.BaseLevel = nightLevel
		night.Level = nightLevel
		night.Shadows = shadowLevel
		night.Azimuth = sunAngle
		night.calcRedshift(frame)
		night.generation++
		night.mu.Unlock()
		return true
	}
	if n, err := fmt.Sscanf(rest, "%d", &nightLevel); err == nil && n == 1 {
		night.mu.Lock()
		night.BaseLevel = nightLevel
		night.Level = nightLevel
		night.calcCurLevel()
		night.calcRedshift(frame)
		night.generation++
		night.mu.Unlock()
		return true
	}
	return false
}

func init() {
	if nightImg == nil {
		img, _, err := image.Decode(bytes.NewReader(nightImage))
		if err != nil {
			return
		}
		// Use the decoded image directly without adding a border to avoid
		// off-by-one sizing issues.
		nightImg = newManagedImageFromImage(img)
	}
	if blackImg == nil {
		blackImg = newManagedImage(1, 1)
		blackImg.Fill(color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	}
}

func drawNightOverlay(screen *ebiten.Image, ox, oy int) {
	drawNightOverlayForNight(screen, ox, oy, primarySession.night.snapshot())
}

func drawNightOverlayForNight(screen *ebiten.Image, ox, oy int, night nightRenderState) {
	ox += screen.Bounds().Min.X
	oy += screen.Bounds().Min.Y
	lvl := effectiveNightLevel(night)
	if lvl <= 0 {
		return
	}

	img := nightImg
	if img == nil {
		return
	}

	// Scale overlay exactly to the current game view size so it fully covers it.
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	vw := float64(int(math.Round(float64(gameAreaSizeX) * gs.GameScale)))
	vh := float64(int(math.Round(float64(gameAreaSizeY) * gs.GameScale)))
	sx := 0.0
	sy := 0.0
	if iw > 0 {
		sx = vw / float64(iw)
	}
	if ih > 0 {
		sy = vh / float64(ih)
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true}
	op.GeoM.Scale(sx, sy)
	alpha := float32(lvl) / 100.0
	op.ColorScale.ScaleAlpha(alpha)
	op.GeoM.Translate(float64(ox), float64(oy))
	screen.DrawImage(img, op)
}
