package main

import (
	"bytes"
	"context"
	"image"
	stdraw "image/draw"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"
	"time"

	"gothoom/eui"

	imagedraw "golang.org/x/image/draw"
)

func TestStreamFrameRatePanelUpdatesSettings(t *testing.T) {
	if err := eui.Init(); err != nil {
		t.Fatal(err)
	}
	originalSettings, originalDirty := gs, settingsDirty
	originalWindow := streamWin
	originalControls := []*eui.ItemData{streamSource, streamResolution, streamFrameRate, streamHint, streamIdle, streamStatus, streamLight, streamStartButton, streamStopButton}
	gs = gsdef
	streamWin = nil
	t.Cleanup(func() {
		if streamWin != nil {
			streamWin.RemoveWindow()
		}
		gs, settingsDirty, streamWin = originalSettings, originalDirty, originalWindow
		streamSource, streamResolution, streamFrameRate = originalControls[0], originalControls[1], originalControls[2]
		streamHint, streamIdle, streamStatus, streamLight = originalControls[3], originalControls[4], originalControls[5], originalControls[6]
		streamStartButton, streamStopButton = originalControls[7], originalControls[8]
	})
	makeStreamWindow()
	if got := streamResolution.Options; len(got) != 4 || got[1] != "1920 × 1080 (1080p)" || streamResolution.Selected != 1 {
		t.Fatalf("resolution options = %v, selected %d", got, streamResolution.Selected)
	}
	for _, selection := range []struct{ index, fps int }{{0, 15}, {2, 60}, {1, 30}, {2, 60}} {
		// EUI sets Selected before emitting the dropdown event, as a click does.
		streamFrameRate.Selected = selection.index
		settingsDirty = false
		streamFrameRate.Handler.Emit(eui.UIEvent{Item: streamFrameRate, Type: eui.EventDropdownSelected, Index: selection.index})
		if gs.StreamFPS != selection.fps || !settingsDirty {
			t.Fatalf("selection %d: StreamFPS=%d dirty=%t", selection.index, gs.StreamFPS, settingsDirty)
		}
		refreshStreamWindow()
		if streamFrameRate.Selected != selection.index {
			t.Fatalf("refresh replaced FPS selection %d with %d", selection.index, streamFrameRate.Selected)
		}
		t.Logf("panel selection %d -> StreamFPS=%d", selection.index, gs.StreamFPS)
	}
}

func TestStreamResolutionValidation(t *testing.T) {
	for resolution, want := range map[int]bool{0: false, 720: true, 1080: true, 1440: true, 2160: true, 1234: false} {
		if got := validStreamResolution(resolution); got != want {
			t.Errorf("validStreamResolution(%d) = %t, want %t", resolution, got, want)
		}
	}
}

func TestStreamCaptureCadence(t *testing.T) {
	for _, fps := range []int{15, 30, 60} {
		t.Run(strconv.Itoa(fps), func(t *testing.T) {
			var next, lastOld time.Time
			captured, oldCaptured := 0, 0
			start := time.Unix(1, 0)
			jitter := [...]time.Duration{0, 200 * time.Microsecond, -200 * time.Microsecond, 100 * time.Microsecond}
			for draw := range 600 {
				now := start.Add(time.Duration(draw)*time.Second/60 + jitter[draw%len(jitter)])
				if streamCaptureDue(now, next, fps) {
					captured++
					next = advanceStreamCapture(now, next, fps)
				}
				if lastOld.IsZero() || now.Sub(lastOld) >= time.Second/time.Duration(fps) {
					oldCaptured++
					lastOld = now
				}
			}
			if captured != fps*10 {
				t.Fatalf("captured %d frames in 10 seconds, want %d", captured, fps*10)
			}
			t.Logf("jittered 60 Hz draws: old limiter %.1f FPS, fixed clock %.1f FPS", float64(oldCaptured)/10, float64(captured)/10)
			late := next.Add(5 * time.Second)
			next = advanceStreamCapture(late, next, fps)
			if streamCaptureDue(late, next, fps) {
				t.Fatal("late capture left an immediately due catch-up frame")
			}
		})

	}
}

