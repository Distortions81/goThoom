//go:build nodenoise

package climg

import "image"

// denoiseImage is a stub used when the nodenoise build tag is set.
func denoiseImage(img *image.RGBA, sharpness, maxPercent float32) {}
