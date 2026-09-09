package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	stdraw "image/draw"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	imagedraw "golang.org/x/image/draw"
)

const (
	streamOutputURL    = "http://127.0.0.1:8080/"
	streamBoundary     = "gothoom-frame"
	streamWriteTimeout = 5 * time.Second
)

type streamConfig struct {
	wholeClient bool
	resolution  int
	fps         int
	idleBlack   bool
	outputSize  image.Point // Fixed for the session, shared by standby and live frames.
}

type streamFrame struct {
	pixels []byte
	buffer *streamPixelBuffer
	width  int
	height int
}

type encodedStreamFrame struct {
	jpeg     []byte
	sequence uint64
	reusable chan struct{}
}

// mjpegPool keeps the newest encoded frame for all connected OBS media
// sources. Slow clients skip old frames rather than delaying the game.
type mjpegPool struct {
	mu          sync.Mutex
	frame       []byte
	header      []byte
	version     uint64
	subscribers map[chan struct{}]struct{}
	metrics     streamMetrics
}

// Read-only counters expose actual pipeline behavior at /stats without logging
// every frame or adding work to the normal UI.
type streamMetrics struct {
	captured      atomic.Uint64
	queued        atomic.Uint64
	encoded       atomic.Uint64
	encodeErrors  atomic.Uint64
	lateResults   atomic.Uint64
	queueFull     atomic.Uint64
	clientSkipped atomic.Uint64
	writeErrors   atomic.Uint64
	readbackNS    atomic.Uint64
	encodeNS      atomic.Uint64
}

func newMJPEGPool() *mjpegPool { return &mjpegPool{subscribers: make(map[chan struct{}]struct{})} }

func (pool *mjpegPool) update(frame []byte) {
	pool.mu.Lock()
	pool.header = append(pool.header[:0], "--"+streamBoundary+"\r\nContent-Type: image/jpeg\r\nContent-Length: "...)
	pool.header = strconv.AppendInt(pool.header, int64(len(frame)), 10)
	pool.header = append(pool.header, "\r\n\r\n"...)
	pool.frame = append(pool.frame[:0], frame...)
	pool.version++
	for updated := range pool.subscribers {
		select {
		case updated <- struct{}{}:
		default:
		}
	}
	pool.mu.Unlock()
}

func (pool *mjpegPool) serveHTTP(w http.ResponseWriter, r *http.Request) {
	updated := pool.subscribe()
	defer pool.unsubscribe(updated)
	setNoCacheHeaders(w.Header())
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+streamBoundary)
	control := http.NewResponseController(w)
	var seen uint64
	var frame, header []byte
	for {
		pool.mu.Lock()
		version := pool.version
		if version != seen {
			// Each connection owns reusable write buffers. Copy only compressed
			// bytes, so a blocked writer cannot pin an encoder's working buffer.
			frame = append(frame[:0], pool.frame...)
			header = append(header[:0], pool.header...)
		}
		pool.mu.Unlock()
		if frame != nil && version != seen {
			if seen != 0 && version > seen+1 {
				pool.metrics.clientSkipped.Add(version - seen - 1)
			}
			// Bound blocked writes so a stalled OBS source cannot retain a frame
			// and connection indefinitely. Each client writes outside the pool lock.
			if err := control.SetWriteDeadline(time.Now().Add(streamWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
				return
			}
			if _, err := w.Write(header); err != nil {
				pool.metrics.writeErrors.Add(1)
				return
			}
			if _, err := w.Write(frame); err != nil {
				pool.metrics.writeErrors.Add(1)
				return
			}
			if _, err := w.Write(streamPartEnd); err != nil {
				pool.metrics.writeErrors.Add(1)
				return
			}
			if err := control.Flush(); err != nil {
				pool.metrics.writeErrors.Add(1)
				return
			}
			// No write is pending while waiting for a new game frame.
			if err := control.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
				return
			}
			seen = version
			continue
		}
		select {
		case <-r.Context().Done():
			return
		case <-updated:
		}
	}
}

