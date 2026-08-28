package main

import (
	"time"
)

// Simple manager to coordinate multi-batch /be-who scans and throttle requests.
var (
	whoActive           bool
	whoScanStarted      time.Time
	whoLastRequest      time.Time
	whoCooldown               = 1 * time.Second
	whoLastCommandFrame int32 = -1
)

// considerNextWhoBatch decides if we should ask for another page.
// The classic client expects batches of up to 20; continue when we saw a full
// batch. Duplicate detection in parseBackendWho ends the scan early.
func considerNextWhoBatch(batchCount int) {
	if batchCount >= 20 {
		whoActive = true
		// Schedule another request soon; the actual command emission happens
		// in pickQueuedCommand() to coalesce with other command traffic.
		return
	}
	// Fewer than 20: end the scan.
	whoActive = false
	finishBeWhoScan()
}

// maybeEnqueueWho sets a pending /be-who when throttled and no other command
// is pending. Returns true if it queued a command.
func maybeEnqueueWho() bool {
	if !whoActive {
		return false
	}
	// Only if there have been no commands in the last 30 frames.
	lastCommandFrame := lastCommandFrameSnapshot()
	if lastCommandFrame >= 0 {
		if (ackFrame - lastCommandFrame) < 30 {
			return false
		}
	}
	if time.Since(whoLastRequest) < whoCooldown {
		return false
	}
	if !enqueueCommandIfIdle("/be-who") {
		return false
	}
	whoLastRequest = time.Now()
	return true
}
