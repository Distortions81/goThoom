package main

import (
	"image"
	"math"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

// Floor reflections attach to ordinary scenery pictures; their source artwork
// remains unchanged. Add another ID and profile here to reuse the same mobile
// projection without a full-tile shader or reflection texture. Pictures 160
// and 178 are water-area tiles reserved for a separate water shader.
type floorReflectionProfile struct {
	red, green, blue float32
	alpha            float32
	heightScale      float64
}

func floorReflectionProfileForPict(id uint16) (floorReflectionProfile, bool) {
	if !replacementEffectsEnabled() {
		return floorReflectionProfile{}, false
	}
	switch id {
	case 44, 249, 303, 309, 477, 988, 989, 990, 991, 992, 993, 994, 995, 1197, 5163, 8006:
		return floorReflectionProfile{
			red: 0.55, green: 0.76, blue: 0.90, alpha: 0.32,
			heightScale: 0.72,
		}, true
	default:
		return floorReflectionProfile{}, false
	}
}

func pictureUsesFloorReflection(id uint16) bool {
	_, ok := floorReflectionProfileForPict(id)
	return ok
}

func groundPictureDrawsBelowMobiles(id uint16) bool {
	return replacementEffectPictureDrawsBelowMobiles(id) || pictureUsesFloorReflection(id)
}

// The upside-down sprite's opaque foot row must meet the upright sprite's
// contact point. This projection is shared with the town-puddle reflections.
func mobileReflectionTop(footY, reflectedHeight, footFraction float64) float64 {
	return footY + reflectedHeight*footFraction
}

type mobileReflectionPose struct {
	image      *ebiten.Image
	influence  *ebiten.Image
	palette    *mobilePaletteShaderState
	gpuRecolor bool
	footRow    float64
	state      uint8
}

func loadMobileReflectionPose(desc frameDescriptor, state uint8) (mobileReflectionPose, bool) {
	colors := playerColorsForDescriptor(desc)
	img, influence, palette, gpuRecolor := loadGPURecoloredMobileFrame(desc.PictID, state, colors)
	metricsKey := makeMobileKey(desc.PictID, state, colors)
	if !gpuRecolor {
		img = loadMobileFrame(desc.PictID, state, colors)
		img = getScaledMobileFrame(metricsKey, img)
	} else {
		metricsKey = mobileRecolorSharedKey(desc.PictID, state)
	}
	if img == nil || img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		return mobileReflectionPose{}, false
	}
	return mobileReflectionPose{
		image: img, influence: influence, palette: palette, gpuRecolor: gpuRecolor,
		footRow: float64(mobileSpriteMetricsFor(metricsKey, img).footFraction), state: state,
	}, true
}

func mobileReflectionPoseSkew(state uint8, drawWidth, reflectedHeight float64) float64 {
	if state >= poseDead || drawWidth <= 0 || reflectedHeight <= 0 {
		return 0
	}
	// Facing groups contain four synchronized walk poses. Lean the reflected
	// upper body slightly toward its facing side, with the feet kept fixed.
	facing := [...]float64{1, 0.7, 0, -0.7, -1, -0.7, 0, 0.7}
	lean := facing[int(state)/4] * drawWidth * 0.055
	return lean * math.Min(1, reflectedHeight/(drawWidth*0.72))
}

func drawMobileReflectionSprite(target *ebiten.Image, pose mobileReflectionPose, options frameBlendDrawOptions) {
	if target == nil || pose.image == nil {
		return
	}
	bounds := pose.image.Bounds()
	if bounds.Empty() {
		return
	}
	// The opaque foot row is the reflection's contact point. Do not mirror the
	// unused pixels beneath it; dissolve the last few reflected pixels instead.
	contact := math.Min(float64(bounds.Dy()), math.Max(1, math.Ceil(pose.footRow*float64(bounds.Dy()))))
	fadeStart := math.Max(0, contact-math.Max(2, float64(bounds.Dy())*0.10))
	left, right := float32(options.Left), float32(options.Left+float64(bounds.Dx())*options.ScaleX)
	skew := mobileReflectionPoseSkew(pose.state, math.Abs(float64(bounds.Dx())*options.ScaleX), math.Abs(float64(bounds.Dy())*options.ScaleY))
	red, green, blue, alpha := premultipliedDrawColor(options.Red, options.Green, options.Blue, options.Alpha)
	rows := [...]float64{0, fadeStart, contact}
	var vertices [6]ebiten.Vertex
	for row, offset := range rows {
		dstY := float32(options.Top + offset*options.ScaleY)
		srcY := float32(float64(bounds.Min.Y) + offset)
		rowSkew := float32(skew * (1 - offset/contact))
		strength := float32(1)
		if row == 2 {
			strength = 0
		}
		for column, dstX := range [...]float32{left, right} {
			v := &vertices[row*2+column]
			v.DstX, v.DstY = dstX+rowSkew, dstY
			v.SrcX, v.SrcY = float32(bounds.Min.X), srcY
			if column == 1 {
				v.SrcX = float32(bounds.Max.X)
			}
			v.ColorR, v.ColorG = red*strength, green*strength
			v.ColorB, v.ColorA = blue*strength, alpha*strength
		}
	}
	indices := [...]uint32{0, 1, 2, 1, 3, 2, 2, 3, 4, 3, 5, 4}
	if pose.gpuRecolor && pose.influence != nil && pose.palette != nil && mobileRecolorShader != nil {
		linear := float32(0)
		if options.Linear {
			linear = 1
		}
		for i := range vertices {
			vertices[i].Custom0 = linear
		}
		op := &pose.palette.op
		op.Images[0], op.Images[1] = pose.image, pose.influence
		pose.palette.flash = options.FlashColor
		target.DrawTrianglesShader32(vertices[:], indices[:], mobileRecolorShader, op)
		return
	}
	op := &ebiten.DrawTrianglesOptions{Address: ebiten.AddressClampToZero, DisableMipmaps: true}
	if options.Linear {
		op.Filter = ebiten.FilterLinear
	} else {
		op.Filter = ebiten.FilterNearest
	}
	target.DrawTriangles32(vertices[:], indices[:], pose.image, op)
}

