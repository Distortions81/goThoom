//go:build !js

package main

import (
	"errors"
	"github.com/ncruces/zenity"
)

func pickBardTuneFile() (string, error) {
	path, err := zenity.SelectFile(zenity.Title("Import tune"), zenity.FileFilter{Name: "CL tune text", Patterns: []string{"*.tune", "*.txt", "*.TUNE", "*.TXT"}})
	if errors.Is(err, zenity.ErrCanceled) {
		return "", nil
	}
	return path, err
}
