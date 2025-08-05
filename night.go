package main

import (
	"fmt"
	"image/color"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

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
}

var gNight NightInfo

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

func (n *NightInfo) calcRedshift() {
	const ticksPerGameSecond = 60.0 / 4.09
	const twilightLength = 30 * 60 * ticksPerGameSecond
	const maxRedshift = 1.25

	if n.oldAzimuth != n.Azimuth {
		if (n.oldAzimuth == -2 && n.Azimuth == -1) || (n.oldAzimuth == 179 && n.Azimuth == 180) {
			n.startOfTwilight = frameCounter
		} else {
			n.startOfTwilight = 0
		}
		n.oldAzimuth = n.Azimuth
	}

	if n.Azimuth != -1 && n.Azimuth != 180 {
		n.startOfTwilight = 0
	}

	if n.startOfTwilight != 0 {
		shift := float64(frameCounter-n.startOfTwilight) / twilightLength
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
	n.mu.Lock()
	n.Flags = f
	n.calcCurLevel()
	n.calcRedshift()
	n.mu.Unlock()
}

func parseNightCommand(s string) bool {
	if m := nightRE.FindStringSubmatch(s); m != nil {
		lvl, _ := strconv.Atoi(m[1])
		sa, _ := strconv.Atoi(m[2])
		cloudy := m[3] != "0"
		gNight.mu.Lock()
		gNight.BaseLevel = lvl
		gNight.Level = lvl
		gNight.Azimuth = sa
		gNight.Cloudy = cloudy
		gNight.calcCurLevel()
		gNight.calcRedshift()
		gNight.mu.Unlock()
		return true
	}
	const prefix = "/nt "
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	rest := s[len(prefix):]
	var nightLevel, shadowLevel, sunAngle, declination int
	if n, err := fmt.Sscanf(rest, "%d %d %d %d", &nightLevel, &shadowLevel, &sunAngle, &declination); err == nil && n >= 3 {
		gNight.mu.Lock()
		gNight.BaseLevel = nightLevel
		gNight.Level = nightLevel
		gNight.Azimuth = sunAngle
		gNight.calcCurLevel()
		gNight.calcRedshift()
		gNight.mu.Unlock()
		return true
	}
	if n, err := fmt.Sscanf(rest, "%d", &nightLevel); err == nil && n == 1 {
		gNight.mu.Lock()
		gNight.BaseLevel = nightLevel
		gNight.Level = nightLevel
		gNight.calcCurLevel()
		gNight.calcRedshift()
		gNight.mu.Unlock()
		return true
	}
	return false
}

var (
	nightImgs            = map[int]*ebiten.Image{}
	nightImgW, nightImgH int
)

func drawNightOverlay(screen *ebiten.Image) {
	gNight.mu.Lock()
	lvl := gNight.Level
	redshift := gNight.redshift
	gNight.mu.Unlock()
	if lvl <= 0 {
		return
	}

	var overlayLevel int
	switch {
	case lvl < 38:
		overlayLevel = 25
	case lvl < 63:
		overlayLevel = 50
	case lvl < 88:
		overlayLevel = 75
	default:
		overlayLevel = 100
	}

	w := gameAreaSizeX * scale
	h := gameAreaSizeY * scale
	if nightImgW != w || nightImgH != h {
		nightImgs = map[int]*ebiten.Image{}
		nightImgW, nightImgH = w, h
	}
	nightImg := nightImgs[overlayLevel]
	if nightImg == nil {
		nightImg = rebuildNightOverlay(w, h, overlayLevel)
		nightImgs[overlayLevel] = nightImg
	}

	op := &ebiten.DrawImageOptions{}
	if redshift > 1 {
		var cm ebiten.ColorM
		cm.SetElement(0, 3, redshift-1)
		op.ColorM = cm
	}
	screen.DrawImage(nightImg, op)
}

// rebuildNightOverlay creates a radial gradient that becomes fully opaque at
// the given radius percentage of the maximum possible radius from the center
// of the screen.
func rebuildNightOverlay(w, h, radiusPercent int) *ebiten.Image {
	img := ebiten.NewImage(w, h)

	cx := float64(w) / 2
	cy := float64(h) / 2

	maxRadius := math.Sqrt(cx*cx + cy*cy)
	radius := maxRadius * float64(radiusPercent) / 100

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			var a float64
			if dist >= radius {
				a = 1
			} else {
				a = dist / radius
			}

			clr := color.RGBA{
				R: 0,
				G: 0,
				B: 0,
				A: uint8(a * 255),
			}
			img.Set(x, y, clr)
		}
	}
	return img
}