var streamPartEnd = []byte("\r\n")

func (pool *mjpegPool) hasClients() bool {
	pool.mu.Lock()
	hasClients := len(pool.subscribers) > 0
	pool.mu.Unlock()
	return hasClients
}

func (pool *mjpegPool) subscribe() chan struct{} {
	updated := make(chan struct{}, 1)
	pool.mu.Lock()
	pool.subscribers[updated] = struct{}{}
	pool.mu.Unlock()
	return updated
}

func (pool *mjpegPool) unsubscribe(updated chan struct{}) {
	pool.mu.Lock()
	delete(pool.subscribers, updated)
	pool.mu.Unlock()
}

// streamOutput buffers one frame per worker in addition to active encodes.
// This absorbs short capture bursts without accumulating an unlimited backlog.
var streamOutput struct {
	mu          sync.Mutex
	server      *http.Server
	listener    net.Listener
	frames      chan streamFrame
	done        chan struct{}
	pool        *mjpegPool
	config      streamConfig
	nextCapture time.Time
}

type streamPixelBuffer struct{ pixels []byte }

var streamFrameBufferPool = sync.Pool{New: func() any { return new(streamPixelBuffer) }}

func acquireStreamFrame(width, height int) streamFrame {
	buffer := streamFrameBufferPool.Get().(*streamPixelBuffer)
	size := 4 * width * height
	if cap(buffer.pixels) < size {
		buffer.pixels = make([]byte, size)
	}
	buffer.pixels = buffer.pixels[:size]
	return streamFrame{pixels: buffer.pixels, buffer: buffer, width: width, height: height}
}

func releaseStreamFrame(frame streamFrame) {
	if frame.buffer != nil && cap(frame.pixels) <= 64*1024*1024 {
		streamFrameBufferPool.Put(frame.buffer)
	}
}

func startStreamOutput(config streamConfig) error {
	if !validStreamResolution(config.resolution) {
		config.resolution = 1080
	}
	if config.fps != 15 && config.fps != 30 && config.fps != 60 {
		config.fps = 60
	}
	width, height := eui.ScreenSize()
	if !config.wholeClient {
		width, height = worldViewRect.Dx(), worldViewRect.Dy()
		if width < 1 || height < 1 {
			width, height = gameAreaSizeX, gameAreaSizeY
		}
	}
	config.outputSize, _ = streamCaptureLayout(max(1, width), max(1, height), config)
	standby, err := encodeStreamStandbyFrame(config)
	if err != nil {
		return err
	}
	stopStreamOutput()
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		return err
	}
	pool := newMJPEGPool()
	frames := make(chan streamFrame, streamEncoderWorkers())
	done := make(chan struct{})
	server := &http.Server{
		Handler:           streamHTTPHandler(pool, config, frames),
		ReadHeaderTimeout: 5 * time.Second,
		ConnContext: func(ctx context.Context, connection net.Conn) context.Context {
			// Keep TCP from hiding megabytes of stale JPEGs beyond our bounded
			// frame queue. The current frame can still be written in chunks.
			if tcp, ok := connection.(*net.TCPConn); ok {
				if err := tcp.SetWriteBuffer(64 * 1024); err != nil {
					logError("stream send buffer: %v", err)
				}
			}
			return ctx
		},
	}

	streamOutput.mu.Lock()
	streamOutput.server = server
	streamOutput.listener = listener
	streamOutput.frames = frames
	streamOutput.done = done
	streamOutput.pool = pool
	streamOutput.config = config
	streamOutput.nextCapture = time.Time{}
	streamOutput.mu.Unlock()
	pool.update(standby)
	// Capture already produces the requested size on the GPU.
	go encodeStreamFrames(frames, done, pool, 0)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logError("stream output: %v", err)
		}
	}()
	consoleMessage("stream output started: " + streamOutputURL)
	return nil
}

func validStreamResolution(resolution int) bool {
	switch resolution {
	case 720, 1080, 1440, 2160:
		return true
	default:
		return false
	}
}

