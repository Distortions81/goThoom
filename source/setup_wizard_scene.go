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
	case setupWizardInterfacePage:
		setupWizardSceneModeValue = setupWizardSceneIndoor
	case setupWizardMotionPage:
		setupWizardSceneModeValue = setupWizardSceneMotion
	case setupWizardNightPage:
		setupWizardSceneModeValue = setupWizardSceneNight
	default:
		setupWizardSceneModeValue = setupWizardSceneDay
	}
}

func setupWizardSceneName(mode setupWizardSceneMode) string {
	switch mode {
	case setupWizardSceneIndoor:
		return "OVERCAST CONTACT SHADOWS"
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
	case setupWizardSceneIndoor:
		night.azimuth = 90
		night.oldAzimuth = 90
		night.cloudy = true
		night.shadows = 25
	case setupWizardSceneNight:
		night.baseLevel = 100
		night.level = 100
	default:
		night.shadows = 50
	}
	restoreMovieNightState(night)
}

func setupWizardWalkPosition(step int64) int16 {
	const halfCycle = int64(18)
	phase := step % (halfCycle * 2)
	if phase >= halfCycle {
		phase = halfCycle*2 - phase
	}
	return int16(-220 + phase*22)
}

func setupWizardScenePicture(id uint16, h, v int16) framePicture {
	plane := 0
	if clImages != nil {
		plane = clImages.Plane(uint32(id))
	}
	return framePicture{PictID: id, H: h, V: v, PrevH: h, PrevV: v, Plane: plane}
}

func prepareSetupWizardSceneSnapshot(snap *drawSnapshot, now time.Time) {
	// The preview is generated locally and can change without a network-world
	// generation, so force render-side scene caches to compare its actual keys.
	snap.worldGeneration = 0
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
		{Index: 1, Type: kDescPlayer, PictID: 447, Name: "Traveler", Plane: 0},
		{Index: 2, Type: kDescPlayer, PictID: 456, Name: "Guide", Plane: 0},
		{Index: 3, Type: kDescPlayer, PictID: 565, Name: "Companion", Plane: 0},
		{Index: 4, Type: kDescPlayer, PictID: 447, Name: "Apprentice", Plane: 0},
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
	prevWalker := frameMobile{Index: 1, State: facing + uint8(step%4), H: prevH, V: 105}
	curWalker := frameMobile{Index: 1, State: facing + uint8((step+1)%4), H: curH, V: 105}
	prevGuide := frameMobile{Index: 2, State: 8 + uint8(step%4), H: 35, V: 20}
	curGuide := frameMobile{Index: 2, State: 8 + uint8((step+1)%4), H: 35, V: 20}
	prevCompanion := frameMobile{Index: 3, State: uint8(step % 4), H: -100, V: 28, Colors: lowHealth}
	curCompanion := frameMobile{Index: 3, State: uint8((step + 1) % 4), H: -100, V: 28, Colors: lowHealth}
	prevApprentice := frameMobile{Index: 4, State: 8 + uint8(step%4), H: 10, V: 48, Colors: lowHealth}
	curApprentice := frameMobile{Index: 4, State: 8 + uint8((step+1)%4), H: 10, V: 48, Colors: lowHealth}
	snap.prevMobiles[1] = prevWalker
	snap.prevMobiles[2] = prevGuide
	snap.prevMobiles[3] = prevCompanion
	snap.prevMobiles[4] = prevApprentice
	snap.mobiles = append(snap.mobiles[:0], curWalker, curGuide, curCompanion, curApprentice)
	sortMobiles(snap.mobiles)
	snap.liveMobs = append(snap.liveMobs[:0], snap.mobiles...)
	snap.deadMobs = snap.deadMobs[:0]

	// Build a single riverside scene from real CL_Images artwork found in the
	// bundled movies. Animated water and grounded lanterns exercise world-frame
	// blending and shader lighting without looking like disconnected effects.
	pictures := []framePicture{
		setupWizardScenePicture(4928, -200, -200),
		setupWizardScenePicture(4928, 200, -200),
		setupWizardScenePicture(4928, -200, 200),
		setupWizardScenePicture(4928, 200, 200),
		setupWizardScenePicture(5120, -190, -245),
		setupWizardScenePicture(5121, 190, -245),
		setupWizardScenePicture(4622, -190, -30),
		setupWizardScenePicture(4621, 190, -25),
		setupWizardScenePicture(873, -220, -15),
		setupWizardScenePicture(873, 220, 0),
		setupWizardScenePicture(1491, 0, -170),
		setupWizardScenePicture(2271, 0, -62),
		setupWizardScenePicture(4086, -205, -105),
		setupWizardScenePicture(5095, 110, -92),
		setupWizardScenePicture(5094, -100, -82),
		setupWizardScenePicture(3574, -105, 165),
		setupWizardScenePicture(3582, -220, 165),
		setupWizardScenePicture(5118, -170, 195),
		setupWizardScenePicture(5115, -180, 125),
		setupWizardScenePicture(5114, 195, -25),
		setupWizardScenePicture(5116, 155, 165),
		setupWizardScenePicture(4615, 105, 25),
		setupWizardScenePicture(1925, -48, -55),
		setupWizardScenePicture(1925, 48, -55),
	}
	logicalFrame := int(step + 1)
	cachePictureObscuring(pictures, snap.mobiles, snap.descriptors, snap.prevMobiles, logicalFrame)
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
	snap.logicalFrame = logicalFrame
	snap.bubbles = snap.bubbles[:0]
	if setupWizardPage == setupWizardInterfacePage {
		snap.bubbles = append(snap.bubbles, bubble{
			Index: 2, Text: "Welcome to Puddleby!", Type: kBubbleNormal,
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
