package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Test that preparePiper extracts voice archives and removes them.
func TestPreparePiperVoiceArchive(t *testing.T) {
	dataDir := t.TempDir()
	piperDir := filepath.Join(dataDir, "piper")
	binDir := filepath.Join(piperDir, "bin")
	if err := os.MkdirAll(filepath.Join(binDir, "piper"), 0o755); err != nil {
		t.Fatal(err)
	}
	// create dummy piper binary inside nested directory to skip download
	binName := "piper"
	if runtime.GOOS == "windows" {
		binName = "piper.exe"
	}
	binPath := filepath.Join(binDir, "piper", binName)
	if err := os.WriteFile(binPath, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	voicesDir := filepath.Join(piperDir, "voices")
	if err := os.MkdirAll(voicesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// create voice tarball
	tarPath := filepath.Join(voicesDir, piperFemaleVoice+".tar.gz")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		filepath.Join(piperFemaleVoice, piperFemaleVoice+".onnx"):      "",
		filepath.Join(piperFemaleVoice, piperFemaleVoice+".onnx.json"): "{}",
	}
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// run preparePiper which should extract and remove tarball
	path, model, cfg, err := preparePiper(dataDir)
	if err != nil {
		t.Fatalf("preparePiper: %v", err)
	}
	if path != binPath {
		t.Fatalf("bin path = %v, want %v", path, binPath)
	}
	if _, err := os.Stat(tarPath); !os.IsNotExist(err) {
		t.Fatalf("archive not removed")
	}
	if _, err := os.Stat(model); err != nil {
		t.Fatalf("model missing: %v", err)
	}
	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("config missing: %v", err)
	}
}

// Test that preparePiper locates the piper binary inside a nested directory
// created by the archive.
func TestPreparePiperNestedDir(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("test only runs on linux/amd64")
	}
	dataDir := t.TempDir()
	piperDir := filepath.Join(dataDir, "piper")
	voicesDir := filepath.Join(piperDir, "voices")
	if err := os.MkdirAll(voicesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdir := filepath.Join(voicesDir, piperFemaleVoice)
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vdir, piperFemaleVoice+".onnx"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vdir, piperFemaleVoice+".onnx.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	archPath := filepath.Join(piperDir, "piper_linux_x86_64.tar.gz")
	if err := os.MkdirAll(piperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(archPath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "piper/", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "piper/piper", Mode: 0o755, Size: 0}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	binPath, _, _, err := preparePiper(dataDir)
	if err != nil {
		t.Fatalf("preparePiper: %v", err)
	}
	fi, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("binary missing: %v", err)
	}
	if fi.IsDir() {
		t.Fatalf("expected binary file, got directory: %s", binPath)
	}
}

// Test that preparePiper finds voices inside directories with different names.
func TestPreparePiperMismatchedVoiceDir(t *testing.T) {
	dataDir := t.TempDir()
	piperDir := filepath.Join(dataDir, "piper")
	binDir := filepath.Join(piperDir, "bin")
	if err := os.MkdirAll(filepath.Join(binDir, "piper"), 0o755); err != nil {
		t.Fatal(err)
	}
	binName := "piper"
	if runtime.GOOS == "windows" {
		binName = "piper.exe"
	}
	binPath := filepath.Join(binDir, "piper", binName)
	if err := os.WriteFile(binPath, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	voicesDir := filepath.Join(piperDir, "voices")
	mismatch := filepath.Join(voicesDir, "mismatch")
	if err := os.MkdirAll(mismatch, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mismatch, "voice.onnx"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mismatch, "voice.onnx.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := gs.ChatTTSVoice
	gs.ChatTTSVoice = "voice"
	defer func() { gs.ChatTTSVoice = orig }()

	gotBin, model, cfg, err := preparePiper(dataDir)
	if err != nil {
		t.Fatalf("preparePiper: %v", err)
	}
	if gotBin != binPath {
		t.Fatalf("bin path = %v, want %v", gotBin, binPath)
	}
	wantModel := filepath.Join(mismatch, "voice.onnx")
	wantCfg := filepath.Join(mismatch, "voice.onnx.json")
	if model != wantModel {
		t.Fatalf("model = %v, want %v", model, wantModel)
	}
	if cfg != wantCfg {
		t.Fatalf("cfg = %v, want %v", cfg, wantCfg)
	}
}
