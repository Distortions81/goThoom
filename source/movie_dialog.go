//go:build !js

package main

import (
	"errors"

	"github.com/sqweek/dialog"
)

var errMovieDialogCancelled = errors.New("movie dialog cancelled")

func pickMovieFile() (string, error) {
	filename, err := dialog.File().Filter("clMov files", "clMov", "clmov", "zip", "ZIP").Load()
	if err != nil {
		if err == dialog.Cancelled {
			return "", errMovieDialogCancelled
		}
		return "", err
	}
	return filename, nil
}
