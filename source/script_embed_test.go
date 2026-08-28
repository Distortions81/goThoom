package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUserScriptSourcesExcludeTestFiles(t *testing.T) {
	entries, err := scriptScripts.ReadDir(bundledScriptDir)
	if err != nil {
		t.Fatalf("read embedded scripts: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_test.go") {
			t.Fatalf("test source embedded in user scripts: %s", entry.Name())
		}
	}

	dir := t.TempDir()
	valid := `package main
const scriptName = "Visible"
const scriptAuthor = "Test"
const scriptCategory = "Tests"
const scriptAPIVersion = 2
`
	for _, name := range []string{"visible.go", "hidden_test.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(valid), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	scanned := scanscripts([]string{dir}, nil)
	if len(scanned) != 1 {
		t.Fatalf("scanned scripts = %v, want only visible.go", scanned)
	}
	if _, ok := scanned["visible"]; !ok {
		t.Fatalf("visible script missing: %v", scanned)
	}
}

func TestBundledScriptsCompileWithYaegi(t *testing.T) {
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		entry := entry
		t.Run(entry.Filename, func(t *testing.T) {
			source, err := scriptScripts.ReadFile(filepath.ToSlash(filepath.Join(bundledScriptDir, entry.Filename)))
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := compileScriptSource(entry.ID, source, restrictedStdlib())
			if err != nil {
				t.Fatalf("compile bundled script: %v", err)
			}
			disposePreparedScript(prepared)
		})
	}
}

func TestBundledScriptsTypeCheckWithEditorStub(t *testing.T) {
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := installScriptEditorSupport(dir); err != nil {
		t.Fatal(err)
	}
	for index, entry := range entries {
		source, err := scriptScripts.ReadFile(filepath.ToSlash(filepath.Join(bundledScriptDir, entry.Filename)))
		if err != nil {
			t.Fatal(err)
		}
		exampleDir := filepath.Join(dir, "examples", fmt.Sprintf("%02d", index))
		if err := os.MkdirAll(exampleDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(exampleDir, "script.go"), source, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-tags", "script", "./...")
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("bundled scripts do not type-check with the installed gt2 editor stub: %v\n%s", err, output)
	}
}

func TestPopulateBundledScriptsOnlySeedsEmptyFolder(t *testing.T) {
	dir := t.TempDir()
	if err := populateBundledScriptsIfEmpty(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := scriptLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(dir, entry.Filename)); err != nil {
			t.Errorf("seeded script %s: %v", entry.Filename, err)
		}
	}

	customDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(customDir, "mine.go"), []byte("package main\nfunc Init() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := populateBundledScriptsIfEmpty(customDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(customDir, entries[0].Filename)); !os.IsNotExist(err) {
		t.Fatalf("non-empty folder was seeded: %v", err)
	}
}