func streamHTTPHandler(pool *mjpegPool, config streamConfig, frames chan streamFrame) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><head><style>html,body,img{margin:0;width:100%;height:100%;background:#000;object-fit:contain;overflow:hidden}</style></head><body><img src=\"/stream\"></body></html>"))
	})
	mux.HandleFunc("/stream", pool.serveHTTP)
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		pool.mu.Lock()
		clients, published := len(pool.subscribers), pool.version
		pool.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configured_fps": config.fps, "resolution": config.resolution,
			"workers": streamEncoderWorkers(), "queue_depth": len(frames), "queue_capacity": cap(frames),
			"clients": clients, "captured": pool.metrics.captured.Load(), "queued": pool.metrics.queued.Load(),
			"encoded": pool.metrics.encoded.Load(), "encode_errors": pool.metrics.encodeErrors.Load(),
			"published_including_standby": published, "late_results_dropped": pool.metrics.lateResults.Load(),
			"queue_full_skips": pool.metrics.queueFull.Load(), "client_frames_skipped": pool.metrics.clientSkipped.Load(),
			"write_errors":      pool.metrics.writeErrors.Load(),
			"readback_total_ms": float64(pool.metrics.readbackNS.Load()) / 1e6,
			"encode_total_ms":   float64(pool.metrics.encodeNS.Load()) / 1e6,
		})
	})
	return noCacheHTTP(mux)
}

func noCacheHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setNoCacheHeaders(w.Header())
		next.ServeHTTP(w, r)
	})
}

func setNoCacheHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
}

func stopStreamOutput() {
	streamOutput.mu.Lock()
	server, done := streamOutput.server, streamOutput.done
	streamOutput.server = nil
	streamOutput.listener = nil
	streamOutput.frames = nil
	streamOutput.done = nil
	streamOutput.pool = nil
	streamOutput.nextCapture = time.Time{}
	streamOutput.mu.Unlock()
	if done != nil {
		close(done)
	}
	if server != nil {
		// Streaming responses never finish on their own. Close cancels handlers
		// immediately instead of waiting for a graceful-shutdown timeout.
		_ = server.Close()
	}
}

func streamOutputRunning() bool {
	streamOutput.mu.Lock()
	running := streamOutput.server != nil
	streamOutput.mu.Unlock()
	return running
}

// setStreamOutputSource changes the capture source without changing the fixed
// output canvas selected when streaming began.
func setStreamOutputSource(wholeClient bool) bool {
	streamOutput.mu.Lock()
	defer streamOutput.mu.Unlock()
	if streamOutput.server == nil {
		return false
	}
	if streamOutput.config.wholeClient != wholeClient {
		// Let the newly selected source publish a frame even when the game
		// itself has not changed since the last game-only capture.
		streamCapture.captured = false
	}
	streamOutput.config.wholeClient = wholeClient
	return true
}

// This staging image belongs to the render thread. Read last draw's capture
// before issuing this draw's graphics commands, then reuse it for the next one.
// ReadPixels is still synchronous; staging avoids flushing the new world/UI
// draw just to capture it, and bounds the transfer to the requested resolution.
var streamCapture struct {
	frames   chan streamFrame
	image    *ebiten.Image
	pending  bool
	captured bool
}

