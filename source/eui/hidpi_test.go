package eui

import "testing"

func TestUserUIScaleIncludesDeviceScale(t *testing.T) {
	originalScale := uiScale
	originalUserScale := userUIScale
	originalDeviceScale := lastDeviceScale
	t.Cleanup(func() {
		userUIScale = originalUserScale
		lastDeviceScale = originalDeviceScale
		SetUIScale(originalScale)
	})

	lastDeviceScale = 2
	SetUserUIScale(1)
	if got := UIScale(); got != 2 {
		t.Fatalf("Retina effective UI scale = %v, want 2", got)
	}
	if got := UserUIScale(); got != 1 {
		t.Fatalf("Retina user UI scale = %v, want 1", got)
	}

	SetUserUIScale(1.25)
	if got := UIScale(); got != 2.5 {
		t.Fatalf("Retina effective UI scale after user adjustment = %v, want 2.5", got)
	}

	SetUserUIScale(4)
	if got := UIScale(); got != 8 {
		t.Fatalf("Retina effective UI scale at maximum user preference = %v, want 8", got)
	}
}
