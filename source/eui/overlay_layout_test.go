package eui

import "testing"

func TestOverlayChildrenShareOneOrigin(t *testing.T) {
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	SetUIScale(1)

	background, _ := NewButton()
	background.Size = Point{X: 100, Y: 24}
	background.Position = Point{}
	action, _ := NewButton()
	action.Size = Point{X: 20, Y: 24}
	action.Position = Point{X: 80}
	overlay := NewOverlay(background, action)
	overlay.Fixed = true
	overlay.Size = Point{X: 100, Y: 24}

	if overlay.FlowType != FLOW_OVERLAY {
		t.Fatal("NewOverlay did not create overlay flow")
	}
	if got := overlay.contentBounds(); got != (Point{X: 100, Y: 24}) {
		t.Fatalf("overlay bounds = %+v, want children sharing 100x24 rectangle", got)
	}
}
