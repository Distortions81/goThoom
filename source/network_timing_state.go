package main

import (
	"sync"
	"time"
)

// networkTimingState owns cadence measurements and the adaptive networking
// controller for one session. The safety-percentage setting remains app-owned.
type networkTimingState struct {
	wake chan struct{}

	cadenceMu        sync.Mutex
	lastFrameAt      time.Time
	interval         time.Duration
	lastFrame        int32
	samples          []timedDurationSample
	jitter           time.Duration
	updatesPerSecond float64

	replyMu sync.Mutex
	reply   time.Duration

	controllerMu sync.Mutex
	controller   pnaControllerState

	fallbackMu sync.Mutex
	fallback   pnaFallbackState
}

func newNetworkTimingState() *networkTimingState {
	return &networkTimingState{
		wake:     make(chan struct{}, 1),
		interval: framems * time.Millisecond,
	}
}

func (s *networkTimingState) resetCadence() {
	s.cadenceMu.Lock()
	s.lastFrameAt = time.Time{}
	s.interval = framems * time.Millisecond
	s.lastFrame = 0
	s.samples = nil
	s.jitter = 0
	s.updatesPerSecond = 0
	s.cadenceMu.Unlock()
}

func (s *networkTimingState) resetReply() {
	s.replyMu.Lock()
	s.reply = 0
	s.replyMu.Unlock()
}

func (s *networkTimingState) recordReply(reply time.Duration) {
	if reply < 0 {
		return
	}
	s.replyMu.Lock()
	s.reply = reply
	s.replyMu.Unlock()
}

func (s *networkTimingState) snapshot() (reply, jitter time.Duration) {
	s.replyMu.Lock()
	reply = s.reply
	s.replyMu.Unlock()
	s.cadenceMu.Lock()
	jitter = s.jitter
	s.cadenceMu.Unlock()
	return reply, jitter
}

func (s *networkTimingState) setInterval(interval time.Duration) {
	s.cadenceMu.Lock()
	s.interval = interval
	s.cadenceMu.Unlock()
}

func (s *networkTimingState) cadenceSnapshot() (last time.Time, interval, jitter time.Duration, updatesPerSecond float64, sampleCount int) {
	s.cadenceMu.Lock()
	defer s.cadenceMu.Unlock()
	return s.lastFrameAt, s.interval, s.jitter, s.updatesPerSecond, len(s.samples)
}

func (s *networkTimingState) resetController() {
	s.controllerMu.Lock()
	s.controller = pnaControllerState{}
	s.controllerMu.Unlock()
}

func (s *networkTimingState) resetFallback() {
	s.fallbackMu.Lock()
	s.fallback = pnaFallbackState{}
	s.fallbackMu.Unlock()
}

func (s *networkTimingState) drainWake() {
	for {
		select {
		case <-s.wake:
		default:
			return
		}
	}
}
