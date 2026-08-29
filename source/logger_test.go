package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateLogFilesKeepsBoundedHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, diagnosticsLogName)
	files := map[string]string{
		path:        "current",
		path + ".1": "previous",
		path + ".2": "oldest",
	}
	for name, contents := range files {
		if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := rotateLogFiles(path, 2); err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, path+".1", "current")
	assertFileContents(t, path+".2", "previous")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("current log still exists after rotation: %v", err)
	}
}

func TestRotatingLogWriterRotatesBeforeSizeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), diagnosticsLogName)
	w, err := newRotatingLogWriter(path, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("12345678")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("abcde")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	assertFileContents(t, path+".1", "12345678")
	assertFileContents(t, path, "abcde")
}

func TestRotatingLogWriterStartsANewSessionLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), diagnosticsLogName)
	if err := os.WriteFile(path, []byte("prior session"), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := newRotatingLogWriter(path, 1024, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("new session")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	assertFileContents(t, path+".1", "prior session")
	assertFileContents(t, path, "new session")
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