func readPendingStreamOutput() {
	streamOutput.mu.Lock()
	frames, pool := streamOutput.frames, streamOutput.pool
	streamOutput.mu.Unlock()
	if streamCapture.frames != frames {
		if streamCapture.image != nil {
			streamCapture.image.Deallocate()
			streamCapture.image = nil
		}
		streamCapture.frames = frames
		streamCapture.pending = false
		streamCapture.captured = false
	}
	if !streamCapture.pending {
		return
	}
	streamCapture.pending = false
	if frames == nil || pool == nil || !pool.hasClients() {
		return
	}
	if len(frames) == cap(frames) {
		pool.metrics.queueFull.Add(1)
		return
	}
	bounds := streamCapture.image.Bounds()
	frame := acquireStreamFrame(bounds.Dx(), bounds.Dy())
	readStarted := time.Now()
	streamCapture.image.ReadPixels(frame.pixels)
	pool.metrics.readbackNS.Add(uint64(time.Since(readStarted)))
	if framePacingTraceThreshold > 0 {
		framePacingStreamReadback.Store(int64(time.Since(readStarted)))
	}
	streamOutput.mu.Lock()
	defer streamOutput.mu.Unlock()
	if streamOutput.frames != frames {
		releaseStreamFrame(frame)
		return
	}
	select {
	case frames <- frame:
		pool.metrics.queued.Add(1)
	default:
		pool.metrics.queueFull.Add(1)
		releaseStreamFrame(frame)
	}
}

// Keep capture on a fixed clock. A small tolerance absorbs display scheduling
// jitter; measuring a full interval from the last actual draw can turn 60 FPS
// capture into alternating 60/30 FPS whenever a draw arrives slightly early.
func streamCaptureDue(now, next time.Time, fps int) bool {
	if next.IsZero() {
		return true
	}
	interval := time.Second / time.Duration(max(1, fps))
	tolerance := interval / 8
	if tolerance > time.Millisecond {
		tolerance = time.Millisecond
	}
	return !now.Add(tolerance).Before(next)
}

func advanceStreamCapture(now, next time.Time, fps int) time.Time {
	interval := time.Second / time.Duration(max(1, fps))
	if next.IsZero() {
		return now.Add(interval)
	}
	// Skip expired slots after a long stall rather than emitting a catch-up burst.
	slots := max(int64(1), int64(now.Sub(next)/interval)+1)
	return next.Add(time.Duration(slots) * interval)
}

// captureStreamOutput only queues GPU work. CPU readback happens at the start
// of the next Draw, after Ebitengine has submitted this frame for presentation.
func captureStreamOutput(screen *ebiten.Image, gameChanged bool) {
	now := time.Now()
	streamOutput.mu.Lock()
	frames, pool, config := streamOutput.frames, streamOutput.pool, streamOutput.config
	if frames == nil || pool == nil || (!config.wholeClient && !gameChanged && streamCapture.captured) || !pool.hasClients() || !streamCaptureDue(now, streamOutput.nextCapture, config.fps) {
		streamOutput.mu.Unlock()
		return
	}
	if len(frames) == cap(frames) {
		pool.metrics.queueFull.Add(1)
		streamOutput.nextCapture = advanceStreamCapture(now, streamOutput.nextCapture, config.fps)
		streamOutput.mu.Unlock()
		return
	}
	streamOutput.mu.Unlock()

	source := screen
	if !config.wholeClient {
		if gameImage == nil || worldViewRect.Empty() {
			return
		}
		source = gameImage.SubImage(worldViewRect).(*ebiten.Image)
	}
	bounds := source.Bounds()
	if bounds.Empty() || streamCapture.frames != frames {
		return
	}
	size := config.outputSize
	target := streamFitRect(bounds.Dx(), bounds.Dy(), size.X, size.Y)
	width, height := size.X, size.Y
	if streamCapture.image == nil || streamCapture.image.Bounds().Size() != image.Pt(width, height) {
		if streamCapture.image != nil {
			streamCapture.image.Deallocate()
		}
		streamCapture.image = newUnmanagedImage(width, height)
	}
	destination := streamCapture.image
	var options ebiten.DrawImageOptions
	if bounds.Size() == size {
		// An exact-size source, including native capture before any resize,
		// needs only a 1:1 copy.
		options.Blend = ebiten.BlendCopy
	} else {
		if target.Size() != size {
			destination.Fill(color.Black)
		} else {
			options.Blend = ebiten.BlendCopy
		}
		options.GeoM.Scale(float64(target.Dx())/float64(bounds.Dx()), float64(target.Dy())/float64(bounds.Dy()))
		options.GeoM.Translate(float64(target.Min.X), float64(target.Min.Y))
		options.Filter = ebiten.FilterLinear
	}
	if !config.wholeClient && config.idleBlack && showingGameSplash() {
		destination.Fill(color.Black)
	} else {
		destination.DrawImage(source, &options)
	}
	streamCapture.pending = true
	streamCapture.captured = true
	pool.metrics.captured.Add(1)
	streamOutput.mu.Lock()
	if streamOutput.frames == frames {
		streamOutput.nextCapture = advanceStreamCapture(now, streamOutput.nextCapture, config.fps)
	}
	streamOutput.mu.Unlock()
}

