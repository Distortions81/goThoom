package eui

import "testing"

func TestSliderReleaseEventRequiresChangedValue(t *testing.T) {
	releases := 0
	handler := newHandler()
	handler.Handle = func(ev UIEvent) {
		if ev.Type == EventSliderReleased {
			releases++
		}
	}
	item := &itemData{
		ItemType:       ITEM_SLIDER,
		Value:          1,
		dragStartValue: 1,
		dragStartInit:  true,
		Handler:        handler,
	}

	item.emitSliderReleased()
	if releases != 0 {
		t.Fatalf("unchanged slider emitted %d release events, want 0", releases)
	}

	item.dragStartInit = true
	item.Value = 1.5
	item.emitSliderReleased()
	if releases != 1 {
		t.Fatalf("changed slider emitted %d release events, want 1", releases)
	}
	if item.dragStartInit {
		t.Fatal("slider drag remained active after release")
	}
}
