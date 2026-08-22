package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDataDirPathRelativeToExecutable(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("dataDirPath uses user home on darwin")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	want := filepath.Join(filepath.Dir(exe), "data")
	if dataDirPath != want {
		t.Fatalf("dataDirPath = %q, want %q", dataDirPath, want)
	}
}
