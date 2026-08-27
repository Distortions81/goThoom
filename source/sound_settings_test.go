package main

import "testing"

func TestSetHighQualityResamplingEnabled(t *testing.T) {
	orig := highQualityResamplingEnabled()
	defer setHighQualityResamplingEnabled(orig)

	setHighQualityResamplingEnabled(false)
	if highQualityResamplingEnabled() {
		t.Fatalf("expected highQualityResampling to be false")
	}

	setHighQualityResamplingEnabled(true)
	if !highQualityResamplingEnabled() {
		t.Fatalf("expected highQualityResampling to be true")
	}
}
