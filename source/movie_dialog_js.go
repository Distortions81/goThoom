//go:build js

package main

import "errors"

var errMovieDialogCancelled = errors.New("movie dialog cancelled")

func pickMovieFile() (string, error) {
	return "", errors.New("movie playback is not available in the browser build")
}
