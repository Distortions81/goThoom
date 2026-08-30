package main

import (
	"image"
	"math"
	"time"
)

const bubbleTortureFrameInterval = 220 * time.Millisecond
const bubbleTortureTextCycle = 8 * time.Second
const bubbleTortureVisibleCount = 6

var bubbleTortureStarted time.Time

type bubbleTortureSpec struct {
	owner int
	typ   int
}

var bubbleTortureSpecs = []bubbleTortureSpec{
	{owner: 0, typ: kBubbleNormal},
	{owner: 1, typ: kBubbleWhisper},
	{owner: 7, typ: kBubbleRealAction},
	{owner: 3, typ: kBubbleThought},
	{owner: 4, typ: kBubblePonder},
	{owner: 5, typ: kBubbleMonster},
	{owner: 6, typ: kBubblePlayerAction},
	{owner: 2, typ: kBubbleYell},
	{owner: 8, typ: kBubbleNarrate},
	{owner: 9, typ: kBubbleNormal},
	{owner: 10, typ: kBubbleWhisper},
	{owner: 11, typ: kBubbleYell},
}

var bubbleTortureTexts = []string{
	"Hi!",
	"A quiet whisper with just enough text to wrap once.",
	"WATCH OUT — EVERYONE IS MOVING!",
	"I wonder whether this thought can stay connected to its speaker while the crowd crosses paths.",
	"Considering several possibilities...",
	"GRRRR! Keep away from my corner!",
	"waves, turns around, and points toward the busiest part of the crowd",
	"A sudden action happens here while every nearby bubble shares the available space.",
	"BUBBLE TORTURE: all styles, moving speakers, camera motion, and event-driven reflow",
	"This deliberately oversized message contains enough words to exercise balanced wrapping, the maximum bubble dimensions, and font shrinking while several nearby bubbles compete for the same limited patch of screen space. It should remain readable, rectangular, attached to the correct person, and free from overlapping text even as the camera and every speaker continue moving.",
	"Tiny.",
	"Line one is intentionally short.\nLine two is much longer and should still fit.\nLine three checks explicit newlines.",
}

var bubbleTortureNames = []string{
	"Tester", "Whisperer", "Shouter", "Thinker",
	"Ponderer", "Growler", "Actor", "Striker",
	"Narrator", "Walker", "Murmurer", "Caller",
}

var bubbleTorturePictIDs = []uint16{447, 456, 565}

type bubbleTortureWorldPicture struct {
	id   uint16
	h, v int
}

var bubbleTortureWorldPictures = []bubbleTortureWorldPicture{
	{id: 4928, h: -260, v: -190}, {id: 4928, h: 140, v: -190},
	{id: 4928, h: -260, v: 210}, {id: 4928, h: 140, v: 210},
	{id: 5120, h: -240, v: -245}, {id: 5121, h: 235, v: -245},
	{id: 4622, h: -225, v: -25}, {id: 4621, h: 225, v: -20},
	{id: 873, h: -275, v: 35}, {id: 873, h: 275, v: 40},
	{id: 1491, h: 0, v: -185}, {id: 2271, h: 0, v: -70},
	{id: 4086, h: -210, v: -110}, {id: 5095, h: 125, v: -95},
	{id: 3574, h: -115, v: 175}, {id: 3582, h: 215, v: 175},
}

func bubbleTortureCameraPosition(step int64) image.Point {
	phase := float64(step)
	return image.Pt(
		roundToInt(90*math.Sin(phase*0.12)),
		roundToInt(55*math.Sin(phase*0.085+0.7)),
	)
}

func bubbleTortureWorldPosition(index int, step int64) image.Point {
	column := index % 4
	row := index / 4
	baseX := -135 + column*90
	baseY := -65 + row*70
	phase := float64(step)*0.22 + float64(index)*0.71
	return image.Pt(
		baseX+roundToInt(34*math.Sin(phase)),
		baseY+roundToInt(24*math.Sin(phase*0.73+float64(index)*0.37)),
	)
}

func bubbleTortureFacing(dx, dy int) uint8 {
	if dx == 0 && dy == 0 {
		return 2
	}
	angle := math.Atan2(float64(dy), float64(dx))
	direction := int(math.Round(angle/(math.Pi/4))) & 7
	// atan2 directions run east, southeast, south, southwest, west,
	// northwest, north, northeast in screen coordinates, matching poses.
	return uint8(direction)
}

func bubbleTortureMobile(index int, step int64, camera image.Point) frameMobile {
	world := bubbleTortureWorldPosition(index, step)
	previous := bubbleTortureWorldPosition(index, step-1)
	dx, dy := world.X-previous.X, world.Y-previous.Y
	action := uint8(1 + (step+int64(index))&1)
	return frameMobile{
		Index: uint8(index),
		State: bubbleTortureFacing(dx, dy)*4 + action,
		H:     int16(world.X - camera.X),
		V:     int16(world.Y - camera.Y),
	}
}

func bubbleTorturePicture(spec bubbleTortureWorldPicture, camera, previousCamera image.Point) framePicture {
	plane := 0
	if clImages != nil {
		plane = clImages.Plane(uint32(spec.id))
	}
	return framePicture{
		PictID: spec.id,
		H:      int16(spec.h - camera.X),
		V:      int16(spec.v - camera.Y),
		PrevH:  int16(spec.h - previousCamera.X),
		PrevV:  int16(spec.v - previousCamera.Y),
		Plane:  plane,
	}
}

