package main

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

func TestClassicMovementUsesCurrentInput(t *testing.T) {
	old := gs.MotionSmoothing
	gs.MotionSmoothing = false
	t.Cleanup(func() { gs.MotionSmoothing = old })
	east := inputState{mouseX: 100, mouseDown: true}
	north := inputState{mouseY: -100, mouseDown: true}
	west := inputState{mouseX: -100, mouseDown: true}
	stop := inputState{mouseX: -100}
	for _, tc := range []struct {
		name    string
		initial inputState
		inputs  []inputState
		want    inputState
	}{
		{"latest direction", east, []inputState{north, west}, west},
		{"release supersedes direction", east, []inputState{north, west, stop}, stop},
		{"return to sent direction", east, []inputState{north, east}, east},
		{"released before sample", inputState{}, []inputState{west, stop}, stop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := newSessionInputState()
			input.latest = tc.initial
			for _, state := range tc.inputs {
				input.enqueue(state)
			}
			if got := input.next(); got != tc.want {
				t.Fatalf("next send = %+v, want current input %+v", got, tc.want)
			}
			if got := input.next(); got != tc.want {
				t.Fatalf("subsequent send replayed stale input: %+v", got)
			}
		})
	}
}

type latencyStepConn struct {
	bufConn
	write func([]byte) (int, error)
}

func (c *latencyStepConn) Write(packet []byte) (int, error) { return c.write(packet) }

func TestPNAInputLimiterRecovery(t *testing.T) {
	old := gs
	gs.AltNetMode = true
	t.Cleanup(func() { gs = old })
	for _, mode := range []struct {
		name      string
		smoothing bool
	}{{"classic", false}, {"smoothed", true}} {
		t.Run(mode.name, func(t *testing.T) {
			gs.MotionSmoothing = mode.smoothing
			for _, scenario := range []string{"failed write", "failed write with new movement", "fallback movement", "resend request"} {
				t.Run(scenario, func(t *testing.T) {
					// The phase deadline has already passed; only the duplicate-input
					// limiter can defer these sends. Each write triggers the next wake.
					s := latencyTestSession(time.Now().Add(-50*time.Millisecond), 50*time.Millisecond)
					initial := inputState{mouseX: 100, mouseDown: true}
					s.input.enqueue(initial)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					attempts := 0
					conn := &latencyStepConn{write: func(packet []byte) (int, error) {
						attempts++
						wantX := initial.mouseX
						if attempts == 2 && (scenario == "fallback movement" || scenario == "failed write with new movement") {
							wantX = -100
						}
						if gotX := int16(binary.BigEndian.Uint16(packet[4:6])); gotX != wantX {
							t.Errorf("write %d used movement %d, want current direction %d", attempts, gotX, wantX)
						}
						if attempts == 1 {
							switch scenario {
							case "failed write", "failed write with new movement":
								if scenario == "failed write with new movement" {
									s.input.enqueue(inputState{mouseX: -100, mouseDown: true})
								}
								s.timing.wake <- struct{}{}
								return 0, errors.New("temporary write failure")
							case "fallback movement":
								s.timing.pausePNA("unstable frame timing", time.Now())
								s.input.enqueue(inputState{mouseX: -100, mouseDown: true})
							case "resend request":
								s.frames.requestResend(101)
							}
							s.timing.wake <- struct{}{}
						} else if scenario == "fallback movement" && attempts == 2 {
							s.timing.resetFallback()
							s.input.enqueue(initial)
							s.timing.wake <- struct{}{}
						} else {
							cancel()
						}
						return len(packet), nil
					}}
					done := make(chan struct{})
					go func() {
						defer close(done)
						sendSessionInputLoop(s, ctx, conn, nil)
					}()
					s.timing.wake <- struct{}{}
					select {
					case <-done:
					case <-time.After(time.Second):
						cancel()
						<-done
						t.Fatalf("limiter suppressed recovery input after %d writes", attempts)
					}
				})
			}
		})
	}
}

