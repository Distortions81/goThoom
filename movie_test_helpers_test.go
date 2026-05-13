package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func movieFixturePath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	base := filepath.Join(filepath.Dir(file), "clmovFiles", name)
	if _, err := os.Stat(base); err == nil {
		return base
	}
	if _, err := os.Stat(base + ".zip"); err == nil {
		return base + ".zip"
	}
	t.Fatalf("movie fixture %q not found", name)
	return ""
}

func readMovieFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := movieFixturePath(t, name)
	if !strings.HasSuffix(path, ".zip") {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		return data
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if filepath.Base(f.Name) != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry: %v", err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry: %v", err)
		}
		return data
	}
	t.Fatalf("movie fixture %q not found in %s", name, path)
	return nil
}
