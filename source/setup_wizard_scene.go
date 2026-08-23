package main

import (
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type setupWizardSceneMode uint8

const (
	setupWizardSceneDay setupWizardSceneMode = iota
	setupWizardSceneIndoor
	setupWizardSceneNight
	setupWizardSceneMotion
)

var (
	setupWizardSceneModeValue = setupWizardSceneDay
	setupWizardScenePage      = -1
	setupWizardSceneStarted   time.Time
)

func selectSetupWizardSceneForPage(page int) {
	if setupWizardScenePage == page {
		return
	}
	setupWizardScenePage = page
	switch page {
	case 3:
		setupWizardSceneModeValue = setupWizardSceneIndoor
	case 5:
		setupWizardSceneModeValue = setupWizardSceneMotion
	case 6:
		setupWizardSceneModeValue = setupWizardSceneNight
	default:
		setupWizardSceneModeValue = setupWizardSceneDay
	}
}

func setupWizardSceneName(mode setupWizardSceneMode) string {
	switch mode {
	case setupWizardSceneIndoor:
		return "INDOOR CONTACT SHADOWS"
	case setupWizardSceneNight:
		return "NIGHT GLOW + LIGHT CONES"
	case setupWizardSceneMotion:
		return "MOVEMENT + FRAME BLENDING"
	default:
		return "DAYLIGHT CHARACTER SHADOWS"
	}
}

func applySetupWizardSceneLighting(mode setupWizardSceneMode) {
	night := movieNightState{azimuth: 60, oldAzimuth: 60, redshift: 1}
	switch mode {
	case setupWizardSceneIndoor, setupWizardSceneMotion:
		night.azimuth = 90
		night.oldAzimuth = 90
		night.cloudy = true
		night.shadows = 25
	case setupWizardSceneNight:
		night.baseLevel = 78
		night.level = 78
	default:
		night.shadows = 50
	}
	restoreMovieNightState(night)
}

func setupWizardWalkPosition(step int64) (int16, bool) {
	const halfCycle = int64(10)
	phase := step % (halfCycle * 2)
	movingRight := phase < halfCycle
	if !movingRight {
		phase = halfCycle*2 - phase
	}
	return int16(-180 + phase*16), movingRight
}

func setupWizardScenePicture(id uint16, h, v int16) framePicture {
	plane := 0
	if clImages != nil {
		plane = clImages.Plane(uint32(id))
	}
	return framePicture{PictID: id, H: h, V: v, PrevH: h, PrevV: v, Plane: plane}
}

func prepareSetupWizardSceneSnapshot(snap *drawSnapshot, now time.Time) {
	if setupWizardSceneStarted.IsZero() {
		setupWizardSceneStarted = now
	}
	applySetupWizardSceneLighting(setupWizardSceneModeValue)

	const interval = 420 * time.Millisecond
	elapsed := now.Sub(setupWizardSceneStarted)
	if elapsed < 0 {
		elapsed = 0
	}
	step := int64(elapsed / interval)
	tickStart := setupWizardSceneStarted.Add(time.Duration(step) * interval)

	if snap.descriptors == nil {
		snap.descriptors = make(map[uint8]frameDescriptor, 4)
	} else {
		clear(snap.descriptors)
	}
	if snap.prevMobiles == nil {
		snap.prevMobiles = make(map[uint8]frameMobile, 4)
	} else {
		clear(snap.prevMobiles)
	}
	if snap.prevDescs == nil {
		snap.prevDescs = make(map[uint8]frameDescriptor, 4)
	} else {
		clear(snap.prevDescs)
	}
	if snap.prevPicturePositions != nil {
		clear(snap.prevPicturePositions)
	}
	snap.prevPictureIndexValid = false

	descriptors := []frameDescriptor{
		{Index: 1, Type: 1, PictID: 447, Name: "Motion", Plane: 0},
		{Index: 2, Type: 1, PictID: 456, Name: "Shadows", Plane: 0},
		{Index: 3, Type: 3, PictID: 565, Name: "Creature", Plane: 0},
	}
	for _, descriptor := range descriptors {
		if clImages != nil {
			descriptor.Plane = clImages.Plane(uint32(descriptor.PictID))
		}
		snap.descriptors[descriptor.Index] = descriptor
		snap.prevDescs[descriptor.Index] = descriptor
	}

	prevH, prevRight := setupWizardWalkPosition(step)
	curH, curRight := setupWizardWalkPosition(step + 1)
	prevFacing, curFacing := uint8(16), uint8(16)
	if prevRight {
		prevFacing = 0
	}
	if curRight {
		curFacing = 0
	}
	prevWalker := frameMobile{Index: 1, State: prevFacing + uint8(step%4), H: prevH, V: 45}
	curWalker := frameMobile{Index: 1, State: curFacing + uint8((step+1)%4), H: curH, V: 45}
	prevCompanion := frameMobile{Index: 2, State: 8 + uint8(step%4), H: 70, V: 100}
	curCompanion := frameMobile{Index: 2, State: 8 + uint8((step+1)%4), H: 70, V: 100}
	prevCreature := frameMobile{Index: 3, State: uint8(step % 4), H: 180, V: -70}
	curCreature := frameMobile{Index: 3, State: uint8((step + 1) % 4), H: 180, V: -70}
	snap.prevMobiles[1] = prevWalker
	snap.prevMobiles[2] = prevCompanion
	snap.prevMobiles[3] = prevCreature
	snap.mobiles = append(snap.mobiles[:0], curWalker, curCompanion, curCreature)
	sortMobiles(snap.mobiles)
	snap.liveMobs = append(snap.liveMobs[:0], snap.mobiles...)
	snap.deadMobs = snap.deadMobs[:0]

	// Tile and decorate the visible right side of the world. These are real
	// CL_Images assets, including two animated, light-emitting fixtures.
	pictures := []framePicture{
		setupWizardScenePicture(282, -300, -270),
		setupWizardScenePicture(282, -100, -270),
		setupWizardScenePicture(282, 100, -270),
		setupWizardScenePicture(282, -300, -70),
		setupWizardScenePicture(282, -100, -70),
		setupWizardScenePicture(282, 100, -70),
		setupWizardScenePicture(282, -300, 130),
		setupWizardScenePicture(282, -100, 130),
		setupWizardScenePicture(282, 100, 130),
		setupWizardScenePicture(203, -245, -145),
		setupWizardScenePicture(3574, 145, -15),
		setupWizardScenePicture(4117, -160, 205),
		setupWizardScenePicture(4116, 160, 220),
		setupWizardScenePicture(679, -35, -45),
		setupWizardScenePicture(1889, -245, 5),
		setupWizardScenePicture(1888, -60, -25),
		setupWizardScenePicture(1890, 155, 5),
		setupWizardScenePicture(330, 220, 55),
		setupWizardScenePicture(425, 115, 80),
	}
	// A small independently moving world sprite makes positional smoothing
	// visible separately from the walking character.
	moving := setupWizardScenePicture(6872, int16(-80+(step+1)%8*10), 190)
	moving.PrevH = int16(-80 + step%8*10)
	moving.Moving = true
	pictures = append(pictures, moving)
	sortPictures(pictures)
	snap.picsNeg = snap.picsNeg[:0]
	snap.picsZero = snap.picsZero[:0]
	snap.picsPos = snap.picsPos[:0]
	for _, picture := range pictures {
		switch {
		case picture.Plane < 0:
			snap.picsNeg = append(snap.picsNeg, picture)
		case picture.Plane > 0:
			snap.picsPos = append(snap.picsPos, picture)
		default:
			snap.picsZero = append(snap.picsZero, picture)
		}
	}

	snap.prevTime = tickStart
	snap.curTime = tickStart.Add(interval)
	snap.picShiftX = 0
	snap.picShiftY = 0
	snap.dropped = 0
	snap.lightingFlags = 0
	snap.logicalFrame = int(step + 1)
	snap.bubbles = snap.bubbles[:0]
	if setupWizardPage == 3 {
		snap.bubbles = append(snap.bubbles, bubble{
			Index: 2, Text: "Adjust bubbles and visibility here", Type: kBubbleNormal,
			CreatedFrame: snap.logicalFrame, LifeFrames: 100000,
		})
	}
	snap.hp, snap.hpMax = 68, 100
	snap.sp, snap.spMax = 43, 100
	snap.balance, snap.balanceMax = 82, 100
	snap.prevHP, snap.prevHPMax = snap.hp, snap.hpMax
	snap.prevSP, snap.prevSPMax = snap.sp, snap.spMax
	snap.prevBalance, snap.prevBalanceMax = snap.balance, snap.balanceMax
}

func drawSetupWizardSceneLabel(dst *ebiten.Image, scale float64) {
	if dst == nil || scale <= 0 {
		return
	}
	label := setupWizardSceneName(setupWizardSceneModeValue)
	const x, y = 285, 20
	w, _ := text.Measure(label, mainFontBold, 0)
	pad := 8 * scale
	vector.FillRect(dst, float32(float64(x)*scale-pad), float32(float64(y)*scale-pad/2), float32(w+pad*2), float32(22*scale), color.RGBA{R: 15, G: 18, B: 24, A: 205}, false)
	op := acquireTextDrawOpts()
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x)*scale, float64(y)*scale)
	text.Draw(dst, label, mainFontBold, op)
	releaseTextDrawOpts(op)
}