func encodeStreamFrames(frames <-chan streamFrame, done <-chan struct{}, pool *mjpegPool, resolution int) {
	encodeStreamFramesWithWorkers(frames, done, pool, streamEncoderWorkers(), func() func(streamFrame) ([]byte, error) {
		var encoder streamFrameEncoder
		return func(frame streamFrame) ([]byte, error) {
			return encoder.encode(frame, resolution)
		}
	})
}

func encodeStreamFramesWithWorkers(frames <-chan streamFrame, done <-chan struct{}, pool *mjpegPool, workerCount int, newEncoder func() func(streamFrame) ([]byte, error)) {
	encoded := make(chan encodedStreamFrame, workerCount)
	var workers sync.WaitGroup
	// Serialize dequeue and numbering so scheduling cannot reverse capture order.
	var receive sync.Mutex
	var sequence uint64
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			encode := newEncoder()
			reusable := make(chan struct{}, 1)
			for {
				receive.Lock()
				select {
				case <-done:
					receive.Unlock()
					return
				case frame := <-frames:
					sequence++
					frameSequence := sequence
					receive.Unlock()
					if !pool.hasClients() {
						releaseStreamFrame(frame)
						continue
					}
					encodeStarted := time.Now()
					jpegFrame, err := encode(frame)
					pool.metrics.encodeNS.Add(uint64(time.Since(encodeStarted)))
					releaseStreamFrame(frame)
					if err != nil {
						pool.metrics.encodeErrors.Add(1)
						logError("stream frame: %v", err)
						continue
					}
					pool.metrics.encoded.Add(1)
					select {
					case encoded <- encodedStreamFrame{jpeg: jpegFrame, sequence: frameSequence, reusable: reusable}:
						// The publisher copies into its reusable latest-frame buffer
						// before this worker may overwrite its JPEG storage.
						select {
						case <-reusable:
						case <-done:
							return
						}
					case <-done:
						return
					}
				}
			}
		}()
	}
	go func() {
		workers.Wait()
		for {
			select {
			case frame := <-frames:
				releaseStreamFrame(frame)
			default:
				close(encoded)
				return
			}
		}
	}()

	defer func() {
		for range encoded {
		}
	}()
	var newest uint64
	for {
		select {
		case <-done:
			return
		case result, ok := <-encoded:
			if !ok {
				return
			}
			// Publish completed work immediately; late frames and encoder errors
			// must never hold newer frames in an unbounded reorder queue.
			if result.sequence > newest {
				pool.update(result.jpeg)
				newest = result.sequence
			} else {
				pool.metrics.lateResults.Add(1)
			}
			result.reusable <- struct{}{}
		}
	}
}

// Keep encoding from occupying every core on large machines, and respect
// container CPU limits reflected by GOMAXPROCS.
func streamEncoderWorkers() int { return max(1, min(4, runtime.GOMAXPROCS(0))) }

type streamFrameEncoder struct {
	destination *image.RGBA
	target      image.Rectangle
	encoded     bytes.Buffer
	source      image.RGBA
}

func encodeStreamFrame(frame streamFrame, resolution int) ([]byte, error) {
	var encoder streamFrameEncoder
	return encoder.encode(frame, resolution)
}