func TestSwitchingToClassicMovementDiscardsBacklog(t *testing.T) {
	old := gs.MotionSmoothing
	t.Cleanup(func() { gs.MotionSmoothing = old })
	gs.MotionSmoothing = true
	input := newSessionInputState()
	input.enqueue(inputState{mouseX: 100, mouseDown: true})
	stop := inputState{mouseX: 100}
	input.enqueue(stop)
	gs.MotionSmoothing = false
	if got := input.next(); got != stop {
		t.Fatalf("switch replayed old movement: %+v", got)
	}
}

func TestSmoothedMovementCoalescesDirectionsAndPreservesClick(t *testing.T) {
	old := gs.MotionSmoothing
	gs.MotionSmoothing = true
	t.Cleanup(func() { gs.MotionSmoothing = old })
	east := inputState{mouseX: 100, mouseDown: true}
	north := inputState{mouseY: -100, mouseDown: true}
	west := inputState{mouseX: -100, mouseDown: true}
	hover := inputState{mouseX: -100}
	for _, tc := range []struct {
		name    string
		initial inputState
		inputs  []inputState
		want    []inputState
	}{
		{"latest held direction", east, []inputState{north, west}, []inputState{west, west}},
		{"release supersedes direction", east, []inputState{north, west, hover}, []inputState{hover, hover}},
		{"return to sent direction", east, []inputState{north, east}, []inputState{east, east}},
		{"short click", inputState{}, []inputState{west, hover}, []inputState{west, hover}},
		{"short click after hover", inputState{}, []inputState{hover, west, hover}, []inputState{west, hover}},
		{"repress uses current direction", inputState{}, []inputState{west, hover, north}, []inputState{north, north}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := newSessionInputState()
			input.latest = tc.initial
			for _, state := range tc.inputs {
				input.enqueue(state)
			}
			for i, want := range tc.want {
				if got := input.next(); got != want {
					t.Fatalf("send %d = %+v, want %+v", i+1, got, want)
				}
			}
		})
	}
}

// Seed actual cadence observations, so tests exercise the same readiness and
// health checks as a live session without a server or a rendered window.
func latencyTestSession(now time.Time, interval time.Duration) *Session {
	s := mustNewSession(2)
	for i := 0; i <= 100; i++ {
		s.timing.recordServerFrame(int32(i+1), now.Add(time.Duration(i-100)*interval))
	}
	s.frames.set(101, 0)
	return s
}

func TestPNASlowRepliesDoNotEncourageLaterSends(t *testing.T) {
	old := gs.AltNetMode
	gs.AltNetMode = true
	t.Cleanup(func() { gs.AltNetMode = old })
	now := time.Now()
	s := latencyTestSession(now, 200*time.Millisecond)
	initial := s.timing.pnaLead(200*time.Millisecond, 0)
	for i := 0; i < 12; i++ {
		s.recordPNACommandFeedback(190*time.Millisecond, 200*time.Millisecond-initial,
			200*time.Millisecond, 101, 102, now.Add(time.Duration(i)*time.Millisecond))
	}
	if got := s.timing.pnaLead(200*time.Millisecond, 0); got != initial {
		t.Fatalf("190ms replies moved the send later: lead %s -> %s", initial, got)
	}
	for _, reply := range []time.Duration{250 * time.Millisecond, 260 * time.Millisecond} {
		s.recordPNACommandFeedback(reply, 150*time.Millisecond, 200*time.Millisecond, 101, 103, now)
		if use, reason := s.timing.pnaStatus(0, now); use || reason != "cooldown after slow command replies" {
			t.Fatalf("%s reply did not pause prediction: %t %q", reply, use, reason)
		}
	}
	if use, _ := s.timing.pnaStatus(0, now.Add(pnaFallbackCooldown-time.Millisecond)); use {
		t.Fatal("prediction resumed before cooldown")
	}
	if use, reason := s.timing.pnaStatus(0, now.Add(pnaFallbackCooldown)); !use {
		t.Fatalf("healthy connection did not recover: %s", reason)
	}
	if got := s.timing.pnaLead(200*time.Millisecond, 0); got < 100*time.Millisecond {
		t.Fatalf("recovery did not retain an earlier send: lead %s", got)
	}
}