func TestStreamOutputSourceCanChangeWhileRunning(t *testing.T) {
	streamOutput.mu.Lock()
	originalServer, originalConfig := streamOutput.server, streamOutput.config
	streamOutput.server = &http.Server{}
	streamOutput.config = streamConfig{wholeClient: false}
	streamOutput.mu.Unlock()
	t.Cleanup(func() {
		streamOutput.mu.Lock()
		streamOutput.server, streamOutput.config = originalServer, originalConfig
		streamOutput.mu.Unlock()
	})

	if !setStreamOutputSource(true) {
		t.Fatal("source change reported no active stream")
	}
	streamOutput.mu.Lock()
	wholeClient := streamOutput.config.wholeClient
	streamOutput.mu.Unlock()
	if !wholeClient {
		t.Fatal("source did not change to whole client")
	}
}

func TestStreamCaptureLayoutKeepsFixedCanvas(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
		config        streamConfig
		wantSize      image.Point
		wantTarget    image.Rectangle
	}{
		{"game 4:3", 640, 480, streamConfig{resolution: 720}, image.Pt(1280, 720), image.Rect(160, 0, 1120, 720)},
		{"game wide", 2000, 500, streamConfig{resolution: 720}, image.Pt(1280, 720), image.Rect(0, 200, 1280, 520)},
		{"game 1080p", 640, 480, streamConfig{resolution: 1080}, image.Pt(1920, 1080), image.Rect(240, 0, 1680, 1080)},
		{"game 1440p", 640, 480, streamConfig{resolution: 1440}, image.Pt(2560, 1440), image.Rect(320, 0, 2240, 1440)},
		{"game 4K", 640, 480, streamConfig{resolution: 2160}, image.Pt(3840, 2160), image.Rect(480, 0, 3360, 2160)},
		{"unaligned width", 1094, 1080, streamConfig{resolution: 1080}, image.Pt(1920, 1080), image.Rect(413, 0, 1507, 1080)},
		{"odd height", 1920, 1001, streamConfig{resolution: 1080}, image.Pt(1920, 1080), image.Rect(0, 39, 1920, 1040)},
		{"native", 701, 503, streamConfig{}, image.Pt(701, 503), image.Rect(0, 0, 701, 503)},
		{"whole client", 640, 480, streamConfig{resolution: 720, wholeClient: true}, image.Pt(1280, 720), image.Rect(160, 0, 1120, 720)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			size, target := streamCaptureLayout(tc.width, tc.height, tc.config)
			if size != tc.wantSize || target != tc.wantTarget {
				t.Fatalf("size=%v target=%v, want %v %v", size, target, tc.wantSize, tc.wantTarget)
			}
		})
	}
}

