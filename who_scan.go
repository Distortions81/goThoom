package main

import (
	"time"
)

// Simple manager to coordinate multi-batch /be-who scans and throttle requests.
var (
	whoActive           bool
	whoLastRequest      time.Time
	whoCooldown               = 1 * time.Second
	whoLastCommandFrame int32 = -1
)

// considerNextWhoBatch decides if we should ask for another page.
// The classic client expects batches of up to 20; continue when we saw a full
// batch and it added at least one new name.
func considerNextWhoBatch(batchCount, newCount int) {
	if batchCount >= 20 && newCount > 0 {
		whoActive = true
		// Schedule another request soon; the actual command emission happens
		// in pickQueuedCommand() to coalesce with other command traffic.
		return
	}
	// Fewer than 20 or no new names: end the scan.
	whoActive = false
}

// maybeEnqueueWho sets a pending /be-who when throttled and no other command
// is pending. Returns true if it queued a command.
func maybeEnqueueWho() bool {
	if !whoActive {
		return false
	}
	// Only if there have been no commands in the last 30 frames.
	if whoLastCommandFrame >= 0 {
		if (ackFrame - whoLastCommandFrame) < 30 {
			return false
		}
	}
	if time.Since(whoLastRequest) < whoCooldown {
		return false
	}
	if pendingCommand != "" {
		return false
	}
	pendingCommand = "/be-who"
	whoLastRequest = time.Now()
	return true
}
