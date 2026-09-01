//go:build !js

package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/sqweek/dialog"
)

var errStorageDirectoryDialogCancelled = errors.New("storage directory dialog cancelled")

func pickStorageDirectory(title, start string) (string, error) {
	start = existingDirectory(start)
	builder := dialog.Directory().Title(title)
	if start != "" {
		builder.SetStartDir(start)
	}
	directory, err := builder.Browse()
	if err != nil {
		if errors.Is(err, dialog.Cancelled) {
			return "", errStorageDirectoryDialogCancelled
		}
		return "", err
	}
	return directory, nil
}

func existingDirectory(path string) string {
	path = filepath.Clean(path)
	for path != "." && path != "" {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return ""
}
