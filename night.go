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

type nightData struct {
	level       int
	shadow      int
	azimuth     int
	cloudy      bool
	haveShadow  bool
	haveAzimuth bool
	haveCloudy  bool
}

func parseNightData(s string) (nightData, bool) {
	if m := nightRE.FindStringSubmatch(s); m != nil {
		lvl, _ := strconv.Atoi(m[1])
		sa, _ := strconv.Atoi(m[2])
		cloudy := m[3] != "0"
		return nightData{level: lvl, azimuth: sa, cloudy: cloudy, haveAzimuth: true, haveCloudy: true}, true
	}
	const prefix = "/nt "
	if !strings.HasPrefix(s, prefix) {
		return nightData{}, false
	}
	rest := s[len(prefix):]
	var nightLevel, shadowLevel, sunAngle, declination int
	if n, err := fmt.Sscanf(rest, "%d %d %d %d", &nightLevel, &shadowLevel, &sunAngle, &declination); err == nil && n >= 3 {
		return nightData{level: nightLevel, shadow: shadowLevel, azimuth: sunAngle, haveShadow: true, haveAzimuth: true}, true
	}
	if n, err := fmt.Sscanf(rest, "%d", &nightLevel); err == nil && n == 1 {
		return nightData{level: nightLevel}, true
	}
	return nightData{}, false
}

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
	nd, ok := parseNightData(s)
	if !ok {
		return false
	}
	gNight.mu.Lock()
	gNight.BaseLevel = nd.level
	gNight.Level = nd.level
	if nd.haveAzimuth {
		gNight.Azimuth = nd.azimuth
	}
	if nd.haveCloudy {
		gNight.Cloudy = nd.cloudy
	}
	gNight.calcCurLevel()
	if nd.haveShadow {
		gNight.Shadows = nd.shadow
	}
	gNight.mu.Unlock()
	return true
}

var (
	nightImgs            = map[int]*ebiten.Image{}
	nightImgW, nightImgH int
)

func drawNightOverlay(screen *ebiten.Image) {
	gNight.mu.Lock()
	lvl := gNight.Level
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

	op := &ebiten.DrawImageOptions{CompositeMode: ebiten.CompositeModeMultiply}
	screen.DrawImage(nightImg, op)
}

func rebuildNightOverlay(w, h, level int) *ebiten.Image {
	img := ebiten.NewImage(w, h)
	lf := float64(level) / 100.0
	rim := 1.0 - lf
	center := rim
	if lf >= 0.5 {
		center = 0.5
	}
	cx := float64(w) / 2
	cy := float64(h) / 2
	radius := 325.0 * float64(scale)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			t := math.Sqrt(dx*dx+dy*dy) / radius
			if t > 1 {
				t = 1
			}
			c := center*(1-t) + rim*t
			if c < 0 {
				c = 0
			} else if c > 1 {
				c = 1
			}
			clr := color.RGBA{
				R: uint8(c * 255),
				G: uint8(c * 255),
				B: uint8(c * 255),
				A: 0xff,
			}
			img.Set(x, y, clr)
		}
	}
	return img
}
