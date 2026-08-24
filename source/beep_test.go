package main

import "testing"

func TestCommonNotificationSoundsAreEmbedded(t *testing.T) {
	tests := []struct {
		name string
		pcm  []byte
	}{
		{name: "default", pcm: embeddedBeepPCM(46, 60)},
		{name: "mention", pcm: embeddedBeepPCM(0, 84)},
		{name: "fallen", pcm: embeddedHarpPCM([]int{72, 69, 65})},
		{name: "recovered", pcm: embeddedHarpPCM([]int{60, 64, 67})},
		{name: "online", pcm: embeddedHarpPCM([]int{84, 84})},
	}
	for _, tt := range tests {
		if len(tt.pcm) == 0 {
			t.Errorf("%s notification PCM is empty", tt.name)
		}
	}
	if pcm := embeddedBeepPCM(1, 1); pcm != nil {
		t.Error("unexpected embedded PCM for arbitrary beep")
	}
	if pcm := embeddedHarpPCM([]int{1, 2, 3}); pcm != nil {
		t.Error("unexpected embedded PCM for arbitrary harp sequence")
	}
}
