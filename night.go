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
	mu        sync.Mutex
	BaseLevel int
	Azimuth   int
	Cloudy    bool
	Flags     uint
	Level     int
	Shadows   int
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

func (n *NightInfo) SetFlags(f uint) {
	n.mu.Lock()
	n.Flags = f
	n.calcCurLevel()
	n.mu.Unlock()
}

func parseNightCommand(s string) bool {
	if m := nightRE.FindStringSubmatch(s); m != nil {
		lvl, _ := strconv.Atoi(m[1])
		sa, _ := strconv.Atoi(m[2])
		cloudy := m[3] != "0"
		gNight.mu.Lock()
		gNight.BaseLevel = lvl
		gNight.Azimuth = sa
		gNight.Cloudy = cloudy
		gNight.calcCurLevel()
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
		gNight.Level = nightLevel
		gNight.Shadows = shadowLevel
		gNight.Azimuth = sunAngle
		gNight.mu.Unlock()
		return true
	}
	if n, err := fmt.Sscanf(rest, "%d", &nightLevel); err == nil && n == 1 {
		shadowLevel = 50 - nightLevel
		if shadowLevel < 0 {
			shadowLevel = 0
		}
		gNight.mu.Lock()
		gNight.Level = nightLevel
		gNight.Shadows = shadowLevel
		gNight.mu.Unlock()
		return true
	}
	return false
}

var (
	nightImg         *ebiten.Image
	nightImgLevel    int
	nightImgRedshift float64
	nightImgShadows  int
)

func drawNightOverlay(screen *ebiten.Image) {
	gNight.mu.Lock()
	lvl := gNight.Level
	shd := gNight.Shadows
	gNight.mu.Unlock()
	if lvl <= 0 && shd <= 0 {
		return
	}
	redshift := 1.0
	w := gameAreaSizeX * scale
	h := gameAreaSizeY * scale
	if nightImg == nil || nightImg.Bounds().Dx() != w || nightImg.Bounds().Dy() != h || nightImgLevel != lvl || nightImgShadows != shd || nightImgRedshift != redshift {
		rebuildNightOverlay(w, h, lvl, redshift, shd)
	}
	op := &ebiten.DrawImageOptions{}
	op.CompositeMode = ebiten.CompositeModeMultiply
	screen.DrawImage(nightImg, op)
}

func rebuildNightOverlay(w, h, level int, redshift float64, shadows int) {
	if nightImg == nil || nightImg.Bounds().Dx() != w || nightImg.Bounds().Dy() != h {
		nightImg = ebiten.NewImage(w, h)
	} else {
		nightImg.Clear()
	}
	lf := float64(level) / 100.0
	rim := 1.0 - lf
	center := rim
	if lf >= 0.5 {
		center = 0.5
	}
	cx := float64(w) / 2
	cy := float64(h) / 2
	radius := 325.0 * float64(scale)
	sf := float64(shadows) / 50.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			t := math.Sqrt(dx*dx+dy*dy) / radius
			if t > 1 {
				t = 1
			}
			c := center*(1-t) + rim*t
			c *= 1 - sf
			r := c * redshift
			if r > 1 {
				r = 1
			}
			if c < 0 {
				c = 0
			}
			clr := color.RGBA{
				R: uint8(r * 255),
				G: uint8(c * 255),
				B: uint8(c * 255),
				A: uint8(c * 255),
			}
			nightImg.Set(x, y, clr)
		}
	}
	nightImgLevel = level
	nightImgRedshift = redshift
	nightImgShadows = shadows
}
