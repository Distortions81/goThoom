//go:build !js

package main

import (
	"errors"
	"github.com/ncruces/zenity"
)

func pickBardTuneFile() (string, error) {
	path, err := zenity.SelectFile(zenity.Title("Import tune"), zenity.FileFilter{Name: "goThoom songs and CL tune text", Patterns: []string{"*.gttune", "*.tune", "*.txt", "*.GTTUNE", "*.TUNE", "*.TXT"}})
	if errors.Is(err, zenity.ErrCanceled) {
		return "", nil
	}
	return path, err
}
