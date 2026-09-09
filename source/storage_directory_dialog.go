//go:build !js

package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ncruces/zenity"
)

var errStorageDirectoryDialogCancelled = errors.New("storage directory dialog cancelled")

func pickStorageDirectory(title, start string) (string, error) {
	start = existingDirectory(start)
	options := []zenity.Option{zenity.Directory(), zenity.Title(title)}
	if start != "" {
		options = append(options, zenity.Filename(start+string(os.PathSeparator)))
	}
	directory, err := zenity.SelectFile(options...)
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
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
