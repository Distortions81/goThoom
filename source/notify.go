package main

import (
    "os"
    "runtime"

    "github.com/gen2brain/beeep"
)

// notifyDesktop shows a desktop notification, best-effort and non-fatal.
func notifyDesktop(title, body string) {
    // Desktop notifications are controlled by callers; only drop empty body.
    if body == "" {
        return
    }
    // Skip on headless Linux without DISPLAY; beeep would error.
    if runtime.GOOS == "linux" && (os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "") {
        return
    }
    _ = beeep.Notify(title, body, "")
}
