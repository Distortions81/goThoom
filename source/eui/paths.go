//go:build !test

package eui

import (
	"os"
	"path/filepath"
)

func init() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	// Ignore error to avoid failing when permissions prevent chdir
	_ = os.Chdir(dir)
}
