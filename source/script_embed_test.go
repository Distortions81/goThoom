package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserScriptSourcesExcludeTestFiles(t *testing.T) {
	entries, err := scriptScripts.ReadDir("scripts")
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
const scriptAPIVersion = 1
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
	if _, ok := scanned["Visible_visible"]; !ok {
		t.Fatalf("visible script missing: %v", scanned)
	}
}
