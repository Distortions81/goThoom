package main

import (
	"encoding/binary"
	"image/color"
	"math"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"gothoom/climg"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestReuseCachedNameTagFromPreviousFrame(t *testing.T) {
	clearSharedNameTagCache()
	t.Cleanup(clearSharedNameTagCache)
	key := nameTagKey{Text: "Busy", FontGen: 7}
	image := new(ebiten.Image)
	previous := map[uint8]frameMobile{
		3: {Index: 3, nameTag: image, nameTagW: 42, nameTagH: 12, nameTagKey: key},
	}
	mobile := frameMobile{Index: 3}
	if !reuseCachedNameTag(&mobile, previous, key) {
		t.Fatal("matching previous-frame name tag was not reused")
	}
	if mobile.nameTag != image || mobile.nameTagW != 42 || mobile.nameTagH != 12 || mobile.nameTagKey != key {
		t.Fatal("previous-frame name tag cache was not copied completely")
	}
	if reuseCachedNameTag(&mobile, previous, nameTagKey{Text: "Changed", FontGen: 7}) {
		t.Fatal("name tag with a changed key was reused")
	}
}

func TestRelativeNameTagScaleFollowsDisplayedPlayerScale(t *testing.T) {
	if got := relativeNameTagScale(2, 4); got != 0.5 {
		t.Fatalf("relativeNameTagScale(2, 4) = %v, want 0.5", got)
	}
	if got := relativeNameTagScale(4, 2); got != 2 {
		t.Fatalf("relativeNameTagScale(4, 2) = %v, want 2", got)
	}
	if got := relativeNameTagScale(0, 4); got != 1 {
		t.Fatalf("relativeNameTagScale(0, 4) = %v, want 1", got)
	}
}

func TestNameTagCacheSeparatesNativeRasterScales(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.NameHealthBarModern = true

	low := makeNameTagKey("Native", 0, kDescPlayer, 200, styleRegular, false, 2)
	high := makeNameTagKey("Native", 0, kDescPlayer, 200, styleRegular, false, 3.5)
	if low == high {
		t.Fatal("name tags at different display scales shared a cache key")
	}
	if got := nameTagRasterScaleFromKey(high.RasterScale); got != 3.5 {
		t.Fatalf("decoded raster scale = %v, want 3.5", got)
	}
	key, scale := quantizedNameTagRasterScale(2.333)
	if key == 0 || math.Abs(scale-2.333) > 1.0/(2*nameTagRasterScaleUnits) {
		t.Fatalf("quantized raster scale = key %d scale %v", key, scale)
	}
}

func TestReuseCachedNameTagFromSharedCache(t *testing.T) {
	clearSharedNameTagCache()
	t.Cleanup(clearSharedNameTagCache)
	key := nameTagKey{Text: "Returning", FontGen: 9, FrameColor: color.RGBA{R: 12, G: 34, B: 56, A: 200}}
	image := new(ebiten.Image)
	sharedNameTagCache[key] = cachedNameTagImage{image: image, width: 48, height: 14}

	mobile := frameMobile{Index: 7}
	if !reuseCachedNameTag(&mobile, nil, key) {
		t.Fatal("shared name tag was not reused")
	}
	if mobile.nameTag != image || mobile.nameTagW != 48 || mobile.nameTagH != 14 || mobile.nameTagKey != key {
		t.Fatal("shared name tag cache was not copied completely")
	}
	changedFrame := key
	changedFrame.FrameColor.R++
	if reuseCachedNameTag(&frameMobile{Index: 7}, nil, changedFrame) {
		t.Fatal("name tag with a changed frame color was reused")
	}

	clearSharedNameTagCacheFor(key.Text)
	if reuseCachedNameTag(&frameMobile{Index: 7}, nil, key) {
		t.Fatal("name-specific cache clear retained the shared name tag")
	}
}

func TestModernNameTagKeyIgnoresSeparateHealthBarColor(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.NameHealthBarModern = true
	gs.DarkBubblesAndNames = true

	green := makeNameTagKey("Healthy", uint8(kColorCodeBackGreen<<4), kDescPlayer, 200, styleRegular, false, 1)
	red := makeNameTagKey("Healthy", uint8(kColorCodeBackRed<<4), kDescPlayer, 200, styleRegular, false, 1)
	if green != red {
		t.Fatal("modern dark name-tag surface key changed with separately drawn health color")
	}
	dark := green

	gs.DarkBubblesAndNames = false
	green = makeNameTagKey("Healthy", uint8(kColorCodeBackGreen<<4), kDescPlayer, 200, styleRegular, false, 1)
	red = makeNameTagKey("Healthy", uint8(kColorCodeBackRed<<4), kDescPlayer, 200, styleRegular, false, 1)
	if green != red {
		t.Fatal("modern light name-tag surface key changed with separately drawn health color")
	}
	if green == dark {
		t.Fatal("modern light and dark name tags shared a cache key")
	}

	gs.NameHealthBarModern = false
	green = makeNameTagKey("Healthy", uint8(kColorCodeBackGreen<<4), kDescPlayer, 200, styleRegular, false, 1)
	red = makeNameTagKey("Healthy", uint8(kColorCodeBackRed<<4), kDescPlayer, 200, styleRegular, false, 1)
	if green == red {
		t.Fatal("classic name-tag surface key ignored its background color")
	}
}

func TestModernNameColorsAreIndependentFromHealth(t *testing.T) {
	originalSettings := gs
	t.Cleanup(func() { gs = originalSettings })
	gs.NameHealthBarModern = true

	gs.DarkBubblesAndNames = false
	lightText, lightBG, _ := mobileNameColors(uint8(kColorCodeBackRed<<4), kDescPlayer)
	if lightText != (color.RGBA{A: 0xff}) || lightBG != (color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) {
		t.Fatalf("modern light name colors = text %v, background %v", lightText, lightBG)
	}

	gs.DarkBubblesAndNames = true
	darkText, darkBG, _ := mobileNameColors(uint8(kColorCodeBackRed<<4), kDescPlayer)
	if darkText != (color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}) || darkBG != (color.RGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff}) {
		t.Fatalf("modern dark name colors = text %v, background %v", darkText, darkBG)
	}

	gs.NameHealthBarModern = false
	classicText, classicBG, _ := mobileNameColors(uint8(kColorCodeBackRed<<4), kDescPlayer)
	wantText, wantBG, _ := classicMobileNameColors(uint8(kColorCodeBackRed << 4))
	if classicText != wantText || classicBG != wantBG {
		t.Fatalf("classic player name colors = text %v, background %v", classicText, classicBG)
	}
}

