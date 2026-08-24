package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewScriptTemplatesAreUniqueAndCompile(t *testing.T) {
	dir := t.TempDir()
	for _, template := range newScriptTemplates {
		path, err := createScriptFromTemplate(dir, template)
		if err != nil {
			t.Fatalf("create %s template: %v", template.name, err)
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := compileScriptSource(normalizeScriptID(strings.TrimSuffix(filepath.Base(path), ".go")), source, restrictedStdlib())
		if err != nil {
			t.Fatalf("compile %s template: %v", template.name, err)
		}
		disposePreparedScript(prepared)
	}

	first := newScriptTemplates[0]
	secondPath, err := createScriptFromTemplate(dir, first)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(secondPath) != first.filename+"_2.go" {
		t.Fatalf("second template path = %q", secondPath)
	}
	source, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `const scriptName = "Command 2"`) {
		t.Fatalf("second template did not get a unique display name:\n%s", source)
	}
}
