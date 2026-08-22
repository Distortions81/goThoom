package main

import "testing"

func TestHighestLightPlane(t *testing.T) {
	tests := []struct {
		name   string
		lights []lightSource
		want   float32
	}{
		{name: "none", want: 32767},
		{name: "one", lights: []lightSource{{Plane: 5}}, want: 5},
		{name: "mixed", lights: []lightSource{{Plane: 255}, {Plane: 0}, {Plane: 136}}, want: 255},
		{name: "negative", lights: []lightSource{{Plane: -10}, {Plane: -2}}, want: -2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := highestLightPlane(tt.lights); got != tt.want {
				t.Fatalf("highestLightPlane() = %v, want %v", got, tt.want)
			}
		})
	}
}
