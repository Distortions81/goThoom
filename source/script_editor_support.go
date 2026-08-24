package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	scriptWorkspaceModule = `// Managed by goThoom for script editor support.
module gothoom-scripts

go 1.26.6

require gt2 v0.0.0

replace gt2 => ./gt2
`
	scriptWorkspaceFile = `// Managed by goThoom for script editor support.
go 1.26.6

use (
	.
	./gt2
)
`
)

func installScriptEditorSupport(scriptsDir string) error {
	if scriptsDir == "" {
		return nil
	}
	files := []struct {
		path string
		data []byte
	}{
		{path: "go.mod", data: []byte(scriptWorkspaceModule)},
		{path: "go.work", data: []byte(scriptWorkspaceFile)},
	}
	for _, embedded := range embeddedGT2EditorFiles {
		files = append(files, struct {
			path string
			data []byte
		}{path: filepath.FromSlash(embedded.path), data: embedded.data})
	}
	for _, file := range files {
		destination := filepath.Join(scriptsDir, file.path)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("create script editor support folder: %w", err)
		}
		if err := os.WriteFile(destination, file.data, 0o644); err != nil {
			return fmt.Errorf("write script editor support file %s: %w", destination, err)
		}
	}
	return nil
}
