//go:build js

package main

import "errors"

var errStorageDirectoryDialogCancelled = errors.New("storage directory dialog cancelled")

func pickStorageDirectory(string, string) (string, error) {
	return "", errors.New("alternate file paths are not available in the browser build")
}
