package main

import (
	"archive/zip"
	"bytes"
	"image/png"
	"testing"
	"testing/fstest"
)

func TestHDPictureBundleAndSourceScale(t *testing.T) {
	for _, item := range []struct {
		id     uint16
		label  string
		size   int
		opaque bool
	}{
		{23, "data/hdimg/held/23.png", 168, false},
		{208, "data/hdimg/held/208.png", 168, false},
		{210, "data/hdimg/held/210.png", 168, false},
		{417, "data/hdimg/held/417.png", 168, false},
		{626, "data/hdimg/held/626.png", 168, false},
		{635, "data/hdimg/held/635.png", 168, false},
		{738, "data/hdimg/held/738.png", 168, false},
		{1068, "data/hdimg/held/1068.png", 168, false},
		{1279, "data/hdimg/held/1279.png", 168, false},
		{2252, "data/hdimg/held/2252.png", 168, false},
		{307, "data/hdimg/ground/307.png", 800, true},
		{317, "data/hdimg/ground/317.png", 800, true},
		{4495, "data/hdimg/held/4495.png", 168, false},
		{5764, "data/hdimg/ground/5764.png", 800, true},
	} {
		source, ok := hdPictureSources[item.id]
		if !ok || source.label != item.label {
			t.Fatalf("HD picture %d source = (%q, %v), want %q", item.id, source.label, ok, item.label)
		}
		reader, err := source.open(hdPictureFiles)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if got := img.Bounds().Size(); got.X != item.size || got.Y != item.size {
			t.Fatalf("HD picture %d size = %v, want %dx%d", item.id, got, item.size, item.size)
		}
		if _, _, _, alpha := img.At(0, 0).RGBA(); item.opaque && alpha != 0xffff {
			t.Fatalf("HD picture %d has a transparent tile edge", item.id)
		} else if !item.opaque && alpha != 0 {
			t.Fatalf("HD picture %d lost its transparent background", item.id)
		}
	}
	sx, sy := pictureSourceScale(42, 42, 168, 168)
	if sx != 0.25 || sy != 0.25 {
		t.Fatalf("HD picture source scale = (%v, %v), want (0.25, 0.25)", sx, sy)
	}
}

func TestHDPictureSourcesFindSubfoldersAndZIPEntries(t *testing.T) {
	pngData, err := hdPictureFiles.ReadFile("data/hdimg/held/1068.png")
	if err != nil {
		t.Fatal(err)
	}
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
	reader, err := sources[309].open(files)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	img, err := png.Decode(reader)
	if err != nil || img.Bounds().Dx() != 168 {
		t.Fatalf("nested ZIP entry did not decode as HD PNG: image=%v err=%v", img, err)
	}
}
