package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScriptEditorSupportResolvesGT2(t *testing.T) {
	dir := t.TempDir()
	userScript := filepath.Join(dir, "mine.go")
	const userSource = "package main\n\nimport \"gt2\"\n\nvar _ = gt2.CLVersion\n"
	if err := os.WriteFile(userScript, []byte(userSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installScriptEditorSupport(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "go.work", "gt2/go.mod", "gt2/pluginapi.go", "gt2/API_REFERENCE.md"} {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil || info.IsDir() {
			t.Errorf("installed support file %s: info=%v err=%v", name, info, err)
		}
	}
	data, err := os.ReadFile(userScript)
	if err != nil || string(data) != userSource {
		t.Fatalf("user script changed: data=%q err=%v", data, err)
	}

	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "./...")
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("installed gt2 workspace does not type-check: %v\n%s", err, output)
	}
}

func TestInstallScriptEditorSupportRefreshesManagedFiles(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "gt2", "pluginapi.go")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installScriptEditorSupport(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(stale)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "package gt2") || strings.Contains(string(data), "stale") {
		t.Fatalf("managed editor stub was not refreshed: %q", data)
	}
}

func TestEmbeddedGT2EditorFilesAreCurrent(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	sourceDir := filepath.Dir(thisFile)
	for _, embedded := range embeddedGT2EditorFiles {
		canonical, err := os.ReadFile(filepath.Join(sourceDir, filepath.FromSlash(embedded.path)))
		if err != nil {
			t.Fatalf("read canonical %s: %v", embedded.path, err)
		}
		if string(canonical) != string(embedded.data) {
			t.Fatalf("embedded %s is stale; run go generate in source/gt2", embedded.path)
		}
	}
}