func TestPictureObscuringFadeUsesCachedUpdateStates(t *testing.T) {
	const opacity = float32(0.4)
	if got := pictureObscuringFadeAlpha(false, true, opacity, 0.25); got != 0.85 {
		t.Fatalf("fade into obscuring = %v, want 0.85", got)
	}
	if got := pictureObscuringFadeAlpha(true, false, opacity, 0.25); got != 0.55 {
		t.Fatalf("fade out of obscuring = %v, want 0.55", got)
	}
	if got := pictureObscuringFadeAlpha(true, true, opacity, 0.75); got != opacity {
		t.Fatalf("steady obscuring = %v, want %v", got, opacity)
	}
}

func TestPictureEligibleForObscuringIncludesPlaneZero(t *testing.T) {
	original := pictureSemiTransparent
	pictureSemiTransparent = func(uint16) bool { return false }
	t.Cleanup(func() { pictureSemiTransparent = original })

	if !pictureEligibleForObscuring(framePicture{PictID: 1, Plane: 0}) {
		t.Fatal("plane-zero foreground picture was excluded from mobile occlusion")
	}
	if pictureEligibleForObscuring(framePicture{PictID: 1, Plane: -1}) {
		t.Fatal("negative-plane background picture was included in mobile occlusion")
	}
	pictureSemiTransparent = func(uint16) bool { return true }
	if pictureEligibleForObscuring(framePicture{PictID: 1, Plane: 1}) {
		t.Fatal("semi-transparent picture was included in mobile occlusion")
	}
}

