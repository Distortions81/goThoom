package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// bufConn is a simple in-memory net.Conn implementation that collects writes.
type bufConn struct{ bytes.Buffer }

func (c *bufConn) Read(b []byte) (int, error)         { return 0, io.EOF }
func (c *bufConn) Write(b []byte) (int, error)        { return c.Buffer.Write(b) }
func (c *bufConn) Close() error                       { return nil }
func (c *bufConn) LocalAddr() net.Addr                { return dummyAddr{} }
func (c *bufConn) RemoteAddr() net.Addr               { return dummyAddr{} }
func (c *bufConn) SetDeadline(t time.Time) error      { return nil }
func (c *bufConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *bufConn) SetWriteDeadline(t time.Time) error { return nil }

type writeErrorConn struct{ bufConn }

func (c *writeErrorConn) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type dummyAddr struct{}

func (dummyAddr) Network() string { return "dummy" }
func (dummyAddr) String() string  { return "dummy" }

// packetConn returns predetermined datagrams on each Read call.
type packetConn struct {
	packets [][]byte
	idx     int
}

func (c *packetConn) Read(b []byte) (int, error) {
	if c.idx >= len(c.packets) {
		return 0, io.EOF
	}
	p := c.packets[c.idx]
	c.idx++
	return copy(b, p), nil
}

func (c *packetConn) Write(b []byte) (int, error)        { return len(b), nil }
func (c *packetConn) Close() error                       { return nil }
func (c *packetConn) LocalAddr() net.Addr                { return dummyAddr{} }
func (c *packetConn) RemoteAddr() net.Addr               { return dummyAddr{} }
func (c *packetConn) SetDeadline(t time.Time) error      { return nil }
func (c *packetConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *packetConn) SetWriteDeadline(t time.Time) error { return nil }

// chunkConn exposes a byte stream in deliberately small reads.
type chunkConn struct {
	chunks [][]byte
	idx    int
	offset int
}

func (c *chunkConn) Read(b []byte) (int, error) {
	for c.idx < len(c.chunks) {
		chunk := c.chunks[c.idx]
		if c.offset >= len(chunk) {
			c.idx++
			c.offset = 0
			continue
		}
		n := copy(b, chunk[c.offset:])
		c.offset += n
		return n, nil
	}
	return 0, io.EOF
}

func (c *chunkConn) Write(b []byte) (int, error)        { return len(b), nil }
func (c *chunkConn) Close() error                       { return nil }
func (c *chunkConn) LocalAddr() net.Addr                { return dummyAddr{} }
func (c *chunkConn) RemoteAddr() net.Addr               { return dummyAddr{} }
func (c *chunkConn) SetDeadline(t time.Time) error      { return nil }
func (c *chunkConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *chunkConn) SetWriteDeadline(t time.Time) error { return nil }

type shortWriteConn struct {
	bufConn
	writes int
}

func (c *shortWriteConn) Write(b []byte) (int, error) {
	c.writes++
	if len(b) == 0 {
		return 0, nil
	}
	return len(b) - 1, nil
}

// extractCommand reads the command number from a packet written to bufConn.
func extractCommand(t *testing.T, buf *bufConn) uint32 {
	data := buf.Bytes()
	if len(data) < 22 { // size (2) + header (20)
		t.Fatalf("packet too small: %d bytes", len(data))
	}
	size := int(binary.BigEndian.Uint16(data[:2]))
	if len(data) < 2+size {
		t.Fatalf("incomplete packet: got %d want %d", len(data)-2, size)
	}
	pkt := data[2 : 2+size]
	return binary.BigEndian.Uint32(pkt[16:20])
}

func extractCommandText(t *testing.T, buf *bufConn) string {
	t.Helper()
	data := buf.Bytes()
	if len(data) < 22 {
		t.Fatalf("packet too small: %d bytes", len(data))
	}
	size := int(binary.BigEndian.Uint16(data[:2]))
	pkt := data[2 : 2+size]
	cmdField := pkt[20:]
	nul := bytes.IndexByte(cmdField, 0)
	if nul < 0 {
		t.Fatal("missing command terminator")
	}
	return string(cmdField[:nul])
}

