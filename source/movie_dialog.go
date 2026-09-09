//go:build !js

package main

import (
	"errors"

	"github.com/ncruces/zenity"
)

var errMovieDialogCancelled = errors.New("movie dialog cancelled")

func pickMovieFile() (string, error) {
	filename, err := zenity.SelectFile(zenity.FileFilter{
		Name: "clMov files", Patterns: []string{"*.clMov", "*.clmov", "*.zip", "*.ZIP"},
	})
	if err != nil {
		if errors.Is(err, zenity.ErrCanceled) {
			return "", errMovieDialogCancelled
		}
		return "", err
	}
	return filename, nil
}
