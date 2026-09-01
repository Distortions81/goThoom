package main

import (
	"image"
	"image/color"
	"runtime"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"gothoom/climg"
)

func TestArtworkWorkerPoolRunsIndependentJobsConcurrently(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("concurrent artwork workers require GOMAXPROCS >= 2")
	}
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		runArtworkJobs([]func(){
			func() { started <- struct{}{}; <-release },
			func() { started <- struct{}{}; <-release },
		})
		close(done)
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("independent artwork jobs did not run concurrently")
		}
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("artwork worker batch did not finish")
	}
}

func TestTransparentMobilePosesAreDetectedBeforeProcessing(t *testing.T) {
	sheet := image.NewRGBA(image.Rect(0, 0, 18, 18))
	regions := artworkRegionRects(makeSheetKey(1, nil, true), sheet)
	if len(regions) != 256 {
		t.Fatalf("mobile region count = %d, want 256", len(regions))
	}
	blank, visible := copyArtworkRegion(sheet, regions[0])
	if blank != nil || visible {
		t.Fatalf("blank pose = (%v, %t), want no processing buffer", blank, visible)
	}
	sheet.Pix[sheet.PixOffset(regions[1].Min.X, regions[1].Min.Y)+3] = 255
	_, visible = copyArtworkRegion(sheet, regions[1])
	if !visible {
		t.Fatal("opaque pose was classified as empty")
	}
}

func TestArtworkRGBAPoolReusesExactSize(t *testing.T) {
	artworkRGBAPoolMu.Lock()
	originalPool := artworkRGBAPool
	originalPixels := artworkRGBAPixels
	artworkRGBAPool = make(map[image.Point][]*image.RGBA)
	artworkRGBAPixels = 0
	artworkRGBAPoolMu.Unlock()
	t.Cleanup(func() {
		artworkRGBAPoolMu.Lock()
		artworkRGBAPool = originalPool
		artworkRGBAPixels = originalPixels
		artworkRGBAPoolMu.Unlock()
	})

	first := acquireArtworkRGBA(image.Rect(0, 0, 17, 23))
	firstPixel := &first.Pix[0]
	releaseArtworkRGBA(first)
	second := acquireArtworkRGBA(image.Rect(0, 0, 17, 23))
	if &second.Pix[0] != firstPixel {
		t.Fatal("artwork RGBA pool did not reuse an exact-size buffer")
	}
	releaseArtworkRGBA(second)
}

func TestFrameBlendShaderCompiles(t *testing.T) {
	shader, err := ebiten.NewShader(frameBlendShaderSource)
	if err != nil {
		t.Fatalf("compile frame blend shader: %v", err)
	}
	shader.Deallocate()
}

func TestMobileRecolorShadersCompile(t *testing.T) {
	for name, source := range map[string][]byte{
		"single": mobileRecolorShaderSource,
		"blend":  mobileRecolorBlendShaderSource,
	} {
		shader, err := ebiten.NewShader(source)
		if err != nil {
			t.Fatalf("compile mobile recolor %s shader: %v", name, err)
		}
		shader.Deallocate()
	}
}

func TestUpscaleInfluenceTracksCornerContributors(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			source.SetRGBA(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	source.SetRGBA(1, 0, color.RGBA{A: 255})
	source.SetRGBA(0, 1, color.RGBA{A: 255})
	source.SetRGBA(2, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	source.SetRGBA(1, 2, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	slots := image.NewGray(source.Bounds())
	slots.SetGray(1, 0, color.Gray{Y: 1})
	slots.SetGray(0, 1, color.Gray{Y: 2})
	slots.SetGray(1, 1, color.Gray{Y: 3})

	_, influence := upscaleSpriteRegionCPUWithInfluence(source, source.Bounds(), slots, slots.Bounds(), 4, artworkUpscaleBalanced, image.NewRGBA)
	if influence == nil {
		t.Fatal("upscale did not produce a custom-color influence image")
	}
	offset := influence.PixOffset(4, 4)
	packed := uint32(influence.Pix[offset]) | uint32(influence.Pix[offset+1])<<8 | uint32(influence.Pix[offset+2])<<16
	center := packed & 31
	first := (packed >> 5) & 31
	second := (packed >> 10) & 31
	weight := packed >> 15
	if center != 3 || first != 2 || second != 1 || weight != 204 {
		t.Fatalf("corner influence = slots %d,%d,%d weight %d, want 3,2,1 weight 204", center, first, second, weight)
	}
}

func BenchmarkArtworkPoseBatch(b *testing.B) {
	const (
		poseCount = 16
		poseSize  = 64
		factor    = 4
	)
	source := image.NewRGBA(image.Rect(0, 0, poseSize, poseSize))
	for y := 0; y < poseSize; y++ {
		for x := 0; x < poseSize; x++ {
			offset := source.PixOffset(x, y)
			value := byte(72)
			if (x+y)&1 != 0 {
				value = 104
			}
			source.Pix[offset] = value
			source.Pix[offset+1] = value + 12
			source.Pix[offset+2] = value + 20
			source.Pix[offset+3] = 255
		}
	}
	process := func() *image.RGBA {
		pose, _ := copyArtworkRegion(source, source.Bounds())
		climg.DenoiseRGBASerial(pose, 10, 0.35)
		return upscaleSpriteRegionCPU(pose, pose.Bounds(), factor, artworkUpscaleBalanced)
	}
	processPooled := func() *image.RGBA {
		pose, _ := copyArtworkRegion(source, source.Bounds())
		climg.DenoiseRGBASerial(pose, 10, 0.35)
		scaled := upscaleSpriteRegionCPUWithAllocator(pose, pose.Bounds(), factor, artworkUpscaleBalanced, acquireArtworkRGBA)
		releaseArtworkRGBA(pose)
		return scaled
	}
	b.SetBytes(poseCount * poseSize * poseSize * 4)
	b.Run("Serial", func(b *testing.B) {
		for range b.N {
			results := make([]*image.RGBA, poseCount)
			for index := range results {
				results[index] = process()
			}
		}
	})
	b.Run("SharedWorkers", func(b *testing.B) {
		for range b.N {
			results := make([]*image.RGBA, poseCount)
			jobs := make([]func(), poseCount)
			for index := range jobs {
				index := index
				jobs[index] = func() { results[index] = process() }
			}
			runArtworkJobs(jobs)
		}
	})
	b.Run("SharedWorkersPooled", func(b *testing.B) {
		for range b.N {
			results := make([]*image.RGBA, poseCount)
			jobs := make([]func(), poseCount)
			for index := range jobs {
				index := index
				jobs[index] = func() { results[index] = processPooled() }
			}
			runArtworkJobs(jobs)
			for _, result := range results {
				releaseArtworkRGBA(result)
			}
		}
	})
}
