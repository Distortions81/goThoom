package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScriptFileSnapshotDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.go")
	if err := os.WriteFile(first, []byte("package main\n// one\n"), 0o644); err != nil {
		t.Fatalf("write first script: %v", err)
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatalf("stat first script: %v", err)
	}

	baseline := snapshotScriptFiles([]string{dir})
	if !sameScriptFileSnapshot(baseline, snapshotScriptFiles([]string{dir})) {
		t.Fatal("unchanged directory produced a different snapshot")
	}

	// Preserve both size and timestamp to prove content edits are detected by
	// the snapshot hash rather than only filesystem metadata.
	if err := os.WriteFile(first, []byte("package main\n// two\n"), 0o644); err != nil {
		t.Fatalf("edit first script: %v", err)
	}
	if err := os.Chtimes(first, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("restore first script timestamp: %v", err)
	}
	if sameScriptFileSnapshot(baseline, snapshotScriptFiles([]string{dir})) {
		t.Fatal("content edit was not detected")
	}

	baseline = snapshotScriptFiles([]string{dir})
	backward := info.ModTime().Add(-time.Hour)
	if err := os.Chtimes(first, backward, backward); err != nil {
		t.Fatalf("move timestamp backward: %v", err)
	}
	if sameScriptFileSnapshot(baseline, snapshotScriptFiles([]string{dir})) {
		t.Fatal("backward timestamp change was not detected")
	}

	baseline = snapshotScriptFiles([]string{dir})
	second := filepath.Join(dir, "second.go")
	if err := os.WriteFile(second, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("add second script: %v", err)
	}
	if sameScriptFileSnapshot(baseline, snapshotScriptFiles([]string{dir})) {
		t.Fatal("addition was not detected")
	}

	baseline = snapshotScriptFiles([]string{dir})
	renamed := filepath.Join(dir, "renamed.go")
	if err := os.Rename(second, renamed); err != nil {
		t.Fatalf("rename second script: %v", err)
	}
	if sameScriptFileSnapshot(baseline, snapshotScriptFiles([]string{dir})) {
		t.Fatal("rename was not detected")
	}

	baseline = snapshotScriptFiles([]string{dir})
	if err := os.Remove(renamed); err != nil {
		t.Fatalf("delete renamed script: %v", err)
	}
	if sameScriptFileSnapshot(baseline, snapshotScriptFiles([]string{dir})) {
		t.Fatal("deletion was not detected")
	}
}