func mockCLImages(w, h int) *climg.CLImages {
	imgs := &climg.CLImages{}
	v := reflect.ValueOf(imgs).Elem()

	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], uint16(h))
	binary.BigEndian.PutUint16(data[2:], uint16(w))
	dataField := v.FieldByName("data")
	reflect.NewAt(dataField.Type(), unsafe.Pointer(dataField.UnsafeAddr())).Elem().Set(reflect.ValueOf(data))

	idrefsField := v.FieldByName("idrefs")
	imagesField := v.FieldByName("images")
	idrefsMap := reflect.MakeMap(idrefsField.Type())
	imagesMap := reflect.MakeMap(imagesField.Type())

	dlType := idrefsField.Type().Elem().Elem()
	idref := reflect.New(dlType)
	imageIDField := idref.Elem().FieldByName("imageID")
	reflect.NewAt(imageIDField.Type(), unsafe.Pointer(imageIDField.UnsafeAddr())).Elem().SetUint(1)
	idrefsMap.SetMapIndex(reflect.ValueOf(uint32(1)), idref)

	imgLoc := reflect.New(dlType)
	imagesMap.SetMapIndex(reflect.ValueOf(uint32(1)), imgLoc)

	reflect.NewAt(idrefsField.Type(), unsafe.Pointer(idrefsField.UnsafeAddr())).Elem().Set(idrefsMap)
	reflect.NewAt(imagesField.Type(), unsafe.Pointer(imagesField.UnsafeAddr())).Elem().Set(imagesMap)

	return imgs
}

