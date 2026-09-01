package main

import (
	"testing"
	"time"
)

func TestAssetLoadFrameTraceCollectsAtlasWork(t *testing.T) {
	originalThreshold := assetLoadTraceThreshold
	assetLoadTraceThreshold = time.Millisecond
	t.Cleanup(func() {
		assetLoadTraceThreshold = originalThreshold
		activeAssetLoadFrameTrace.Store(nil)
	})

	trace := beginAssetLoadFrameTrace(time.Now())
	if trace == nil {
		t.Fatal("asset load trace was not enabled")
	}
	noteFrameArtworkPrepare(5, 2, 3, []uint16{10, 10, 12}, 4*time.Millisecond, time.Millisecond, 2*time.Millisecond, time.Millisecond)
	noteFrameImageCreation(20, 10, 500*time.Microsecond)
	noteFrameManagedImageAdd(800)
	noteFrameManagedImageRemove(400)
	noteFrameUnmanagedImageCreation(30, 20, 250*time.Microsecond, "main.test")
	noteFrameAtlasFactorChange(2, 3)
	noteFrameAtlasCacheClear(7, 4096)

	if !trace.hasAssetWork() {
		t.Fatal("atlas work was not recorded")
	}
	if got := trace.preparedSheets.Load(); got != 2 {
		t.Fatalf("prepared sheets = %d, want 2", got)
	}
	if got := trace.imageCreateBytes.Load(); got != 800 {
		t.Fatalf("image bytes = %d, want 800", got)
	}
	if got := trace.managedAddBytes.Load(); got != 800 {
		t.Fatalf("managed add bytes = %d, want 800", got)
	}
	if got := trace.managedRemoveBytes.Load(); got != 400 {
		t.Fatalf("managed remove bytes = %d, want 400", got)
	}
	if got := trace.unmanagedBytes.Load(); got != 2400 {
		t.Fatalf("unmanaged bytes = %d, want 2400", got)
	}
	trace.unmanagedMu.Lock()
	if got := trace.unmanagedSites["main.test"]; got != 1 {
		trace.unmanagedMu.Unlock()
		t.Fatalf("unmanaged test-site creations = %d, want 1", got)
	}
	trace.unmanagedMu.Unlock()
	if got := trace.lastFactorChange.Load(); got != uint32(2)<<8|3 {
		t.Fatalf("factor transition = %#x, want 2->3", got)
	}
	trace.idsMu.Lock()
	defer trace.idsMu.Unlock()
	if len(trace.ids) != 2 || trace.ids[0] != 10 || trace.ids[1] != 12 {
		t.Fatalf("image IDs = %v, want [10 12]", trace.ids)
	}
}
