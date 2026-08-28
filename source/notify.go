package main

import (
	"os"
	"runtime"

	"github.com/gen2brain/beeep"
)

// notifyDesktop shows a desktop notification, best-effort and non-fatal.
func notifyDesktop(title, body string) {
	// Desktop notifications are controlled by callers; drop empty bodies and
	// events replayed while rebuilding movie state during a seek.
	if body == "" || seekingMov {
		return
	}
	// Skip on headless Linux without DISPLAY; beeep would error.
	if runtime.GOOS == "linux" && (os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "") {
		return
	}
	_ = beeep.Notify(title, body, "")
}
