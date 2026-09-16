package main

import "testing"

func TestHDPictureContactShadowProfiles(t *testing.T) {
	for _, id := range []uint16{41, 985, 2245, 3785, 3786} {
		profile, ok := hdPictureContactShadowProfiles[id]
		if !ok || profile.width <= 0 || profile.width > 1 || profile.y <= 0 || profile.y > 1 || profile.radius <= 0 || profile.alpha <= 0 || profile.alpha >= 1 {
			t.Fatalf("HD picture shadow profile %d = %#v, %v", id, profile, ok)
		}
	}
	if _, ok := hdPictureContactShadowProfiles[194]; ok {
		t.Fatal("transparent legacy shadow picture 194 must not receive a replacement contact shadow")
	}
}
