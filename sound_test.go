package main

import (
	"math"
	"testing"
)

// TestSincNormalization ensures sinc table coefficients sum to approximately 1.
func TestSincNormalization(t *testing.T) {
	initSinc()
	for phase, coeffs := range sincTable {
		var sum float32
		for _, c := range coeffs {
			sum += c
		}
		norm := sum * sincInvSums[phase]
		if math.Abs(float64(norm-1)) > 1e-6 {
			t.Fatalf("phase %d normalized sum %f out of range", phase, norm)
		}
		if sincInvSums[phase] <= 0 {
			t.Fatalf("phase %d inverse sum %f invalid", phase, sincInvSums[phase])
		}
	}
}
