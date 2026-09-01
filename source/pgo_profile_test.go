package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPGOMoviePathFindsDevelopmentFixture(t *testing.T) {
	path := defaultPGOMoviePath()
	if filepath.Base(path) != "test.clMov.zip" {
		t.Fatalf("default PGO movie = %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default PGO movie is unavailable: %v", err)
	}
}
