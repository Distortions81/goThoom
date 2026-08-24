package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitScriptIDSurvivesMetadataAndFilenameChanges(t *testing.T) {
	dir := t.TempDir()
	write := func(filename, name, author string) {
		t.Helper()
		source := `package main
const scriptID = "my-tool"
const scriptName = "` + name + `"
const scriptAuthor = "` + author + `"
const scriptDescription = "A friendly tool"
func Init() {}
`
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(source), 0o644); err != nil {
			t.Fatalf("write script: %v", err)
		}
	}

	write("first.go", "First Name", "First Author")
	first := scanscripts([]string{dir}, nil)
	if info, ok := first["my-tool"]; !ok || info.invalid || info.name != "First Name" || info.description != "A friendly tool" {
		t.Fatalf("first scan = %+v", first)
	}
	firstPath := scriptStoragePath("my-tool")
	if err := os.Remove(filepath.Join(dir, "first.go")); err != nil {
		t.Fatal(err)
	}
	write("renamed.go", "Better Name", "Different Author")
	second := scanscripts([]string{dir}, nil)
	if info, ok := second["my-tool"]; !ok || info.invalid || info.name != "Better Name" || info.author != "Different Author" {
		t.Fatalf("second scan = %+v", second)
	}
	if secondPath := scriptStoragePath("my-tool"); secondPath != firstPath {
		t.Fatalf("storage path changed from %q to %q", firstPath, secondPath)
	}
}

func TestScriptIDDefaultsToFilename(t *testing.T) {
	if got := normalizeScriptID("My Friendly_Script.go"); got != "my-friendly_script-go" {
		t.Fatalf("normalized ID = %q", got)
	}
	if got := normalizeScriptID("bad/id"); got != "" {
		t.Fatalf("invalid ID normalized to %q", got)
	}
}
