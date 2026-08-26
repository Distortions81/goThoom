package main

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
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
	case 2:
		setupWizardSceneModeValue = setupWizardSceneIndoor
	case 4:
		setupWizardSceneModeValue = setupWizardSceneMotion
	case 5:
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

func setupWizardWalkPosition(step int64) int16 {
	const halfCycle = int64(10)
	phase := step % (halfCycle * 2)
	if phase >= halfCycle {
		phase = halfCycle*2 - phase
	}
	return int16(-180 + phase*16)
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
		{Index: 3, Type: kDescPlayer, PictID: 565, Name: "Creature", Plane: 0},
	}
	for _, descriptor := range descriptors {
		if clImages != nil {
			descriptor.Plane = clImages.Plane(uint32(descriptor.PictID))
		}
		snap.descriptors[descriptor.Index] = descriptor
		snap.prevDescs[descriptor.Index] = descriptor
	}

	prevH := setupWizardWalkPosition(step)
	curH := setupWizardWalkPosition(step + 1)
	facing := uint8(16)
	if curH > prevH {
		facing = 0
	}
	lowHealth := uint8(kColorCodeBackRed << 4)
	prevWalker := frameMobile{Index: 1, State: facing + uint8(step%4), H: prevH, V: 45}
	curWalker := frameMobile{Index: 1, State: facing + uint8((step+1)%4), H: curH, V: 45}
	prevCompanion := frameMobile{Index: 2, State: 8 + uint8(step%4), H: 70, V: 100, Colors: lowHealth}
	curCompanion := frameMobile{Index: 2, State: 8 + uint8((step+1)%4), H: 70, V: 100, Colors: lowHealth}
	prevCreature := frameMobile{Index: 3, State: uint8(step % 4), H: 180, V: -70, Colors: lowHealth}
	curCreature := frameMobile{Index: 3, State: uint8((step + 1) % 4), H: 180, V: -70, Colors: lowHealth}
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
	if setupWizardPage == 2 {
		snap.bubbles = append(snap.bubbles, bubble{
			Index: 2, Text: "Preview names and bubbles here", Type: kBubbleNormal,
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

func drawSetupWizardFPS(dst *ebiten.Image) {
	if dst == nil || mainFontBold == nil || setupWizardWin == nil || !setupWizardWin.IsOpen() {
		return
	}
	label := fmt.Sprintf("%.0f FPS", ebiten.ActualFPS())
	w, _ := text.Measure(label, mainFontBold, 0)
	x := float64(dst.Bounds().Dx()) - w - 8
	if x < 4 {
		x = 4
	}
	const y = 6

	shadow := acquireTextDrawOpts()
	shadow.GeoM.Translate(x+1, y+1)
	shadow.ColorScale.Scale(0, 0, 0, 0.85)
	text.Draw(dst, label, mainFontBold, shadow)
	releaseTextDrawOpts(shadow)

	op := acquireTextDrawOpts()
	op.GeoM.Translate(x, y)
	text.Draw(dst, label, mainFontBold, op)
	releaseTextDrawOpts(op)
}