// encode returns storage owned by this worker, valid until its next encode.
func (encoder *streamFrameEncoder) encode(frame streamFrame, resolution int) ([]byte, error) {
	if frame.width < 1 || frame.height < 1 || len(frame.pixels) != 4*frame.width*frame.height {
		return nil, image.ErrFormat
	}
	encoder.source = image.RGBA{Pix: frame.pixels, Stride: frame.width * 4, Rect: image.Rect(0, 0, frame.width, frame.height)}
	source := &encoder.source
	var output image.Image = source
	if resolution != 0 {
		width := resolution * 16 / 9
		if frame.width != width || frame.height != resolution {
			bounds := image.Rect(0, 0, width, resolution)
			if encoder.destination == nil || encoder.destination.Bounds() != bounds {
				encoder.destination = image.NewRGBA(bounds)
				encoder.target = image.Rectangle{}
			}
			destination := encoder.destination
			target := streamFitRect(frame.width, frame.height, width, resolution)
			// Letterbox bars only need repainting when the fitted rectangle
			// changes. The scaler overwrites every pixel inside the target.
			if target != encoder.target {
				for _, bar := range [...]image.Rectangle{
					image.Rect(0, 0, width, target.Min.Y),
					image.Rect(0, target.Max.Y, width, resolution),
					image.Rect(0, target.Min.Y, target.Min.X, target.Max.Y),
					image.Rect(target.Max.X, target.Min.Y, width, target.Max.Y),
				} {
					stdraw.Draw(destination, bar, image.Black, image.Point{}, stdraw.Src)
				}
				encoder.target = target
			}
			imagedraw.ApproxBiLinear.Scale(destination, target, source, source.Bounds(), stdraw.Src, nil)
			output = destination
		}
	}
	encoder.encoded.Reset()
	// Reserve a practical initial JPEG capacity; detailed scenes may grow it
	// once, and subsequent frames retain that capacity.
	encoder.encoded.Grow(output.Bounds().Dx() * output.Bounds().Dy() / 4)
	if err := jpeg.Encode(&encoder.encoded, output, &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}
	return encoder.encoded.Bytes(), nil
}

// The standby artwork is fitted inside the session's output canvas; its own
// aspect ratio must never choose the JPEG dimensions seen by the decoder.
func encodeStreamStandbyFrame(config streamConfig) ([]byte, error) {
	size := config.outputSize
	if size.X < 1 || size.Y < 1 {
		return nil, image.ErrFormat
	}
	destination := image.NewRGBA(image.Rectangle{Max: size})
	stdraw.Draw(destination, destination.Bounds(), image.Black, image.Point{}, stdraw.Src)
	if !config.idleBlack {
		splash, err := png.Decode(bytes.NewReader(splashPNG))
		if err != nil {
			return nil, err
		}
		target := streamFitRect(splash.Bounds().Dx(), splash.Bounds().Dy(), size.X, size.Y)
		imagedraw.ApproxBiLinear.Scale(destination, target, splash, splash.Bounds(), stdraw.Src, nil)
	}
	return encodeStreamFrame(streamFrame{pixels: destination.Pix, width: size.X, height: size.Y}, 0)
}

// Fixed resolutions retain their full 16:9 canvas so a live source change does
// not alter the dimensions configured in a Browser Source.
func streamCaptureLayout(width, height int, config streamConfig) (image.Point, image.Rectangle) {
	if config.resolution == 0 {
		size := image.Pt(width, height)
		return size, image.Rectangle{Max: size}
	}
	size := image.Pt(config.resolution*16/9, config.resolution)
	target := streamFitRect(width, height, size.X, size.Y)
	return size, target
}

func streamFitRect(sourceW, sourceH, targetW, targetH int) image.Rectangle {
	if sourceW*targetH > sourceH*targetW {
		h := max(1, sourceH*targetW/sourceW)
		return image.Rect(0, (targetH-h)/2, targetW, (targetH-h)/2+h)
	}
	w := max(1, sourceW*targetH/sourceH)
	return image.Rect((targetW-w)/2, 0, (targetW-w)/2+w, targetH)
}
