package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeDialogSelectionAndCancellation(t *testing.T) {
	bin := t.TempDir()
	argsFile := filepath.Join(bin, "args")
	// Exercise the real picker adapter without opening a window. Install both
	// supported tools so this works regardless of the user's desktop preference.
	for _, name := range []string{"zenity", "qarma", "matedialog"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(`#!/bin/sh
printf '%s\n' "$@" > "$GOTHOOM_TEST_DIALOG_ARGS"
printf '%s\n' "$GOTHOOM_TEST_DIALOG_RESULT"
exit "$GOTHOOM_TEST_DIALOG_EXIT"
`), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("GOTHOOM_TEST_DIALOG_ARGS", argsFile)
	t.Setenv("GOTHOOM_TEST_DIALOG_EXIT", "0")
	selected := filepath.Join(bin, "a movie.clMov")
	t.Setenv("GOTHOOM_TEST_DIALOG_RESULT", selected)
	if got, err := pickMovieFile(); err != nil || got != selected {
		t.Fatalf("movie selection = %q, %v; want %q", got, err, selected)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range []string{"*.clMov", "*.clmov", "*.zip", "*.ZIP"} {
		if !strings.Contains(string(args), pattern) {
			t.Errorf("movie filter is missing %q: %s", pattern, args)
		}
	}
	start := filepath.Join(bin, "storage folder")
	if err := os.Mkdir(start, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTHOOM_TEST_DIALOG_RESULT", start)
	if got, err := pickStorageDirectory("Choose storage folder", filepath.Join(start, "missing")); err != nil || got != start {
		t.Fatalf("directory selection = %q, %v; want %q", got, err, start)
	}
	args, err = os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--directory\n", "Choose storage folder\n", "--filename\n" + start + "/\n"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("directory arguments are missing %q: %s", want, args)
		}
	}
	for _, picker := range []struct {
		name   string
		pick   func() (string, error)
		cancel error
	}{
		{"movie", pickMovieFile, errMovieDialogCancelled},
		{"directory", func() (string, error) { return pickStorageDirectory("Storage", start) }, errStorageDirectoryDialogCancelled},
	} {
		t.Run(picker.name, func(t *testing.T) {
			t.Setenv("GOTHOOM_TEST_DIALOG_EXIT", "1")
			if got, err := picker.pick(); got != "" || !errors.Is(err, picker.cancel) {
				t.Fatalf("cancel = %q, %v; want empty selection and %v", got, err, picker.cancel)
			}
			t.Setenv("GOTHOOM_TEST_DIALOG_EXIT", "2")
			if got, err := picker.pick(); got != "" || err == nil || errors.Is(err, picker.cancel) {
				t.Fatalf("picker failure = %q, %v; want a non-cancellation error", got, err)
			}
		})
	}
}