func TestStreamFitRectPreservesAspectRatio(t *testing.T) {
	for _, tc := range []struct {
		name             string
		sourceW, sourceH int
		targetW, targetH int
		want             image.Rectangle
	}{
		{"four-three", 800, 600, 1280, 720, image.Rect(160, 0, 1120, 720)},
		{"wide", 1920, 1080, 1280, 720, image.Rect(0, 0, 1280, 720)},
		{"tall", 600, 800, 1280, 720, image.Rect(370, 0, 910, 720)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := streamFitRect(tc.sourceW, tc.sourceH, tc.targetW, tc.targetH); got != tc.want {
				t.Fatalf("streamFitRect() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStreamEncoderReusePreservesPixels(t *testing.T) {
	var encoder streamFrameEncoder
	// Alternate aspect ratios to expose stale letterbox pixels, then
	// exercise the exact-size fast path and a changed output resolution.
	for _, tc := range []struct{ width, height, resolution int }{
		{40, 20, 72}, {20, 40, 72}, {40, 20, 72}, {128, 72, 72}, {40, 20, 108},
	} {
		frame := streamFrame{width: tc.width, height: tc.height, pixels: make([]byte, 4*tc.width*tc.height)}
		for i := range frame.pixels {
			frame.pixels[i] = 255
		}
		got, err := encoder.encode(frame, tc.resolution)
		if err != nil {
			t.Fatal(err)
		}
		source := &image.RGBA{Pix: frame.pixels, Stride: tc.width * 4, Rect: image.Rect(0, 0, tc.width, tc.height)}
		destination := image.NewRGBA(image.Rect(0, 0, tc.resolution*16/9, tc.resolution))
		stdraw.Draw(destination, destination.Bounds(), image.Black, image.Point{}, stdraw.Src)
		imagedraw.ApproxBiLinear.Scale(destination, streamFitRect(tc.width, tc.height, destination.Bounds().Dx(), tc.resolution), source, source.Bounds(), stdraw.Src, nil)
		var want bytes.Buffer
		if err := jpeg.Encode(&want, destination, &jpeg.Options{Quality: 95}); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want.Bytes()) {
			t.Fatalf("reused image differs from fresh resize for %+v", tc)
		}
	}
}

func TestStreamParallelEncodingSkipsLateAndFailedFrames(t *testing.T) {
	pool := newMJPEGPool()
	subscriber := pool.subscribe()
	defer pool.unsubscribe(subscriber)
	frames := make(chan streamFrame, 4)
	done, finished := make(chan struct{}), make(chan struct{})
	blocked, release := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		encodeStreamFramesWithWorkers(frames, done, pool, 2, func() func(streamFrame) ([]byte, error) {
			return func(frame streamFrame) ([]byte, error) {
				switch frame.width {
				case 1:
					close(blocked)
					<-release
				case 3:
					return nil, image.ErrFormat
				}
				return []byte{byte(frame.width)}, nil
			}
		})
	}()
	t.Cleanup(func() {
		close(release)
		close(done)
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("encoder workers did not stop")
		}
	})
	frames <- streamFrame{width: 1}
	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("first encoder did not start")
	}
	frames <- streamFrame{width: 2}
	waitStreamFrame(t, pool, 2)
	frames <- streamFrame{width: 3}
	frames <- streamFrame{width: 4}
	waitStreamFrame(t, pool, 4)
	// Release the first encode, then send another frame. Every published
	// result must remain monotonic even though the first finishes last.
	release <- struct{}{}
	frames <- streamFrame{width: 5}
	waitStreamFrame(t, pool, 5)
}

func waitStreamFrame(t *testing.T, pool *mjpegPool, want byte) {
	t.Helper()
	updated := pool.subscribe()
	defer pool.unsubscribe(updated)
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		pool.mu.Lock()
		frame := bytes.Clone(pool.frame)
		pool.mu.Unlock()
		if len(frame) > 0 {
			if frame[0] == want {
				return
			}
			if frame[0] == 1 {
				t.Fatal("late frame replaced a newer frame")
			}
		}
		select {
		case <-updated:
		case <-timeout.C:
			t.Fatalf("timed out waiting for frame %d", want)
		}
	}
}

func TestStreamHTTPMultipartAndStop(t *testing.T) {
	pool := newMJPEGPool()
	pool.update([]byte("first JPEG"))
	server := httptest.NewServer(http.HandlerFunc(pool.serveHTTP))
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if got := response.Header.Get("Content-Type"); got != "multipart/x-mixed-replace; boundary="+streamBoundary {
		t.Fatalf("Content-Type = %q", got)
	}
	reader := multipart.NewReader(response.Body, streamBoundary)
	for _, want := range []string{"first JPEG", "second JPEG"} {
		if want != "first JPEG" {
			pool.update([]byte(want))
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Fatal(err)
		}
		length, err := strconv.Atoi(part.Header.Get("Content-Length"))
		if err != nil {
			t.Fatal(err)
		}
		got := make([]byte, length)
		if _, err := io.ReadFull(part, got); err != nil {
			t.Fatal(err)
		}
		if string(got) != want || part.Header.Get("Content-Type") != "image/jpeg" {
			t.Fatalf("part = %q, headers = %v", got, part.Header)
		}
	}
	streamOutput.mu.Lock()
	streamOutput.server = server.Config
	streamOutput.mu.Unlock()
	stopStreamOutput()
	if _, err := io.ReadAll(response.Body); err == nil {
		t.Fatal("stop did not close the open streaming response")
	}
}

