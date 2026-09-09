package main

import "testing"

func BenchmarkStreamEncode(b *testing.B) {
	frame := streamFrame{width: 800, height: 600, pixels: make([]byte, 800*600*4)}
	for y := range frame.height {
		for x := range frame.width {
			i := (y*frame.width + x) * 4
			frame.pixels[i] = byte(x / 3)
			frame.pixels[i+1] = byte(y / 3)
			frame.pixels[i+2] = byte((x ^ y) / 4)
			frame.pixels[i+3] = 255
		}
	}
	for _, tc := range []struct {
		name       string
		resolution int
	}{{"native", 0}, {"720p", 720}, {"1080p", 1080}, {"1440p", 1440}, {"4K", 2160}} {
		b.Run(tc.name, func(b *testing.B) {
			var encoder streamFrameEncoder
			if _, err := encoder.encode(frame, tc.resolution); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := encoder.encode(frame, tc.resolution); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkStreamPublish(b *testing.B) {
	pool := newMJPEGPool()
	subscriber := pool.subscribe()
	defer pool.unsubscribe(subscriber)
	jpeg := make([]byte, 128*1024)
	pool.update(jpeg)
	b.ReportAllocs()
	for b.Loop() {
		pool.update(jpeg)
		<-subscriber
	}
}

func BenchmarkStreamCaptureBuffer(b *testing.B) {
	frame := acquireStreamFrame(1920, 1080)
	releaseStreamFrame(frame)
	b.ReportAllocs()
	for b.Loop() {
		frame := acquireStreamFrame(1920, 1080)
		releaseStreamFrame(frame)
	}
}
