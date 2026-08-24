package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatScriptErrorUsesRealSourcePath(t *testing.T) {
	path := filepath.Join("scripts", "example.go")
	message := formatScriptError(path, errors.New("_.go:12:7: expected expression"))
	if !strings.Contains(message, path+":12:7") || strings.Contains(message, "_.go") {
		t.Fatalf("formatted error = %q", message)
	}
}

func TestScriptStatusLabelsAreDistinct(t *testing.T) {
	tests := []struct {
		name                         string
		disabled, invalid, reloadBad bool
		err                          string
		want                         string
	}{
		{name: "running", want: "Running"},
		{name: "disabled", disabled: true, want: "Disabled"},
		{name: "reload", reloadBad: true, err: "bad edit", want: "Reload Failed (old version still running)"},
		{name: "error", disabled: true, err: "panic", want: "Stopped After Error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := scriptStatusLabel(test.disabled, test.invalid, test.err, test.reloadBad); got != test.want {
				t.Fatalf("status = %q, want %q", got, test.want)
			}
		})
	}
}
