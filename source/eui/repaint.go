package eui

import "time"

const backgroundRepaintPixels = 1_000_000
const maxRepaintDelay = 50 * time.Millisecond

// RepaintStats measures cached-window work. Duration is CPU submission time,
// not GPU execution time. Call WindowRepaintStats on the UI thread.
type RepaintStats struct {
	Title                         string
	Count, Pixels, DeferredFrames uint64
	LastPixels                    int
	LastDuration                  time.Duration
	LastReason                    string
}

// RepaintObserver is optional and runs on the UI thread after a cache repaint.
var RepaintObserver func(RepaintStats)

func WindowRepaintStats() []RepaintStats {
	stats := make([]RepaintStats, 0, len(windows))
	for _, win := range windows {
		s := win.repaintStats
		s.Title = win.Title
		stats = append(stats, s)
	}
	return stats
}

// shouldDeferRepaint charges actual backing pixels rather than logical UI
// size. An old image is usable only at the same size, and at least one eligible
// pane progresses each frame. Deadlines prevent continuously dirty panes from
// starving later panes in drawing order.
func (win *windowData) shouldDeferRepaint(now time.Time, urgent bool, spent *int) bool {
	if win.NoCache || (!win.Dirty && win.Render != nil) {
		return false
	}
	if win.repaintRequested.IsZero() {
		win.repaintRequested = now
	}
	w, h := renderImageSize(win.GetSize())
	pixels := w * h
	canDefer := win.DeferRepaint && !urgent && !win.HasIndeterminate && win.Render != nil &&
		win.Render.Bounds().Dx() == w && win.Render.Bounds().Dy() == h &&
		now.Sub(win.repaintRequested) < maxRepaintDelay
	if canDefer && *spent > 0 && *spent+pixels > backgroundRepaintPixels {
		win.repaintStats.DeferredFrames++
		return true
	}
	*spent += pixels
	return false
}

func (win *windowData) recordRepaint(start time.Time, pixels int) {
	s := &win.repaintStats
	s.Title = win.Title
	s.Count++
	s.Pixels += uint64(pixels)
	s.LastPixels = pixels
	s.LastDuration = time.Since(start)
	s.LastReason = win.repaintReason
	if s.LastReason == "" {
		s.LastReason = "window"
	}
	win.repaintRequested = time.Time{}
	win.repaintReason = ""
	if RepaintObserver != nil {
		RepaintObserver(*s)
	}
}
