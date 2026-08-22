package main

import "testing"

func TestSetHighQualityResamplingEnabled(t *testing.T) {
	orig := highQualityResampling
	defer setHighQualityResamplingEnabled(orig)

	setHighQualityResamplingEnabled(false)
	if highQualityResampling {
		t.Fatalf("expected highQualityResampling to be false")
	}

	setHighQualityResamplingEnabled(true)
	if !highQualityResampling {
		t.Fatalf("expected highQualityResampling to be true")
	}
}
