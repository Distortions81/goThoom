package main

import (
	"testing"
	"time"
)

func TestBubbleTortureSnapshotCoversEveryBubbleTypeAndTextSize(t *testing.T) {
	originalStart := bubbleTortureStarted
	originalPlayerIndex := playerIndex
	t.Cleanup(func() {
		bubbleTortureStarted = originalStart
		playerIndex = originalPlayerIndex
	})

	start := time.Unix(1000, 0)
	bubbleTortureStarted = start
	types := make(map[int]bool)
	minText, maxText := int(^uint(0)>>1), 0
	var snap drawSnapshot
	for phase := 0; phase < 3; phase++ {
		prepareBubbleTortureSnapshot(&snap, start.Add(time.Duration(phase)*bubbleTortureTextCycle+time.Second))
		if got := len(snap.bubbles); got != bubbleTortureVisibleCount+1 {
			t.Fatalf("phase %d bubble count = %d, want %d plus edge bubble", phase, got, bubbleTortureVisibleCount)
		}
		for _, b := range snap.bubbles {
			types[b.Type&kBubbleTypeMask] = true
			minText = min(minText, len(b.Text))
			maxText = max(maxText, len(b.Text))
			if !b.Far {
				if _, ok := snap.descriptors[b.Index]; !ok {
					t.Fatalf("bubble %d has no speaker descriptor", b.DedupeID)
				}
			}
		}
	}
	for typ := kBubbleNormal; typ <= kBubbleNarrate; typ++ {
		if !types[typ] {
			t.Errorf("bubble type %d is absent from torture snapshot", typ)
		}
	}
	if minText > 8 || maxText < 300 {
		t.Fatalf("text length range = %d..%d, want short and oversized samples", minText, maxText)
	}
	if len(snap.liveMobs) != len(bubbleTortureNames) {
		t.Fatalf("mobile count = %d, want %d", len(snap.liveMobs), len(bubbleTortureNames))
	}
}

func TestBubbleTortureSnapshotMovesPeopleAndCamera(t *testing.T) {
	originalStart := bubbleTortureStarted
	originalPlayerIndex := playerIndex
	t.Cleanup(func() {
		bubbleTortureStarted = originalStart
		playerIndex = originalPlayerIndex
	})

	start := time.Unix(2000, 0)
	bubbleTortureStarted = start
	var snap drawSnapshot
	prepareBubbleTortureSnapshot(&snap, start.Add(bubbleTortureFrameInterval/2))
	if snap.picShiftX == 0 && snap.picShiftY == 0 {
		t.Fatal("camera did not move during the first torture-test update")
	}
	movedIndependently := false
	for _, current := range snap.liveMobs {
		previous := snap.prevMobiles[current.Index]
		if int(current.H)-int(previous.H) != snap.picShiftX || int(current.V)-int(previous.V) != snap.picShiftY {
			movedIndependently = true
			break
		}
	}
	if !movedIndependently {
		t.Fatal("no torture-test mobile moved independently of the camera")
	}
}