func resetCommandStateForTest(t testing.TB, number uint32) {
	t.Helper()
	oldCommandNum := primarySession.commands.number
	oldTicket := primarySession.commands.pendingTicket
	oldPending := primarySession.commands.pending
	oldPendingID := primarySession.commands.pendingID
	oldPendingSent := primarySession.commands.pendingSent
	oldPendingSentAt := primarySession.commands.pendingSentAt
	oldPendingSentFrame := primarySession.commands.pendingSentFrame
	oldPendingSentPhase := primarySession.commands.pendingSentPhase
	oldPendingSentInterval := primarySession.commands.pendingSentInterval
	oldPendingSentPredictively := primarySession.commands.pendingSentPredictively
	oldQueue := primarySession.commands.queue
	oldWhoLastCommandFrame := primarySession.commands.lastCommandFrame
	t.Cleanup(func() {
		primarySession.commands.number = oldCommandNum
		primarySession.commands.pendingTicket = oldTicket
		primarySession.commands.pending = oldPending
		primarySession.commands.pendingID = oldPendingID
		primarySession.commands.pendingSent = oldPendingSent
		primarySession.commands.pendingSentAt = oldPendingSentAt
		primarySession.commands.pendingSentFrame = oldPendingSentFrame
		primarySession.commands.pendingSentPhase = oldPendingSentPhase
		primarySession.commands.pendingSentInterval = oldPendingSentInterval
		primarySession.commands.pendingSentPredictively = oldPendingSentPredictively
		primarySession.commands.queue = oldQueue
		primarySession.commands.lastCommandFrame = oldWhoLastCommandFrame
	})
	primarySession.commands.number = number
	primarySession.commands.pendingTicket = nil
	primarySession.commands.pending = ""
	primarySession.commands.pendingID = 0
	primarySession.commands.pendingSent = false
	primarySession.commands.resetPendingTimingLocked()
	primarySession.commands.queue = nil
	primarySession.commands.lastCommandFrame = -1
}

func TestSendPlayerInputEmptyFramesKeepCommandNum(t *testing.T) {
	resetCommandStateForTest(t, 1)

	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := primarySession.commands.number, uint32(1); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn); cmd != 1 {
		t.Fatalf("packet command=%d, want 1", cmd)
	}

	conn2 := &bufConn{}
	if err := sendPlayerInput(conn2, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := primarySession.commands.number, uint32(1); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn2); cmd != 1 {
		t.Fatalf("packet command=%d, want 1", cmd)
	}
}

func TestSendPlayerInputCommandWaitsForAcknowledgement(t *testing.T) {
	resetCommandStateForTest(t, 10)
	primarySession.commands.pending = "/test"

	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := primarySession.commands.number, uint32(11); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn); cmd != 11 {
		t.Fatalf("packet command=%d, want 11", cmd)
	}
	if cmd := extractCommandText(t, conn); cmd != "/test" {
		t.Fatalf("packet command text=%q, want /test", cmd)
	}
	if primarySession.commands.pending != "/test" || primarySession.commands.pendingID != 11 || !primarySession.commands.pendingSent {
		t.Fatalf("pending state = %q id=%d sent=%v", primarySession.commands.pending, primarySession.commands.pendingID, primarySession.commands.pendingSent)
	}

	conn.Reset()
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput waiting for ack: %v", err)
	}
	if cmd := extractCommand(t, conn); cmd != 11 {
		t.Fatalf("waiting packet command=%d, want 11", cmd)
	}
	if cmd := extractCommandText(t, conn); cmd != "" {
		t.Fatalf("waiting packet repeated command %q before a negative ack", cmd)
	}
}

func TestSendPlayerInputAcknowledgementAdvancesFIFO(t *testing.T) {
	resetCommandStateForTest(t, 1)
	primarySession.commands.pending = "/say"
	primarySession.commands.queue = []queuedCommand{{text: "/wave"}}

	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	gotCmd := extractCommandText(t, conn)
	if gotCmd != "/say" {
		t.Fatalf("sent %q want %q", gotCmd, "/say")
	}
	if primarySession.commands.pending != "/say" {
		t.Fatalf("pendingCommand %q want %q before ack", primarySession.commands.pending, "/say")
	}
	if len(primarySession.commands.queue) != 1 || primarySession.commands.queue[0].text != "/wave" {
		t.Fatalf("commandQueue before ack: %v", primarySession.commands.queue)
	}

	acknowledgeCommand(2, 1)
	if primarySession.commands.pending != "/wave" || primarySession.commands.pendingID != 0 || primarySession.commands.pendingSent {
		t.Fatalf("pending after ack = %q id=%d sent=%v", primarySession.commands.pending, primarySession.commands.pendingID, primarySession.commands.pendingSent)
	}
	if len(primarySession.commands.queue) != 0 {
		t.Fatalf("commandQueue after ack: %v", primarySession.commands.queue)
	}
	conn.Reset()
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput second command: %v", err)
	}
	if got := extractCommand(t, conn); got != 3 {
		t.Fatalf("second command number=%d, want 3", got)
	}
	if got := extractCommandText(t, conn); got != "/wave" {
		t.Fatalf("second command=%q, want /wave", got)
	}
}