func TestStreamHTTPResponsesDisableCaching(t *testing.T) {
	pool := newMJPEGPool()
	pool.update([]byte("frame"))
	server := httptest.NewServer(streamHTTPHandler(pool, streamConfig{fps: 60}, make(chan streamFrame, 1)))
	defer server.Close()

	for _, path := range []string{"/", "/stats", "/stream", "/missing"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if got := response.Header.Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, max-age=0" {
			t.Errorf("%s Cache-Control = %q", path, got)
		}
		if got := response.Header.Get("Pragma"); got != "no-cache" {
			t.Errorf("%s Pragma = %q", path, got)
		}
		if got := response.Header.Get("Expires"); got != "0" {
			t.Errorf("%s Expires = %q", path, got)
		}
	}
}

type streamSlowWriter struct {
	header http.Header
	bytes.Buffer
	flushed chan []byte
	resume  chan struct{}
}

func (w *streamSlowWriter) Header() http.Header { return w.header }
func (w *streamSlowWriter) WriteHeader(int)     {}
func (w *streamSlowWriter) Flush() {
	w.flushed <- bytes.Clone(w.Bytes())
	w.Reset()
	<-w.resume
}

func TestStreamSlowClientSkipsOldFrames(t *testing.T) {
	pool := newMJPEGPool()
	pool.update([]byte("first"))
	ctx, cancel := context.WithCancel(context.Background())
	w := &streamSlowWriter{header: make(http.Header), flushed: make(chan []byte, 4), resume: make(chan struct{})}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		pool.serveHTTP(w, httptest.NewRequest("GET", "/stream", nil).WithContext(ctx))
	}()
	t.Cleanup(func() {
		cancel()
		close(w.resume)
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("client did not disconnect")
		}
	})
	for _, want := range []string{"first", "newest"} {
		select {
		case part := <-w.flushed:
			if !bytes.HasSuffix(part, []byte(want+"\r\n")) {
				t.Fatalf("part = %q, want %q", part, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("client did not receive a frame")
		}
		if want == "first" {
			pool.update([]byte("stale"))
			pool.update([]byte("newest"))
			w.resume <- struct{}{}
		}
	}
}

func TestStreamDefaultsUse1080p60GameRender(t *testing.T) {
	if gsdef.StreamSource != 0 || gsdef.StreamResolution != 1080 || gsdef.StreamFPS != 60 {
		t.Fatalf("stream defaults = source:%d resolution:%d fps:%d", gsdef.StreamSource, gsdef.StreamResolution, gsdef.StreamFPS)
	}
	if gsdef.StreamIdleBlack {
		t.Fatal("the splash screen must be the default before login")
	}
}

func TestStreamEncoderWorkersBoundsConcurrency(t *testing.T) {
	want := min(4, runtime.GOMAXPROCS(0))
	if want < 1 {
		want = 1
	}
	if got := streamEncoderWorkers(); got != want {
		t.Fatalf("streamEncoderWorkers() = %d, want %d", got, want)
	}
}

func TestEncodeStreamFrameProducesRequested720pJPEG(t *testing.T) {
	frame := streamFrame{width: 4, height: 2, pixels: make([]byte, 4*4*2)}
	for i := 0; i < len(frame.pixels); i += 4 {
		frame.pixels[i], frame.pixels[i+1], frame.pixels[i+2], frame.pixels[i+3] = 0x40, 0x80, 0xc0, 0xff
	}
	encoded, err := encodeStreamFrame(frame, 720)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.DecodeConfig(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Width != 1280 || decoded.Height != 720 {
		t.Fatalf("encoded dimensions = %dx%d, want 1280x720", decoded.Width, decoded.Height)
	}
}

func TestEncodeStreamFrameAtNativeResolutionDoesNotRescale(t *testing.T) {
	frame := streamFrame{width: 4, height: 2, pixels: make([]byte, 4*4*2)}
	for i := 3; i < len(frame.pixels); i += 4 {
		frame.pixels[i] = 0xff
	}
	encoded, err := encodeStreamFrame(frame, 0)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.DecodeConfig(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Width != frame.width || decoded.Height != frame.height {
		t.Fatalf("native dimensions = %dx%d, want %dx%d", decoded.Width, decoded.Height, frame.width, frame.height)
	}
}
