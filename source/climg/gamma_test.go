package climg

import "testing"

func TestGammaCorrectChannel(t *testing.T) {
	images := &CLImages{}
	const classicGray = uint8(0x88)

	if got := images.GammaCorrectChannel(classicGray); got != classicGray {
		t.Fatalf("disabled correction got %#02x, want %#02x", got, classicGray)
	}

	images.SetGammaCorrection(true, 1.8, 2.2)
	if got, want := images.GammaCorrectChannel(classicGray), uint8(0x98); got != want {
		t.Fatalf("enabled correction got %#02x, want %#02x", got, want)
	}

	images.SetGammaCorrection(false, 1.8, 2.2)
	if got := images.GammaCorrectChannel(classicGray); got != classicGray {
		t.Fatalf("disabled-after-enabled correction got %#02x, want %#02x", got, classicGray)
	}
}
