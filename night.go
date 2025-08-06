package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
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

var (
	nightImg  *ebiten.Image
	nightOnce sync.Once
)

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

func getNightImage() *ebiten.Image {
	nightOnce.Do(func() {
		f, err := os.Open("data/night.png")
		if err != nil {
			return
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return
		}
		nightImg = ebiten.NewImageFromImage(img)
	})
	return nightImg
}

func drawNightOverlay(screen *ebiten.Image) {
	gNight.mu.Lock()
	lvl := gNight.Level
	gNight.mu.Unlock()
	if lvl <= 0 {
		return
	}

	img := getNightImage()
	if img == nil {
		return
	}

	w := float64(gameAreaSizeX * scale)
	h := float64(gameAreaSizeY * scale)
	bw := float64(img.Bounds().Dx())
	bh := float64(img.Bounds().Dy())

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(w/bw, h/bh)
	alpha := float64(lvl) / 100.0
	op.ColorM.Scale(1, 1, 1, alpha)
	screen.DrawImage(img, op)
}
