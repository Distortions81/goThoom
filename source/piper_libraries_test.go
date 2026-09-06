package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Opt in with an installed Piper directory to verify repair and real synthesis.
func TestPiperRuntimeSmoke(t *testing.T) {
	dir := os.Getenv("GOTHOOM_PIPER_SMOKE_DIR")
	if dir == "" {
		t.Skip("set GOTHOOM_PIPER_SMOKE_DIR to an installed Piper directory")
	}
	oldPath, oldModel, oldConfig := piperPath, piperModel, piperConfig
	oldSpeed := gs.ChatTTSSpeed
	t.Cleanup(func() {
		piperPath, piperModel, piperConfig = oldPath, oldModel, oldConfig
		gs.ChatTTSSpeed = oldSpeed
	})
	var err error
	piperPath, piperModel, piperConfig, err = preparePiperDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	gs.ChatTTSSpeed = 1
	wav, err := synthesizeWithPiper("Speech is working.")
	if err != nil {
		t.Fatal(err)
	}
	if len(wav) <= 44 || !bytes.Equal(wav[:4], []byte("RIFF")) || !bytes.Equal(wav[8:12], []byte("WAVE")) {
		t.Fatalf("Piper did not produce WAV audio (%d bytes)", len(wav))
	}
}

func TestRepairPiperLibraryLinks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux shared libraries")
	}
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("C compiler needed to create an ELF library fixture")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "library.c")
	if err := os.WriteFile(source, []byte("int piper_test(void) { return 42; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	library := filepath.Join(dir, "libexample.so.1.2.3")
	cmd := exec.Command(cc, "-shared", "-fPIC", "-Wl,-soname,libexample.so.1", "-o", library, source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build library fixture: %v: %s", err, output)
	}
	for range 2 {
		if err := repairPiperLibraryLinks(dir); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "libexample.so.1")
		if target, err := os.Readlink(link); err != nil || target != filepath.Base(library) {
			t.Fatalf("library link = %q, %v", target, err)
		}
		if _, err := os.Stat(link); err != nil {
			t.Fatalf("library link does not resolve: %v", err)
		}
	}
}
