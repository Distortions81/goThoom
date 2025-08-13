package eui

import "testing"

func TestSaveLoadWindowZones(t *testing.T) {
	screenWidth = 100
	screenHeight = 100
	uiScale = 1

	a := &windowData{Title: "a"}
	b := &windowData{Title: "b"}
	windows = []*windowData{a, b}

	a.SetZone(HZoneLeft, VZoneTop)

	saved := SaveWindowZones()
	if len(saved) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(saved))
	}
	if st := saved["a"]; !st.Zoned || st.Zone.H != HZoneLeft || st.Zone.V != VZoneTop {
		t.Fatalf("unexpected saved state for a: %+v", st)
	}
	if st := saved["b"]; st.Zoned {
		t.Fatalf("expected b to be unzoned")
	}

	a.ClearZone()
	b.SetZone(HZoneRight, VZoneBottom)
	LoadWindowZones(saved)

	if a.zone == nil || a.zone.h != HZoneLeft || a.zone.v != VZoneTop {
		t.Fatalf("a zone not restored: %+v", a.zone)
	}
	if b.zone != nil {
		t.Fatalf("b zone should be cleared: %+v", b.zone)
	}
	windows = nil
}
