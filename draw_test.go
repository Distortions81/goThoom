package main

import (
	"encoding/binary"
	"reflect"
	"testing"
	"unsafe"

	"gothoom/climg"
)

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
	gs.NoCaching = false
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