func TestPictureOnEdge(t *testing.T) {
	halfW := 5
	halfH := 5

	tests := []struct {
		name string
		p    framePicture
		w    int
		h    int
		want bool
	}{
		{"inside", framePicture{PictID: 1, H: 0, V: 0}, 10, 10, false},
		{"left 80% off", framePicture{PictID: 1, H: int16(-fieldCenterX - 8 + halfW), V: 0}, 10, 10, true},
		{"left 60% off", framePicture{PictID: 1, H: int16(-fieldCenterX - 6 + halfW), V: 0}, 10, 10, false},
		{"corner 80% off", framePicture{PictID: 1, H: int16(-fieldCenterX - 8 + halfW), V: int16(-fieldCenterY - 8 + halfH)}, 10, 10, true},
		{"corner 50% off", framePicture{PictID: 1, H: int16(-fieldCenterX - 3 + halfW), V: int16(-fieldCenterY - 3 + halfH)}, 10, 10, false},
		{"outside", framePicture{PictID: 1, H: int16(fieldCenterX + halfW + 1), V: 0}, 10, 10, false},
		{"spanning middle", framePicture{PictID: 1, H: 0, V: 0}, gameAreaSizeX * 2, gameAreaSizeY * 2, false},
		{"wide edge big", framePicture{PictID: 1, H: int16(-fieldCenterX + 150), V: 0}, 300, 10, false},
		{"tall edge big", framePicture{PictID: 1, H: 0, V: int16(-fieldCenterY + 150)}, 10, 300, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clImages = mockCLImages(tt.w, tt.h)
			defer func() { clImages = nil }()
			if got := pictureOnEdge(tt.p); got != tt.want {
				t.Fatalf("pictureOnEdge(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestMobileOnEdge(t *testing.T) {
	orig := mobileSizeFunc
	mobileSizeFunc = func(id uint16) int { return 10 }
	defer func() { mobileSizeFunc = orig }()

	half := 5
	d := frameDescriptor{PictID: 1}
	tests := []struct {
		name string
		m    frameMobile
		want bool
	}{
		{"inside", frameMobile{H: 0, V: 0}, false},
		{"left 80% off", frameMobile{H: int16(-fieldCenterX - 8 + half), V: 0}, true},
		{"left 60% off", frameMobile{H: int16(-fieldCenterX - 6 + half), V: 0}, false},
		{"corner 80% off", frameMobile{H: int16(-fieldCenterX - 8 + half), V: int16(-fieldCenterY - 8 + half)}, true},
		{"outside", frameMobile{H: int16(fieldCenterX + half + 1), V: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mobileOnEdge(tt.m, d); got != tt.want {
				t.Fatalf("mobileOnEdge(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestPictureShiftBackgroundCap(t *testing.T) {
	pixelCountMu.Lock()
	origCache := pixelCountCache
	pixelCountCache = map[uint16]int{
		1: 1000000,
		2: 60000,
		3: 60000,
	}
	pixelCountMu.Unlock()
	defer func() {
		pixelCountMu.Lock()
		pixelCountCache = origCache
		pixelCountMu.Unlock()
	}()

	prev := []framePicture{
		{PictID: 1, H: 0, V: 0},
		{PictID: 2, H: 10, V: 0},
		{PictID: 3, H: 20, V: 0},
	}
	cur := []framePicture{
		{PictID: 1, H: 0, V: 0},
		{PictID: 2, H: 15, V: 0},
		{PictID: 3, H: 25, V: 0},
	}

	dx, dy, _, ok := pictureShift(prev, cur, 100)
	if !ok || dx != 5 || dy != 0 {
		t.Fatalf("pictureShift = (%d,%d) ok=%v, want (5,0) true", dx, dy, ok)
	}
}

func TestHandleDrawStateParseErrorDoesNotAdvance(t *testing.T) {
	resetDrawState()
	ackFrame = 5
	resendFrame = 0
	lastAckFrame = 5
	m := make([]byte, 11)
	binary.BigEndian.PutUint16(m[0:2], 2)
	m[2] = 0
	binary.BigEndian.PutUint32(m[3:7], uint32(10))
	binary.BigEndian.PutUint32(m[7:11], 0)
	handleDrawState(m, false)
	if ackFrame != 5 {
		t.Fatalf("ackFrame = %d, want 5", ackFrame)
	}
	if resendFrame != 6 {
		t.Fatalf("resendFrame = %d, want 6", resendFrame)
	}
}

func minimalDrawStatePacket() []byte {
	const stateLen = 4
	m := make([]byte, 2+21+stateLen)
	binary.BigEndian.PutUint16(m[0:2], 2)
	body := m[2:]

	body[0] = 0
	binary.BigEndian.PutUint32(body[1:5], 1)
	binary.BigEndian.PutUint32(body[5:9], 1)
	body[9] = 0
	body[10] = 10
	body[11] = 10
	body[12] = 10
	body[13] = 10
	body[14] = 10
	body[15] = 10
	body[16] = 0
	body[17] = 0
	body[18] = 0
	binary.BigEndian.PutUint16(body[19:21], stateLen)
	copy(body[21:], []byte{0, 0, 0, 0})

	return m
}

func BenchmarkHandleDrawStateNoEncryption(b *testing.B) {
	origEncrypted := drawStateEncrypted
	drawStateEncrypted = false
	defer func() { drawStateEncrypted = origEncrypted }()

	ackFrame = 0
	resendFrame = 0
	lastAckFrame = 0
	frameCounter = 0

	resetDrawState()
	packet := minimalDrawStatePacket()

	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	handleDrawState(packet, true)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handleDrawState(packet, true)
	}
}

func TestMobileHealthBarColorUsesPlayerHealthBackground(t *testing.T) {
	redCode := uint8(kColorCodeBackRed << 4)
	got, ok := mobileHealthBarColor(redCode, kDescPlayer)
	if !ok || got != nameBackColors[kColorCodeBackRed] {
		t.Fatalf("red player health bar = (%v, %v)", got, ok)
	}
	if _, ok := mobileHealthBarColor(redCode, kDescMonster); ok {
		t.Fatal("monster name produced a player health bar")
	}
	if _, ok := mobileHealthBarColor(uint8(kColorCodeBackWhite<<4), kDescPlayer); ok {
		t.Fatal("full-health player produced a health bar")
	}
	if _, ok := mobileHealthBarColor(uint8(kColorCodeBackBlue<<4), kDescPlayer); ok {
		t.Fatal("blue player name produced a health bar")
	}
}

func TestNameHealthBarOffsets(t *testing.T) {
	nameY, barY := nameHealthBarOffsets(14, 2, true)
	if nameY != 2 || barY != 0 {
		t.Fatalf("above offsets = (%d, %d), want (2, 0)", nameY, barY)
	}
	nameY, barY = nameHealthBarOffsets(14, 2, false)
	if nameY != 0 || barY != 14 {
		t.Fatalf("below offsets = (%d, %d), want (0, 14)", nameY, barY)
	}
}

func TestNameTagHoverHoldAndFade(t *testing.T) {
	clearNameTagHoverReveals()
	t.Cleanup(clearNameTagHoverReveals)
	start := time.Unix(100, 0)
	if got := nameTagHoverAlpha(7, "Bob", true, start); got != 1 {
		t.Fatalf("hover alpha = %v, want 1", got)
	}
	if got := nameTagHoverAlpha(7, "Bob", false, start.Add(nameTagHoverHold)); got != 1 {
		t.Fatalf("hold alpha = %v, want 1", got)
	}
	mid := start.Add(nameTagHoverHold + nameTagHoverFade/2)
	if got := nameTagHoverAlpha(7, "Bob", false, mid); got != 0.5 {
		t.Fatalf("mid-fade alpha = %v, want 0.5", got)
	}
	if got := nameTagHoverAlpha(7, "Bob", false, start.Add(nameTagHoverHold+nameTagHoverFade)); got != 0 {
		t.Fatalf("finished alpha = %v, want 0", got)
	}
}