func mobileReflectionFootOverlap(drawSize, reflectedHeight float64) float64 {
	if drawSize <= 0 || reflectedHeight <= 0 {
		return 0
	}
	// Match the accepted floor join; shallower reflections need less overlap so
	// the fading foot pixels do not recover full opacity at the contact point.
	return math.Min(2*gs.GameScale, drawSize*0.04) * math.Min(1, reflectedHeight/(drawSize*0.72))
}

func mobilePoseCanReflect(state uint8) bool {
	return state != poseDead && state != poseLie
}

func mobileReflectionEligibleForFlags(state uint8, flags uint32) bool {
	return mobilePoseCanReflect(state) && flags&climg.PictDefFlagUprightShadow != 0
}

func mobileArtworkCanReflect(pictID uint16, state uint8) bool {
	return clImages != nil && mobileReflectionEligibleForFlags(state, clImages.Flags(uint32(pictID)))
}

func drawFloorPictureMobileReflections(screen *ebiten.Image, profile floorReflectionProfile, left, top, width, height float64, pictureAlpha float32, ox, oy int, mobiles []frameMobile, descriptors map[uint8]frameDescriptor, prevMobiles map[uint8]frameMobile, shiftX, shiftY int, alpha float64) {
	if screen == nil || gs.hideMobiles || pictureAlpha <= 0 || width <= 0 || height <= 0 || len(mobiles) == 0 {
		return
	}
	clipBounds := image.Rect(
		int(math.Floor(left)), int(math.Floor(top)),
		int(math.Floor(left+width)), int(math.Floor(top+height)),
	).Intersect(screen.Bounds())
	if clipBounds.Empty() {
		return
	}
	var clip *ebiten.Image
	for _, mobile := range mobiles {
		desc, ok := descriptors[mobile.Index]
		if !ok || desc.PictID == 0 || !mobileArtworkCanReflect(desc.PictID, mobile.State) {
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
		reflectedHeight := drawSize * profile.heightScale
		// Reject distant mobiles before loading their pose. A reflection may
		// cross a tile seam, so include its facing lean at adjacent tile edges.
		approximateFootY := mobileY + drawSize*0.40
		skewReach := math.Abs(mobileReflectionPoseSkew(mobile.State, drawSize, reflectedHeight))
		if mobileX+drawSize/2+skewReach <= float64(clipBounds.Min.X) || mobileX-drawSize/2-skewReach >= float64(clipBounds.Max.X) ||
			approximateFootY-reflectedHeight*0.20 >= float64(clipBounds.Max.Y) || approximateFootY+reflectedHeight <= float64(clipBounds.Min.Y) {
			continue
		}
		pose, ok := loadMobileReflectionPose(desc, mobile.State)
		if !ok {
			continue
		}
		footY := mobileY - drawSize/2 + drawSize*pose.footRow
		reflectionTop := mobileReflectionTop(footY, reflectedHeight, pose.footRow) - mobileReflectionFootOverlap(drawSize, reflectedHeight)
		if reflectionTop-reflectedHeight >= float64(clipBounds.Max.Y) || reflectionTop <= float64(clipBounds.Min.Y) {
			continue
		}
		if clip == nil {
			clip = screen.RecyclableSubImage(clipBounds)
			defer clip.Recycle()
		}
		drawMobileReflectionSprite(clip, pose, frameBlendDrawOptions{
			Left: mobileX - drawSize/2, Top: reflectionTop,
			ScaleX: drawSize / float64(pose.image.Bounds().Dx()),
			ScaleY: -reflectedHeight / float64(pose.image.Bounds().Dy()),
			Red:    profile.red, Green: profile.green, Blue: profile.blue,
			Alpha:  profile.alpha * pictureAlpha,
			Linear: worldArtworkFilter() == ebiten.FilterLinear,
		})
	}
}
