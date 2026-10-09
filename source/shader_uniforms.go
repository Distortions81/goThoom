package main

// Ebitengine accepts a one-element float slice for a scalar uniform and copies
// its value when a draw is submitted. Bind the slice once, then update its
// element to avoid boxing a new float32 into an interface on every draw.
func setShaderFloat32(uniforms map[string]any, name string, value float32) {
	uniforms[name].([]float32)[0] = value
}
