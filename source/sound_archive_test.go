package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCLSoundsArchiveUsesDataDirPath(t *testing.T) {
	originalDataDir := dataDirPath
	originalWASM := isWASM
	originalWASMData := wasmCLSoundsData
	t.Cleanup(func() {
		dataDirPath = originalDataDir
		isWASM = originalWASM
		wasmCLSoundsData = originalWASMData
	})

	dataDirPath = t.TempDir()
	isWASM = false
	wasmCLSoundsData = nil
	// A valid empty CL_Sounds keyfile: header, zero entries, and padding.
	data := []byte{0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if err := os.WriteFile(filepath.Join(dataDirPath, CL_SoundsFile), data, 0o644); err != nil {
		t.Fatal(err)
	}

	archive, err := loadCLSoundsArchive()
	if err != nil {
		t.Fatalf("load CL_Sounds from dataDirPath: %v", err)
	}
	if ids := archive.IDs(); len(ids) != 0 {
		t.Fatalf("loaded unexpected CL_Sounds archive with IDs %v", ids)
	}
}
