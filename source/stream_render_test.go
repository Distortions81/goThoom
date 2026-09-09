package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone: Ebitengine pixel reads need its game loop.
func TestRenderStreamFirstAndNextFrames(t *testing.T) {
	if os.Getenv("GOTHOOM_RENDER_STREAM") == "" {
		t.Skip("set GOTHOOM_RENDER_STREAM=1; run alone")
	}
	g := &streamRenderTestGame{}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.err != nil {
		t.Fatal(g.err)
	}
}

type streamRenderTestGame struct {
	done bool
	err  error
}

func (g *streamRenderTestGame) Layout(_, _ int) (int, int) { return 701, 503 }
func (g *streamRenderTestGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}
func (g *streamRenderTestGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	oldImage, oldRect, oldCapture, oldFake := gameImage, worldViewRect, streamCapture, fake
	defer func() {
		streamOutput.mu.Lock()
		streamOutput.frames, streamOutput.pool = nil, nil
		streamOutput.mu.Unlock()
		readPendingStreamOutput()
		gameImage, worldViewRect, streamCapture, fake = oldImage, oldRect, oldCapture, oldFake
	}()
	// A nonzero crop origin and odd native dimensions expose sizing/offset
	// mistakes that a square splash or an already aligned source would hide.
	gameImage = ebiten.NewImage(721, 523)
	defer gameImage.Deallocate()
	gameImage.Fill(color.RGBA{R: 40, G: 120, B: 200, A: 255})
	screen.Fill(color.RGBA{R: 40, G: 120, B: 200, A: 255})
	worldViewRect = image.Rect(10, 10, 711, 513)
	for _, resolution := range []int{0, 720, 1080} {
		for _, whole := range []bool{false, true} {
			for _, black := range []bool{false, true} {
				config := streamConfig{resolution: resolution, wholeClient: whole, idleBlack: black, fps: 30}
				if err := g.checkFrames(screen, config); err != nil {
					g.err = fmt.Errorf("%+v: %w", config, err)
					return
				}
			}
		}
	}
}

func (g *streamRenderTestGame) checkFrames(screen *ebiten.Image, config streamConfig) error {
	config.outputSize, _ = streamCaptureLayout(701, 503, config)
	frames := make(chan streamFrame, 1)
	pool := newMJPEGPool()
	subscriber := pool.subscribe()
	defer pool.unsubscribe(subscriber)
	streamOutput.mu.Lock()
	streamOutput.frames, streamOutput.pool, streamOutput.config = frames, pool, config
	streamOutput.nextCapture = time.Time{}
	streamOutput.mu.Unlock()
	readPendingStreamOutput()
	wantSize := image.Pt(701, 503)
	if config.resolution != 0 {
		wantSize = image.Pt(config.resolution*16/9, config.resolution)
	}
	var encoder streamFrameEncoder
	var firstLive []byte
	standby, err := encodeStreamStandbyFrame(config)
	if err != nil {
		return err
	}
	standbyConfig, err := jpeg.DecodeConfig(bytes.NewReader(standby))
	if err != nil {
		return err
	}
	if image.Pt(standbyConfig.Width, standbyConfig.Height) != wantSize {
		return fmt.Errorf("standby size %dx%d, want %v", standbyConfig.Width, standbyConfig.Height, wantSize)
	}
	oldImage, oldRect := gameImage, worldViewRect
	defer func() { gameImage, worldViewRect = oldImage, oldRect }()
	resized := ebiten.NewImage(300, 100)
	defer resized.Deallocate()
	resized.Fill(color.RGBA{R: 40, G: 120, B: 200, A: 255})
	for i := range 4 {
		fake = i != 0 // Standby, then two identical live frames.
		if i == 3 {
			// Even a later source resize must not change decoder dimensions.
			screen, gameImage, worldViewRect = resized, resized, resized.Bounds()
		}
		streamOutput.mu.Lock()
		streamOutput.nextCapture = time.Time{}
		streamOutput.mu.Unlock()
		// The first frame must also work with an unchanged cached world.
		captureStreamOutput(screen, i != 0)
		readPendingStreamOutput()
		var frame streamFrame
		select {
		case frame = <-frames:
		default:
			return fmt.Errorf("frame %d was not captured", i)
		}
		encoded, err := encoder.encode(frame, 0)
		releaseStreamFrame(frame)
		if err != nil {
			return err
		}
		decoded, err := jpeg.Decode(bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		if decoded.Bounds().Size() != wantSize {
			return fmt.Errorf("frame %d size %v, want %v", i, decoded.Bounds().Size(), wantSize)
		}
		center := color.RGBAModel.Convert(decoded.At(wantSize.X/2, wantSize.Y/2)).(color.RGBA)
		wantCenter := color.RGBA{R: 40, G: 120, B: 200, A: 255}
		if i == 0 && config.idleBlack && !config.wholeClient {
			wantCenter = color.RGBA{A: 255}
		}
		if math.Abs(float64(center.R)-float64(wantCenter.R)) > 2 || math.Abs(float64(center.G)-float64(wantCenter.G)) > 2 || math.Abs(float64(center.B)-float64(wantCenter.B)) > 2 {
			return fmt.Errorf("frame %d center %v, want %v", i, center, wantCenter)
		}
		if i == 0 && (!config.idleBlack || config.wholeClient) || i == 1 && firstLive == nil {
			firstLive = bytes.Clone(encoded)
		} else if i > 0 && i < 3 && !bytes.Equal(firstLive, encoded) {
			return fmt.Errorf("identical source changed JPEG bytes on frame %d", i)
		}
		if i == 3 {
			corner := color.RGBAModel.Convert(decoded.At(0, 0)).(color.RGBA)
			if corner.R > 2 || corner.G > 2 || corner.B > 2 {
				return fmt.Errorf("resized frame letterbox pixel = %v, want black", corner)
			}
		}
	}
	return nil
}
