//go:build js

package main

import "fmt"

func pickBardTuneFile() (string, error) {
	return "", fmt.Errorf("Tune files are available in the desktop client.")
}