func prepareBubbleTortureSnapshot(snap *drawSnapshot, now time.Time) {
	if snap == nil {
		return
	}
	if bubbleTortureStarted.IsZero() || now.Before(bubbleTortureStarted) {
		bubbleTortureStarted = now
	}
	elapsed := now.Sub(bubbleTortureStarted)
	step := int64(elapsed / bubbleTortureFrameInterval)
	tickStart := bubbleTortureStarted.Add(time.Duration(step) * bubbleTortureFrameInterval)
	previousCamera := bubbleTortureCameraPosition(step)
	camera := bubbleTortureCameraPosition(step + 1)

	if snap.descriptors == nil {
		snap.descriptors = make(map[uint8]frameDescriptor, len(bubbleTortureNames))
	} else {
		clear(snap.descriptors)
	}
	if snap.prevDescs == nil {
		snap.prevDescs = make(map[uint8]frameDescriptor, len(bubbleTortureNames))
	} else {
		clear(snap.prevDescs)
	}
	if snap.prevMobiles == nil {
		snap.prevMobiles = make(map[uint8]frameMobile, len(bubbleTortureNames))
	} else {
		clear(snap.prevMobiles)
	}
	if snap.prevPicturePositions != nil {
		clear(snap.prevPicturePositions)
	}
	snap.prevPictureIndexValid = false
	playerIndex = 0

	snap.liveMobs = snap.liveMobs[:0]
	for index, name := range bubbleTortureNames {
		pictID := bubbleTorturePictIDs[index%len(bubbleTorturePictIDs)]
		descriptorType := uint8(kDescPlayer)
		if index == 5 {
			descriptorType = kDescNPC
		}
		descriptor := frameDescriptor{Index: uint8(index), Type: descriptorType, PictID: pictID, Name: name}
		if clImages != nil {
			descriptor.Plane = clImages.Plane(uint32(pictID))
		}
		snap.descriptors[uint8(index)] = descriptor
		snap.prevDescs[uint8(index)] = descriptor
		snap.prevMobiles[uint8(index)] = bubbleTortureMobile(index, step, previousCamera)
		snap.liveMobs = append(snap.liveMobs, bubbleTortureMobile(index, step+1, camera))
	}
	sortMobiles(snap.liveMobs)
	snap.deadMobs = snap.deadMobs[:0]
	snap.mobiles = append(snap.mobiles[:0], snap.liveMobs...)
	sortMobilesNameTags(snap.mobiles)

	pictures := make([]framePicture, 0, len(bubbleTortureWorldPictures))
	for _, spec := range bubbleTortureWorldPictures {
		pictures = append(pictures, bubbleTorturePicture(spec, camera, previousCamera))
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

	textPhase := int(elapsed / bubbleTortureTextCycle)
	snap.bubbles = snap.bubbles[:0]
	firstSpec := (textPhase * bubbleTortureVisibleCount) % len(bubbleTortureSpecs)
	for slot := 0; slot < bubbleTortureVisibleCount; slot++ {
		index := (firstSpec + slot) % len(bubbleTortureSpecs)
		spec := bubbleTortureSpecs[index]
		text := bubbleTortureTexts[(index+textPhase*3)%len(bubbleTortureTexts)]
		descriptor := snap.descriptors[uint8(spec.owner)]
		b := bubble{
			Index:        uint8(spec.owner),
			DedupeID:     uint16(1000 + index),
			OwnerName:    descriptor.Name,
			Text:         text,
			Type:         spec.typ,
			CreatedFrame: int(step),
			LifeFrames:   1 << 30,
		}
		switch spec.typ & kBubbleTypeMask {
		case kBubbleRealAction, kBubblePlayerAction, kBubbleNarrate:
			b.NoArrow = true
		}
		snap.bubbles = append(snap.bubbles, b)
	}
	// A detached edge bubble exercises fixed screen-relative placement and
	// collision handling without borrowing a speaker position.
	snap.bubbles = append(snap.bubbles, bubble{
		Index: 250, DedupeID: 2000, H: int16(-fieldCenterX + 18), V: int16(-fieldCenterY + 34),
		Far: true, NoArrow: true, Text: "Far/off-screen bubble", Type: kBubbleNormal,
		CreatedFrame: int(step), LifeFrames: 1 << 30,
	})

	snap.prevTime = tickStart
	snap.curTime = tickStart.Add(bubbleTortureFrameInterval)
	snap.picShiftX = previousCamera.X - camera.X
	snap.picShiftY = previousCamera.Y - camera.Y
	snap.dropped = 0
	snap.lightingFlags = 0
	snap.logicalFrame = int(step + 1)
	snap.worldGeneration = uint64(step + 1)
	snap.hp, snap.hpMax = 73, 100
	snap.sp, snap.spMax = 41, 100
	snap.balance, snap.balanceMax = 88, 100
	snap.prevHP, snap.prevHPMax = snap.hp, snap.hpMax
	snap.prevSP, snap.prevSPMax = snap.sp, snap.spMax
	snap.prevBalance, snap.prevBalanceMax = snap.balance, snap.balanceMax
}