func TestPNAIsolatedArrivalSpikePausesBeforeLongWindowJitter(t *testing.T) {
	now := time.Now()
	s := latencyTestSession(now, 200*time.Millisecond)
	late := now.Add(285 * time.Millisecond)
	s.timing.recordServerFrame(102, late)
	_, _, jitter, _, _ := s.timing.cadenceSnapshot()
	if jitter != 0 {
		t.Fatalf("fixture should hide the isolated spike from p95, got %s", jitter)
	}
	if use, reason := s.timing.pnaStatus(0, late); use || reason != "unstable frame timing" {
		t.Fatalf("arrival spike did not pause: %t %q", use, reason)
	}
	// The next packet catches up to the regular cadence. Both the late and
	// bunched arrivals are unsafe origins for a new predictive wait.
	s.timing.recordServerFrame(103, now.Add(400*time.Millisecond))
	if use, _ := s.timing.pnaStatus(0, now.Add(400*time.Millisecond)); use {
		t.Fatal("bunched packet immediately resumed prediction")
	}
	recovery := now.Add(6 * time.Second)
	s.timing.cadenceMu.Lock()
	s.timing.samples = append(s.timing.samples, timedDurationSample{at: recovery, value: 235 * time.Millisecond})
	s.timing.cadenceMu.Unlock()
	if use, reason := s.timing.pnaStatus(0, recovery); use || reason != "waiting for frame timing to settle" {
		t.Fatalf("moderate jitter bypassed recovery hysteresis: %t %q", use, reason)
	}
	if use, reason := s.timing.pnaStatus(0, recovery.Add(pnaHealthWindow+time.Millisecond)); !use {
		t.Fatalf("old spike blocked recovery: %s", reason)
	}
}

type latencyTestConn struct {
	bufConn
	packets chan []byte
}

func (c *latencyTestConn) Write(packet []byte) (int, error) {
	c.packets <- append([]byte(nil), packet...)
	return len(packet), nil
}

func TestPNABunchedFrameFlushesLatestMovement(t *testing.T) {
	old := gs
	gs.MotionSmoothing, gs.AltNetMode = false, true
	t.Cleanup(func() { gs = old })
	now := time.Now()
	s := latencyTestSession(now, time.Second)
	s.input.latest = inputState{mouseX: 100, mouseDown: true}
	queueSessionInput(s, inputState{mouseY: -100, mouseDown: true})
	stop := inputState{mouseY: -100}
	queueSessionInput(s, stop)
	conn := &latencyTestConn{packets: make(chan []byte, 8)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sendSessionInputLoop(s, ctx, conn, nil)
	}()
	t.Cleanup(func() { cancel(); <-done })
	s.timing.wake <- struct{}{}
	select {
	case <-conn.packets:
		t.Fatal("healthy prediction did not wait for its phase")
	case <-time.After(20 * time.Millisecond):
	}
	s.frames.set(102, 0)
	s.noteFrameAt(102, time.Now())
	select {
	case packet := <-conn.packets:
		payload := packet[2:]
		got := inputState{
			mouseX:    int16(binary.BigEndian.Uint16(payload[2:4])),
			mouseY:    int16(binary.BigEndian.Uint16(payload[4:6])),
			mouseDown: binary.BigEndian.Uint16(payload[6:8])&kPIMDownField != 0,
		}
		if got != stop {
			t.Fatalf("sent stale movement: %+v, want %+v", got, stop)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("early frame postponed input into another predicted cycle")
	}
}

func TestPNASameFrameNotificationDoesNotFlushOrPreventCancellation(t *testing.T) {
	old := gs.AltNetMode
	gs.AltNetMode = true
	t.Cleanup(func() { gs.AltNetMode = old })
	s := latencyTestSession(time.Now(), time.Second)
	s.timing.wake <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { done <- s.waitForPNASend(ctx) }()
	select {
	case <-done:
		cancel()
		t.Fatal("same-frame wake bypassed the deadline")
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	if <-done {
		t.Fatal("cancelled wait allowed a send")
	}
}

func TestPNAResendDoesNotWaitForPredictedPhase(t *testing.T) {
	old := gs.AltNetMode
	gs.AltNetMode = true
	t.Cleanup(func() { gs.AltNetMode = old })
	s := latencyTestSession(time.Now(), time.Second)
	s.frames.requestResend(101)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if !s.waitForPNASend(ctx) {
		t.Fatal("resend request waited for the predicted movement phase")
	}
}