func TestSendPlayerInputRetriesSameCommandAfterNegativeAck(t *testing.T) {
	resetCommandStateForTest(t, 41)
	enqueueCommand("/equip 123")
	first := &bufConn{}
	if err := sendPlayerInput(first, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	if got := extractCommand(t, first); got != 42 {
		t.Fatalf("first command number=%d, want 42", got)
	}

	acknowledgeCommand(41, 1)
	retry := &bufConn{}
	if err := sendPlayerInput(retry, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	if got := extractCommand(t, retry); got != 42 {
		t.Fatalf("retry command number=%d, want 42", got)
	}
	if got := extractCommandText(t, retry); got != "/equip 123" {
		t.Fatalf("retry command=%q", got)
	}
}

func TestDelayedMatchingAckCompletesCommandBeforeRetry(t *testing.T) {
	resetCommandStateForTest(t, 41)
	enqueueCommand("/equip 123")
	if err := sendPlayerInput(&bufConn{}, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	acknowledgeCommand(41, 1)
	if primarySession.commands.pendingSent {
		t.Fatal("negative acknowledgement did not enable retry")
	}
	acknowledgeCommand(42, 1)
	if primarySession.commands.pending != "" || primarySession.commands.pendingID != 0 || primarySession.commands.pendingSent {
		t.Fatalf("delayed matching ack left pending state %q id=%d sent=%v", primarySession.commands.pending, primarySession.commands.pendingID, primarySession.commands.pendingSent)
	}
}

func TestSendPlayerInputCommandNumberWrapsWithoutZero(t *testing.T) {
	resetCommandStateForTest(t, 255)
	enqueueCommand("/wave")
	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	if got := extractCommand(t, conn); got != 1 {
		t.Fatalf("wrapped command number=%d, want 1", got)
	}
}

func TestSendPlayerInputWriteErrorKeepsCommandForRetry(t *testing.T) {
	resetCommandStateForTest(t, 9)
	enqueueCommand("/equip 123")
	if err := sendPlayerInput(&writeErrorConn{}, 0, 0, false, false); err == nil {
		t.Fatal("sendPlayerInput succeeded with a failing connection")
	}
	if primarySession.commands.pending != "/equip 123" || primarySession.commands.pendingID != 10 || primarySession.commands.pendingSent {
		t.Fatalf("failed send state = %q id=%d sent=%v", primarySession.commands.pending, primarySession.commands.pendingID, primarySession.commands.pendingSent)
	}

	retry := &bufConn{}
	if err := sendPlayerInput(retry, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	if got := extractCommand(t, retry); got != 10 {
		t.Fatalf("retry command number=%d, want 10", got)
	}
	if got := extractCommandText(t, retry); got != "/equip 123" {
		t.Fatalf("retry command=%q", got)
	}
}

func TestSendPlayerInputEncodesCommandAndChatAsMacRoman(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []byte
	}{
		{name: "chat", text: "café ☺", want: append([]byte{'c', 'a', 'f', 0x8e, ' '}, []byte(`\u263A`)...)},
		{name: "command", text: "/think Méme 🚀 \\u263A", want: append([]byte{'/', 't', 'h', 'i', 'n', 'k', ' ', 'M', 0x8e, 'm', 'e', ' '}, []byte(`:rocket: \u263A`)...)},
		{name: "pasted emoji", text: "Hi 😄 🚀", want: []byte(`Hi :smile: :rocket:`)},
		{name: "emoji chat", text: "Hi :smile: :thumbs_up:", want: []byte(`Hi :smile: :thumbs_up:`)},
		{name: "emoji command", text: "/think :woman_technologist::heart: :unknown:", want: []byte(`/think :woman_technologist::heart: :unknown:`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetCommandStateForTest(t, 1)
			primarySession.commands.pending = test.text
			conn := &bufConn{}
			if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
				t.Fatalf("sendPlayerInput: %v", err)
			}
			data := conn.Bytes()
			size := int(binary.BigEndian.Uint16(data[:2]))
			packet := data[2 : 2+size]
			command := packet[20:]
			nul := bytes.IndexByte(command, 0)
			if nul < 0 {
				t.Fatal("missing command terminator")
			}
			if got := command[:nul]; !bytes.Equal(got, test.want) {
				t.Fatalf("command bytes = % x, want % x", got, test.want)
			}
		})
	}
}

func TestSendPlayerInputTruncatesCommandToClassicLimit(t *testing.T) {
	resetCommandStateForTest(t, 1)
	primarySession.commands.pending = strings.Repeat("x", maxPlayerCommandBytes+100)
	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got := len(extractCommandText(t, conn)); got != maxPlayerCommandBytes {
		t.Fatalf("encoded command length = %d, want %d", got, maxPlayerCommandBytes)
	}
}

func TestSendPlayerInputTruncatesAtUnicodeRuneBoundary(t *testing.T) {
	for _, remaining := range []int{0, 1, 5, 9, 10, 11} {
		t.Run(fmt.Sprint(remaining), func(t *testing.T) {
			resetCommandStateForTest(t, 1)
			prefix := strings.Repeat("x", maxPlayerCommandBytes-remaining)
			primarySession.commands.pending = prefix + "𐀀" + strings.Repeat("y", 20)
			conn := &bufConn{}
			if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
				t.Fatal(err)
			}
			want := prefix
			if remaining >= 10 {
				want += `\U00010000` + strings.Repeat("y", remaining-10)
			}
			if got := extractCommandText(t, conn); got != want {
				t.Fatalf("command = %q, want %q", got, want)
			}
		})
	}
}

func TestCommandReplyRequiresMatchingAcknowledgement(t *testing.T) {
	resetCommandStateForTest(t, 10)
	primarySession.timing.replyMu.Lock()
	oldReply := primarySession.timing.reply
	primarySession.timing.reply = 0
	primarySession.timing.replyMu.Unlock()
	primarySession.timing.cadenceMu.Lock()
	oldFrameJitter := primarySession.timing.jitter
	primarySession.timing.jitter = 0
	primarySession.timing.cadenceMu.Unlock()
	t.Cleanup(func() {
		primarySession.timing.replyMu.Lock()
		primarySession.timing.reply = oldReply
		primarySession.timing.replyMu.Unlock()
		primarySession.timing.cadenceMu.Lock()
		primarySession.timing.jitter = oldFrameJitter
		primarySession.timing.cadenceMu.Unlock()
	})

	primarySession.commands.pending = "/test"
	if err := sendPlayerInput(&bufConn{}, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	primarySession.commands.mu.Lock()
	originalSentAt := time.Now().Add(-80 * time.Millisecond)
	primarySession.commands.pendingSentAt = originalSentAt
	primarySession.commands.mu.Unlock()

	acknowledgeCommand(10, 1)
	if reply, jitter := networkTimingSnapshot(); reply != 0 || jitter != 0 {
		t.Fatalf("unmatched ack recorded reply %v jitter %v", reply, jitter)
	}
	if err := sendPlayerInput(&bufConn{}, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	primarySession.commands.mu.Lock()
	retrySentAt := primarySession.commands.pendingSentAt
	primarySession.commands.mu.Unlock()
	if !retrySentAt.Equal(originalSentAt) {
		t.Fatalf("retry replaced first-send timestamp: %v != %v", retrySentAt, originalSentAt)
	}

	acknowledgeCommandAt(11, 1, originalSentAt.Add(80*time.Millisecond))
	reply, _ := networkTimingSnapshot()
	if reply != 80*time.Millisecond {
		t.Fatalf("matching ack reply = %v, want socket-arrival interval 80ms", reply)
	}
}

func TestWarmupCommandReplyDoesNotTunePNA(t *testing.T) {
	resetCommandStateForTest(t, 10)
	originalEnabled := gs.AltNetMode
	gs.AltNetMode = true
	primarySession.timing.replyMu.Lock()
	originalReply := primarySession.timing.reply
	primarySession.timing.reply = 0
	primarySession.timing.replyMu.Unlock()
	primarySession.timing.controllerMu.Lock()
	originalController := primarySession.timing.controller
	primarySession.timing.controller = pnaControllerState{initialized: true, lead: 50 * time.Millisecond, consecutiveHits: 2}
	primarySession.timing.controllerMu.Unlock()
	t.Cleanup(func() {
		gs.AltNetMode = originalEnabled
		primarySession.timing.replyMu.Lock()
		primarySession.timing.reply = originalReply
		primarySession.timing.replyMu.Unlock()
		primarySession.timing.controllerMu.Lock()
		primarySession.timing.controller = originalController
		primarySession.timing.controllerMu.Unlock()
	})

	sentAt := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	primarySession.commands.pending = "/test"
	primarySession.commands.pendingID = 11
	primarySession.commands.pendingSent = true
	primarySession.commands.pendingSentAt = sentAt
	primarySession.commands.pendingSentFrame = 10
	primarySession.commands.pendingSentPhase = 0
	primarySession.commands.pendingSentInterval = 200 * time.Millisecond
	primarySession.commands.pendingSentPredictively = false
	acknowledgeCommandAt(11, 11, sentAt.Add(200*time.Millisecond))

	reply, _ := networkTimingSnapshot()
	if reply != 200*time.Millisecond {
		t.Fatalf("warmup command reply = %v, want 200ms", reply)
	}
	primarySession.timing.controllerMu.Lock()
	controller := primarySession.timing.controller
	primarySession.timing.controllerMu.Unlock()
	if controller.lead != 50*time.Millisecond || controller.consecutiveHits != 2 {
		t.Fatalf("warmup command changed PNA controller: %+v", controller)
	}
}

func TestResetLiveNetworkSessionUsesClassicBootstrap(t *testing.T) {
	resetCommandStateForTest(t, primarySession.commands.number)
	oldAck, oldResend := primarySession.frames.ack, primarySession.frames.resend
	oldLastAck, oldNumFrames, oldLostFrames := primarySession.frames.lastAck, primarySession.frames.received, primarySession.frames.lost
	oldFrameBuckets, oldLostBuckets, oldBucketTimes := primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes
	inputMu.Lock()
	oldInputQueue := append([]inputState(nil), inputQueue...)
	oldKeyStopFrames := keyStopFrames
	inputMu.Unlock()
	primarySession.timing.replyMu.Lock()
	oldReply := primarySession.timing.reply
	primarySession.timing.replyMu.Unlock()
	primarySession.timing.cadenceMu.Lock()
	oldLastFrameTime, oldFrameInterval, oldFrameJitter := primarySession.timing.lastFrameAt, primarySession.timing.interval, primarySession.timing.jitter
	oldLastTimingFrame := primarySession.timing.lastFrame
	oldTimingSamples := append([]timedDurationSample(nil), primarySession.timing.samples...)
	oldUpdatesPerSecond := primarySession.timing.updatesPerSecond
	primarySession.timing.cadenceMu.Unlock()
	primarySession.timing.controllerMu.Lock()
	oldPNAController := primarySession.timing.controller
	primarySession.timing.controllerMu.Unlock()
	primarySession.timing.fallbackMu.Lock()
	oldPNAFallback := primarySession.timing.fallback
	primarySession.timing.fallbackMu.Unlock()
	t.Cleanup(func() {
		primarySession.frames.ack, primarySession.frames.resend = oldAck, oldResend
		primarySession.frames.lastAck, primarySession.frames.received, primarySession.frames.lost = oldLastAck, oldNumFrames, oldLostFrames
		primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes = oldFrameBuckets, oldLostBuckets, oldBucketTimes
		inputMu.Lock()
		inputQueue = oldInputQueue
		keyStopFrames = oldKeyStopFrames
		inputMu.Unlock()
		primarySession.timing.replyMu.Lock()
		primarySession.timing.reply = oldReply
		primarySession.timing.replyMu.Unlock()
		primarySession.timing.cadenceMu.Lock()
		primarySession.timing.lastFrameAt, primarySession.timing.interval, primarySession.timing.jitter = oldLastFrameTime, oldFrameInterval, oldFrameJitter
		primarySession.timing.lastFrame = oldLastTimingFrame
		primarySession.timing.samples = oldTimingSamples
		primarySession.timing.updatesPerSecond = oldUpdatesPerSecond
		primarySession.timing.cadenceMu.Unlock()
		primarySession.timing.controllerMu.Lock()
		primarySession.timing.controller = oldPNAController
		primarySession.timing.controllerMu.Unlock()
		primarySession.timing.fallbackMu.Lock()
		primarySession.timing.fallback = oldPNAFallback
		primarySession.timing.fallbackMu.Unlock()
	})

	primarySession.frames.ack, primarySession.frames.resend = 42, 43
	primarySession.frames.lastAck, primarySession.frames.received, primarySession.frames.lost = 42, 20, 5
	inputMu.Lock()
	inputQueue = []inputState{{mouseX: 1}}
	keyStopFrames = 2
	inputMu.Unlock()
	primarySession.timing.replyMu.Lock()
	primarySession.timing.reply = 50 * time.Millisecond
	primarySession.timing.replyMu.Unlock()
	primarySession.timing.cadenceMu.Lock()
	primarySession.timing.lastFrameAt = time.Now()
	primarySession.timing.lastFrame = 42
	primarySession.timing.interval = 200 * time.Millisecond
	primarySession.timing.samples = []timedDurationSample{{at: time.Now(), value: 200 * time.Millisecond}}
	primarySession.timing.jitter = 10 * time.Millisecond
	primarySession.timing.updatesPerSecond = 5
	primarySession.timing.cadenceMu.Unlock()
	primarySession.timing.controllerMu.Lock()
	primarySession.timing.controller = pnaControllerState{initialized: true, lead: 50 * time.Millisecond}
	primarySession.timing.controllerMu.Unlock()
	primarySession.timing.fallbackMu.Lock()
	primarySession.timing.fallback = pnaFallbackState{activeUntil: time.Now().Add(time.Minute), reason: "recent packet loss"}
	primarySession.timing.fallbackMu.Unlock()
	primarySession.commands.mu.Lock()
	primarySession.commands.pending = "/test"
	primarySession.commands.pendingID = 9
	primarySession.commands.pendingSent = true
	primarySession.commands.pendingSentAt = time.Now()
	primarySession.commands.pendingSentFrame = 42
	primarySession.commands.pendingSentPhase = 150 * time.Millisecond
	primarySession.commands.pendingSentInterval = 200 * time.Millisecond
	primarySession.commands.pendingSentPredictively = true
	primarySession.commands.mu.Unlock()
	select {
	case primarySession.timing.wake <- struct{}{}:
	default:
	}

	resetLiveNetworkSession()
	if primarySession.frames.ack != 0 || primarySession.frames.resend != -1 {
		t.Fatalf("bootstrap ack/resend = %d/%d, want 0/-1", primarySession.frames.ack, primarySession.frames.resend)
	}
	firstInput := &bufConn{}
	if err := sendPlayerInput(firstInput, 0, 0, false, false); err != nil {
		t.Fatalf("first session input: %v", err)
	}
	framed := firstInput.Bytes()
	payloadSize := int(binary.BigEndian.Uint16(framed[:2]))
	payload := framed[2 : 2+payloadSize]
	if got := int32(binary.BigEndian.Uint32(payload[12:16])); got != -1 {
		t.Fatalf("first input resend marker = %d, want -1", got)
	}
	if primarySession.frames.lastAck != 0 || primarySession.frames.received != 0 || primarySession.frames.lost != 0 {
		t.Fatalf("frame statistics not reset: last=%d total=%d lost=%d", primarySession.frames.lastAck, primarySession.frames.received, primarySession.frames.lost)
	}
	if !commandQueueIsIdle() {
		t.Fatal("command queue survived session reset")
	}
	inputMu.Lock()
	queued, stops := len(inputQueue), keyStopFrames
	inputMu.Unlock()
	if queued != 0 || stops != 0 {
		t.Fatalf("input state not reset: queued=%d stops=%d", queued, stops)
	}
	if reply, jitter := networkTimingSnapshot(); reply != 0 || jitter != 0 {
		t.Fatalf("network timing state not reset: %v/%v", reply, jitter)
	}
	primarySession.timing.cadenceMu.Lock()
	newLastFrameTime, newUpdatesPerSecond := primarySession.timing.lastFrameAt, primarySession.timing.updatesPerSecond
	timingSamples := len(primarySession.timing.samples)
	newLastTimingFrame := primarySession.timing.lastFrame
	primarySession.timing.cadenceMu.Unlock()
	if !newLastFrameTime.IsZero() || newUpdatesPerSecond != 0 || timingSamples != 0 || newLastTimingFrame != 0 {
		t.Fatalf("PNA measurements survived session reset: frame=%v id=%d rate=%v timingSamples=%d", newLastFrameTime, newLastTimingFrame, newUpdatesPerSecond, timingSamples)
	}
	primarySession.timing.controllerMu.Lock()
	controllerReset := primarySession.timing.controller == (pnaControllerState{})
	primarySession.timing.controllerMu.Unlock()
	if !controllerReset {
		t.Fatal("PNA controller survived session reset")
	}
	if usePNA, reason := pnaTimingStatus(0, time.Now()); !usePNA || reason != "" {
		t.Fatalf("new session PNA status = use:%v reason:%q, want immediate-learning mode", usePNA, reason)
	}
	select {
	case <-primarySession.timing.wake:
		t.Fatal("stale frame notification survived session reset")
	default:
	}
}

func TestReadUDPMessageExactDatagram(t *testing.T) {
	msg := []byte{0x00, 0x03, 0xde, 0xad}
	datagram := append([]byte{0x00, byte(len(msg))}, msg...)
	conn := &packetConn{packets: [][]byte{datagram}}

	got, err := readUDPMessage(conn)
	if err != nil {
		t.Fatalf("readUDPMessage: %v", err)
	}
	if !bytes.Equal(got, msg) {
		t.Fatalf("got %x want %x", got, msg)
	}
	if conn.idx != 1 {
		t.Fatalf("expected 1 read, got %d", conn.idx)
	}
}

func TestReadUDPMessageRejectsMalformedDatagrams(t *testing.T) {
	tests := []struct {
		name     string
		datagram []byte
	}{
		{name: "missing length", datagram: []byte{0}},
		{name: "payload below tag size", datagram: []byte{0, 1, 0}},
		{name: "declared longer", datagram: []byte{0, 4, 0, 2}},
		{name: "trailing bytes", datagram: []byte{0, 2, 0, 2, 0xff}},
		{name: "payload over protocol maximum", datagram: []byte{byte((maxProtocolMessageSize + 1) >> 8), byte((maxProtocolMessageSize + 1) & 0xff), 0, 2}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := &packetConn{packets: [][]byte{test.datagram}}
			if _, err := readUDPMessage(conn); !errors.Is(err, errMalformedUDPDatagram) {
				t.Fatalf("readUDPMessage error = %v, want malformed datagram", err)
			}
			if conn.idx != 1 {
				t.Fatalf("datagram consumed with %d reads, want 1", conn.idx)
			}
		})
	}
}

func TestReadUDPMessageRecoversAfterMalformedDatagram(t *testing.T) {
	want := []byte{0, 2, 0xaa}
	valid := append([]byte{0, byte(len(want))}, want...)
	conn := &packetConn{packets: [][]byte{{0, 4, 0, 2}, valid}}
	if _, err := readUDPMessage(conn); !errors.Is(err, errMalformedUDPDatagram) {
		t.Fatalf("malformed datagram error = %v", err)
	}
	got, err := readUDPMessage(conn)
	if err != nil {
		t.Fatalf("valid datagram after malformed one: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("valid payload = %x, want %x", got, want)
	}
}

func TestReadTCPMessagePreservesPartialHeaderAndPayload(t *testing.T) {
	want := []byte{0, 2, 0xaa, 0xbb}
	conn := &chunkConn{chunks: [][]byte{{0}, {byte(len(want)), 0}, {2, 0xaa}, {0xbb}}}
	got, err := readTCPMessage(conn)
	if err != nil {
		t.Fatalf("readTCPMessage: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("TCP payload = %x, want %x", got, want)
	}
}

func TestReadTCPMessageWaitsAcrossPartialReadPause(t *testing.T) {
	reader, writer := net.Pipe()
	defer reader.Close()
	want := []byte{0, 2, 0xaa, 0xbb}
	done := make(chan error, 1)
	go func() {
		defer writer.Close()
		if _, err := writer.Write([]byte{0, byte(len(want)), want[0]}); err != nil {
			done <- err
			return
		}
		time.Sleep(20 * time.Millisecond)
		_, err := writer.Write(want[1:])
		done <- err
	}()
	got, err := readTCPMessage(reader)
	if err != nil {
		t.Fatalf("readTCPMessage: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("TCP payload after pause = %x, want %x", got, want)
	}
	if err := <-done; err != nil {
		t.Fatalf("write split TCP message: %v", err)
	}
}

func TestReadTCPMessageRejectsInvalidSize(t *testing.T) {
	for _, size := range []int{0, 1, maxProtocolMessageSize + 1} {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			conn := &chunkConn{chunks: [][]byte{{byte(size >> 8), byte(size)}}}
			if _, err := readTCPMessage(conn); err == nil {
				t.Fatal("readTCPMessage accepted invalid size")
			}
		})
	}
}

func TestSendUDPMessageDoesNotSplitShortWrite(t *testing.T) {
	conn := &shortWriteConn{}
	if err := sendUDPMessage(conn, []byte{0, 3}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("sendUDPMessage error = %v, want short write", err)
	}
	if conn.writes != 1 {
		t.Fatalf("UDP short write used %d writes, want 1", conn.writes)
	}
}

func TestServerMessageDispatcherSerializesWithTCPPriority(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tcpMessages := make(chan incomingServerMessage, 2)
	udpMessages := make(chan incomingServerMessage, 1)
	tcpMessages <- incomingServerMessage{data: []byte("tcp-1")}
	tcpMessages <- incomingServerMessage{data: []byte("tcp-2")}
	udpMessages <- incomingServerMessage{data: []byte("udp-1")}

	got := make(chan string, 3)
	done := make(chan struct{})
	go func() {
		count := 0
		serverMessageDispatchLoopWithHandler(ctx, tcpMessages, udpMessages, func(message incomingServerMessage, reliable bool) {
			transport := "udp:"
			if reliable {
				transport = "tcp:"
			}
			got <- transport + string(message.data)
			count++
			if count == 3 {
				cancel()
			}
		})
		close(done)
	}()

	want := []string{"tcp:tcp-1", "tcp:tcp-2", "udp:udp-1"}
	for i, expected := range want {
		select {
		case actual := <-got:
			if actual != expected {
				t.Fatalf("dispatch %d = %q, want %q", i, actual, expected)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for dispatch %d", i)
		}
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not stop after cancellation")
	}
}

func BenchmarkSendPlayerInputReliableNoAllocs(b *testing.B) {
	oldCommandNum := primarySession.commands.number
	oldPending := primarySession.commands.pending
	oldPendingID := primarySession.commands.pendingID
	oldPendingSent := primarySession.commands.pendingSent
	oldQueue := primarySession.commands.queue
	oldAck := primarySession.frames.ack
	oldResend := primarySession.frames.resend
	oldSentAt := primarySession.commands.pendingSentAt
	oldSentFrame := primarySession.commands.pendingSentFrame
	oldSentPhase := primarySession.commands.pendingSentPhase
	oldSentInterval := primarySession.commands.pendingSentInterval
	oldSentPredictively := primarySession.commands.pendingSentPredictively
	defer func() {
		primarySession.commands.number = oldCommandNum
		primarySession.commands.pending = oldPending
		primarySession.commands.pendingID = oldPendingID
		primarySession.commands.pendingSent = oldPendingSent
		primarySession.commands.queue = oldQueue
		primarySession.frames.ack = oldAck
		primarySession.frames.resend = oldResend
		primarySession.commands.pendingSentAt = oldSentAt
		primarySession.commands.pendingSentFrame = oldSentFrame
		primarySession.commands.pendingSentPhase = oldSentPhase
		primarySession.commands.pendingSentInterval = oldSentInterval
		primarySession.commands.pendingSentPredictively = oldSentPredictively
	}()

	primarySession.commands.number = 1
	primarySession.commands.pending = ""
	primarySession.commands.pendingID = 0
	primarySession.commands.pendingSent = false
	primarySession.commands.queue = nil
	primarySession.frames.ack = 0
	primarySession.frames.resend = 0
	primarySession.commands.resetPendingTimingLocked()

	conn := &bufConn{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.Reset()
		if err := sendPlayerInput(conn, 0, 0, false, true); err != nil {
			b.Fatalf("sendPlayerInput: %v", err)
		}
	}
}

func BenchmarkSendPlayerInputUnreliableNoAllocs(b *testing.B) {
	oldCommandNum := primarySession.commands.number
	oldPending := primarySession.commands.pending
	oldPendingID := primarySession.commands.pendingID
	oldPendingSent := primarySession.commands.pendingSent
	oldQueue := primarySession.commands.queue
	oldAck := primarySession.frames.ack
	oldResend := primarySession.frames.resend
	oldSentAt := primarySession.commands.pendingSentAt
	oldSentFrame := primarySession.commands.pendingSentFrame
	oldSentPhase := primarySession.commands.pendingSentPhase
	oldSentInterval := primarySession.commands.pendingSentInterval
	oldSentPredictively := primarySession.commands.pendingSentPredictively
	defer func() {
		primarySession.commands.number = oldCommandNum
		primarySession.commands.pending = oldPending
		primarySession.commands.pendingID = oldPendingID
		primarySession.commands.pendingSent = oldPendingSent
		primarySession.commands.queue = oldQueue
		primarySession.frames.ack = oldAck
		primarySession.frames.resend = oldResend
		primarySession.commands.pendingSentAt = oldSentAt
		primarySession.commands.pendingSentFrame = oldSentFrame
		primarySession.commands.pendingSentPhase = oldSentPhase
		primarySession.commands.pendingSentInterval = oldSentInterval
		primarySession.commands.pendingSentPredictively = oldSentPredictively
	}()

	primarySession.commands.number = 1
	primarySession.commands.pending = ""
	primarySession.commands.pendingID = 0
	primarySession.commands.pendingSent = false
	primarySession.commands.queue = nil
	primarySession.frames.ack = 0
	primarySession.frames.resend = 0
	primarySession.commands.resetPendingTimingLocked()

	conn := &bufConn{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.Reset()
		if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
			b.Fatalf("sendPlayerInput: %v", err)
		}
	}
}
