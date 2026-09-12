package main

import (
	"sync"
	"time"
)

// frameState owns acknowledgement, resend, and packet-loss accounting for one
// session. Transport timing and PNA tuning will join this boundary separately.
type frameState struct {
	stateMu sync.RWMutex
	ack     int32
	resend  int32

	statsMu     sync.Mutex
	lastAck     int32
	received    int
	lost        int
	frames      [5]int
	lostFrames  [5]int
	bucketTimes [5]int64
}

func newFrameState() *frameState {
	return &frameState{}
}

func (s *frameState) snapshot() (int32, int32) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.ack, s.resend
}

func (s *frameState) acknowledged() int32 {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.ack
}

func (s *frameState) set(ack, resend int32) {
	s.stateMu.Lock()
	s.ack = ack
	s.resend = resend
	s.stateMu.Unlock()
}

func (s *frameState) requestResend(resend int32) {
	s.stateMu.Lock()
	s.resend = resend
	s.stateMu.Unlock()
}

func (s *frameState) resetStatistics() {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.lastAck = 0
	s.received = 0
	s.lost = 0
	for i := range s.frames {
		s.frames[i] = 0
		s.lostFrames[i] = 0
		s.bucketTimes[i] = 0
	}
}

// updateCounters tracks frame statistics and returns the number of missing
// frames between the previous and current acknowledgement numbers.
func (s *frameState) updateCounters(newFrame int32) int {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	now := time.Now().Unix()
	idx := int(now % int64(len(s.frames)))
	if s.bucketTimes[idx] != now {
		s.frames[idx] = 0
		s.lostFrames[idx] = 0
		s.bucketTimes[idx] = now
	}
	// Ignore out-of-order or duplicate frames which can occur on UDP.
	if s.lastAck != 0 && newFrame <= s.lastAck {
		return 0
	}

	s.frames[idx]++
	s.received++
	dropped := 0
	if s.lastAck != 0 {
		missing := int(newFrame - s.lastAck - 1)
		if missing > 0 {
			s.lost += missing
			dropped = missing
			s.frames[idx] += missing
			s.lostFrames[idx] += missing
		}
	}
	s.lastAck = newFrame
	return dropped
}

func (s *frameState) droppedPercent() float64 {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	now := time.Now().Unix()
	total, lost := 0, 0
	for i := range s.frames {
		if now-s.bucketTimes[i] < int64(len(s.frames)) {
			total += s.frames[i]
			lost += s.lostFrames[i]
		}
	}
	if total == 0 {
		return 0
	}
	return float64(lost) * 100 / float64(total)
}

// packetLoss reports the recent rolling loss percentage alongside the
// whole-session loss percentage and counts.
func (s *frameState) packetLoss() (recent, session float64, received, lost int) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	now := time.Now().Unix()
	recentTotal, recentLost := 0, 0
	for i := range s.frames {
		if now-s.bucketTimes[i] < int64(len(s.frames)) {
			recentTotal += s.frames[i]
			recentLost += s.lostFrames[i]
		}
	}
	if recentTotal > 0 {
		recent = float64(recentLost) * 100 / float64(recentTotal)
	}
	received, lost = s.received, s.lost
	if total := received + lost; total > 0 {
		session = float64(lost) * 100 / float64(total)
	}
	return recent, session, received, lost
}
