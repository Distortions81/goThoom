//go:build nodenoise

package climg

import "image"

// denoiseImage is a stub used when the nodenoise build tag is set.
func denoiseImage(img *image.RGBA, sharpness, maxPercent float64) {}

// DenoiseRGBA is a no-op when image denoising is disabled at build time.
func DenoiseRGBA(img *image.RGBA, sharpness, maxPercent float64) {}

// DenoiseRGBASerial is a no-op when image denoising is disabled at build time.
func DenoiseRGBASerial(img *image.RGBA, sharpness, maxPercent float64) {}

// DenoiseRGBAWorkers is a no-op when image denoising is disabled at build time.
func DenoiseRGBAWorkers(img *image.RGBA, sharpness, maxPercent float64, workers int) {}
