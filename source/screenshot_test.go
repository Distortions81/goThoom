package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotFilenames(t *testing.T) {
	for _, name := range []string{"", " ", ".", "..", "../outside", `folder\outside`, "bad:name", "CON", "lpt1.png", "bad\nname", "trailing."} {
		if _, err := snapshotFilename(name, false); err == nil {
			t.Errorf("accepted invalid filename %q", name)
		}
	}
	for _, tc := range []struct {
		name, want string
		jpeg       bool
	}{
		{" Friends in Puddleby ", "Friends in Puddleby.png", false},
		{"Friends.PNG", "Friends.png", false},
		{"Friends.jpeg", "Friends.png", false},
		{"Friends.png", "Friends.jpg", true},
	} {
		got, err := snapshotFilename(tc.name, tc.jpeg)
		if err != nil || got != tc.want {
			t.Errorf("filename(%q) = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestSnapshotNameTagsInvalidateWorldCache(t *testing.T) {
	old := pendingSnapshot
	t.Cleanup(func() { pendingSnapshot = old })
	pendingSnapshot = nil
	normal := currentWorldRenderKey(640, 480)
	pendingSnapshot = &snapshotRequest{hideNameTags: true}
	hidden := currentWorldRenderKey(640, 480)
	if normal == hidden || !snapshotHidesNameTags() {
		t.Fatal("snapshot would reuse a frame with name tags")
	}
	pendingSnapshot = nil
	if currentWorldRenderKey(640, 480) != normal || snapshotHidesNameTags() {
		t.Fatal("snapshot did not restore normal name tag rendering")
	}
}

func TestSaveSnapshotImagePreservesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "Friends.png")
	if err := os.WriteFile(original, []byte("keep existing snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	shot := image.NewRGBA(image.Rect(10, 20, 18, 26))
	shot.Set(10, 20, color.RGBA{R: 255, A: 255})
	for _, asJPEG := range []bool{false, true} {
		filename, err := saveSnapshotImage(shot, dir, "Friends.png", asJPEG)
		if err != nil {
			t.Fatal(err)
		}
		wantName, wantFormat := "Friends (2).png", "png"
		if asJPEG {
			wantName, wantFormat = "Friends.jpg", "jpeg"
		}
		if filepath.Base(filename) != wantName {
			t.Fatalf("saved %s; want %s", filename, wantName)
		}
		f, err := os.Open(filename)
		if err != nil {
			t.Fatal(err)
		}
		decoded, format, err := image.Decode(f)
		f.Close()
		if err != nil || format != wantFormat {
			t.Fatalf("decode: format=%s err=%v", format, err)
		}
		if decoded.Bounds().Dx() != 8 || decoded.Bounds().Dy() != 6 {
			t.Fatalf("snapshot bounds: %v", decoded.Bounds())
		}
	}
	data, err := os.ReadFile(original)
	if err != nil || string(data) != "keep existing snapshot" {
		t.Fatal("overwrote existing snapshot")
	}
}
