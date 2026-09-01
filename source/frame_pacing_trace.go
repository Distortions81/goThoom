package main

import (
	"log"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// framePacingTraceThreshold logs complete Update-to-Update intervals that
// exceed the requested duration. Unlike a CPU profile, this includes time
// outside Game.Update and Game.Draw, such as presentation, driver stalls, and
// Ebitengine frame scheduling.
var framePacingTraceThreshold time.Duration

var (
	framePacingLastUpdateStart atomic.Int64
	framePacingUpdateWork      atomic.Int64
	framePacingDrawWork        atomic.Int64
	framePacingSnapshotWait    atomic.Int64
	framePacingSequence        atomic.Uint64
)

func traceFramePacingUpdateStarted(now time.Time) {
	if framePacingTraceThreshold <= 0 {
		return
	}
	previous := framePacingLastUpdateStart.Swap(now.UnixNano())
	if previous == 0 {
		return
	}
	interval := now.Sub(time.Unix(0, previous))
	if interval < framePacingTraceThreshold {
		return
	}
	updateWork := time.Duration(framePacingUpdateWork.Load())
	drawWork := time.Duration(framePacingDrawWork.Load())
	outside := framePacingOutsideDuration(interval, updateWork, drawWork)

	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	var lastGCPause, lastGCAge time.Duration
	if memory.NumGC > 0 {
		lastGCPause = time.Duration(memory.PauseNs[(memory.NumGC-1)%uint32(len(memory.PauseNs))])
		lastGCAge = now.Sub(time.Unix(0, int64(memory.LastGC)))
	}

	log.Printf("frame pacing: frame=%d interval=%s threshold=%s update=%s draw=%s outside=%s snapshot_wait=%s fps=%.1f focused=%t vsync=%t gc_total=%d last_gc_pause=%s last_gc_age=%s gc_during_interval=%t heap=%dMiB goroutines=%d",
		framePacingSequence.Add(1),
		interval.Round(time.Microsecond), framePacingTraceThreshold,
		updateWork.Round(time.Microsecond), drawWork.Round(time.Microsecond), outside.Round(time.Microsecond),
		time.Duration(framePacingSnapshotWait.Load()).Round(time.Microsecond),
		ebiten.ActualFPS(), windowIsFocused(), effectiveVSyncEnabled(), memory.NumGC, lastGCPause.Round(time.Microsecond), lastGCAge.Round(time.Millisecond), memory.NumGC > 0 && lastGCAge <= interval,
		memory.HeapAlloc>>20, runtime.NumGoroutine(),
	)
}

func traceFramePacingUpdateFinished(elapsed time.Duration) {
	if framePacingTraceThreshold > 0 {
		framePacingUpdateWork.Store(int64(elapsed))
	}
}

func traceFramePacingDrawStarted() {
	if framePacingTraceThreshold > 0 {
		framePacingSnapshotWait.Store(0)
	}
}

func traceFramePacingDrawFinished(elapsed time.Duration) {
	if framePacingTraceThreshold > 0 {
		framePacingDrawWork.Store(int64(elapsed))
	}
}

func traceFramePacingSnapshotLockWait(elapsed time.Duration) {
	if framePacingTraceThreshold > 0 {
		framePacingSnapshotWait.Add(int64(elapsed))
	}
}

func framePacingOutsideDuration(interval, updateWork, drawWork time.Duration) time.Duration {
	outside := interval - updateWork - drawWork
	if outside < 0 {
		return 0
	}
	return outside
}
