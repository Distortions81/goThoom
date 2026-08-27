package main

import (
	"image"
	"image/color"
	"testing"
)

func TestClientActivityIndicatorsUseFixedBottomRightSlots(t *testing.T) {
	x, y := clientActivityIndicatorPosition(image.Rect(10, 12, 74, 76), 0)
	if x != 63 || y != 65 {
		t.Fatalf("indicator position = (%.0f, %.0f), want (63, 65)", x, y)
	}
	x, y = clientActivityIndicatorPosition(image.Rect(10, 12, 74, 76), 2)
	if x != 35 || y != 65 {
		t.Fatalf("left indicator position = (%.0f, %.0f), want (35, 65)", x, y)
	}
	wants := map[clientActivity]color.RGBA{
		clientActivityData:  {R: 52, G: 211, B: 104, A: 255},
		clientActivityAudio: {R: 255, G: 176, B: 32, A: 255},
		clientActivityGPU:   {R: 239, G: 68, B: 68, A: 255},
	}
	for activity, want := range wants {
		if got := clientActivityColors[activity]; got != want {
			t.Errorf("indicator color for activity %d = %v, want %v", activity, got, want)
		}
	}
}

func TestClientActivityIndicatorsKeepSimultaneousEvents(t *testing.T) {
	setClientActivityIndicatorsEnabled(true)
	t.Cleanup(func() { setClientActivityIndicatorsEnabled(false) })

	noteClientActivity(clientActivityData)
	noteClientActivity(clientActivityAudio)
	if got, want := takeClientActivity(), clientActivityData|clientActivityAudio; got != want {
		t.Fatalf("pending activity = %d, want %d", got, want)
	}
	if got := takeClientActivity(); got != clientActivityNone {
		t.Fatalf("activity after take = %d, want none", got)
	}
}

func TestClientActivityIndicatorsIgnoreEventsWhenDisabled(t *testing.T) {
	setClientActivityIndicatorsEnabled(true)
	noteClientActivity(clientActivityData)
	setClientActivityIndicatorsEnabled(false)
	noteClientActivity(clientActivityGPU)
	if got := takeClientActivity(); got != clientActivityNone {
		t.Fatalf("disabled activity = %d, want none", got)
	}
}
