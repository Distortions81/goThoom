package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"
)

func TestHDPictureSourceScale(t *testing.T) {
	sx, sy := pictureSourceScale(42, 42, 168, 168)
	if sx != 0.25 || sy != 0.25 {
		t.Fatalf("HD picture source scale = (%v, %v), want (0.25, 0.25)", sx, sy)
	}
}

func TestHDPictureSourcesFindSubfoldersAndZIPEntries(t *testing.T) {
	pngData := hdPictureTestPNG(t)
	var zipped bytes.Buffer
	writer := zip.NewWriter(&zipped)
	for _, name := range []string{"art/309.png", "477.png", "1068.png", "not-an-id.png"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(pngData); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{
		"data/hdimg/1068.png":           {Data: pngData},
		"data/hdimg/alpha/477.png":      {Data: pngData},
		"data/hdimg/tiles/1068.png":     {Data: pngData},
		"data/hdimg/tiles/477.png":      {Data: pngData},
		"data/hdimg/bundles/staffs.zip": {Data: zipped.Bytes()},
	}
	sources := indexHDPictureSources(files)
	for id, want := range map[uint16]string{
		1068: "data/hdimg/1068.png",
		477:  "data/hdimg/alpha/477.png",
		309:  "data/hdimg/bundles/staffs.zip!art/309.png",
	} {
		source, ok := sources[id]
		if !ok || source.label != want {
			t.Errorf("HD picture %d source = (%q, %v), want %q", id, source.label, ok, want)
		}
	}
	if _, ok := sources[0]; ok {
		t.Fatal("non-numeric ZIP entry was indexed as a picture")
	}
	reader, err := sources[309].open()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	img, err := png.Decode(reader)
	if err != nil || img.Bounds().Dx() != 2 {
		t.Fatalf("nested ZIP entry did not decode as HD PNG: image=%v err=%v", img, err)
	}
}

func TestHDPictureSourcesSupportUserDataFolder(t *testing.T) {
	files := fstest.MapFS{
		"hdimg/42.png": {Data: hdPictureTestPNG(t)},
	}
	source, ok := indexHDPictureSourcesInFolder(files, "hdimg")[42]
	if !ok || source.label != "hdimg/42.png" {
		t.Fatalf("user-data source = (%q, %v), want hdimg/42.png", source.label, ok)
	}
	reader, err := source.open()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := png.Decode(reader); err != nil {
		t.Fatalf("user-data sprite did not decode: %v", err)
	}
}

func TestHDPictureAvailabilityRequiresSpritePackSetting(t *testing.T) {
	originalSettings, originalSources, originalImages := gs, hdPictureSources, clImages
	t.Cleanup(func() {
		gs, hdPictureSources, clImages = originalSettings, originalSources, originalImages
	})
	gs.UseSpritePackFiles = false
	hdPictureSources = map[uint16]hdPictureSource{42: {label: "data/hdimg/42.png"}}
	clImages = nil
	if hdPictureAvailableLocked(42) {
		t.Fatal("sprite pack image was available while sprite packs were disabled")
	}
	gs.UseSpritePackFiles = true
	if !hdPictureAvailableLocked(42) {
		t.Fatal("single-frame sprite pack image was unavailable after enabling sprite packs")
	}
}

func hdPictureTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 100, G: 150, B: 200, A: 255})
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
