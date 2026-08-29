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
	oldCommandNum := commandNum
	oldPending := pendingCommand
	oldPendingID := pendingCommandID
	oldPendingSent := pendingCommandSent
	oldPendingSentAt := pendingCommandSentAt
	oldPendingSentFrame := pendingCommandSentFrame
	oldPendingSentPhase := pendingCommandSentPhase
	oldPendingSentInterval := pendingCommandSentInterval
	oldPendingSentPredictively := pendingCommandSentPredictively
	oldQueue := commandQueue
	oldWhoLastCommandFrame := whoLastCommandFrame
	t.Cleanup(func() {
		commandNum = oldCommandNum
		pendingCommand = oldPending
		pendingCommandID = oldPendingID
		pendingCommandSent = oldPendingSent
		pendingCommandSentAt = oldPendingSentAt
		pendingCommandSentFrame = oldPendingSentFrame
		pendingCommandSentPhase = oldPendingSentPhase
		pendingCommandSentInterval = oldPendingSentInterval
		pendingCommandSentPredictively = oldPendingSentPredictively
		commandQueue = oldQueue
		whoLastCommandFrame = oldWhoLastCommandFrame
	})
	commandNum = number
	pendingCommand = ""
	pendingCommandID = 0
	pendingCommandSent = false
	resetPendingCommandTimingLocked()
	commandQueue = nil
	whoLastCommandFrame = -1
}

func TestSendPlayerInputEmptyFramesKeepCommandNum(t *testing.T) {
	resetCommandStateForTest(t, 1)

	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := commandNum, uint32(1); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn); cmd != 1 {
		t.Fatalf("packet command=%d, want 1", cmd)
	}

	conn2 := &bufConn{}
	if err := sendPlayerInput(conn2, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := commandNum, uint32(1); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn2); cmd != 1 {
		t.Fatalf("packet command=%d, want 1", cmd)
	}
}

func TestSendPlayerInputCommandWaitsForAcknowledgement(t *testing.T) {
	resetCommandStateForTest(t, 10)
	pendingCommand = "/test"

	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := commandNum, uint32(11); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn); cmd != 11 {
		t.Fatalf("packet command=%d, want 11", cmd)
	}
	if cmd := extractCommandText(t, conn); cmd != "/test" {
		t.Fatalf("packet command text=%q, want /test", cmd)
	}
	if pendingCommand != "/test" || pendingCommandID != 11 || !pendingCommandSent {
		t.Fatalf("pending state = %q id=%d sent=%v", pendingCommand, pendingCommandID, pendingCommandSent)
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
	pendingCommand = "/say"
	commandQueue = []string{"/wave"}

	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	gotCmd := extractCommandText(t, conn)
	if gotCmd != "/say" {
		t.Fatalf("sent %q want %q", gotCmd, "/say")
	}
	if pendingCommand != "/say" {
		t.Fatalf("pendingCommand %q want %q before ack", pendingCommand, "/say")
	}
	if len(commandQueue) != 1 || commandQueue[0] != "/wave" {
		t.Fatalf("commandQueue before ack: %v", commandQueue)
	}

	acknowledgeCommand(2, 1)
	if pendingCommand != "/wave" || pendingCommandID != 0 || pendingCommandSent {
		t.Fatalf("pending after ack = %q id=%d sent=%v", pendingCommand, pendingCommandID, pendingCommandSent)
	}
	if len(commandQueue) != 0 {
		t.Fatalf("commandQueue after ack: %v", commandQueue)
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
	if pendingCommandSent {
		t.Fatal("negative acknowledgement did not enable retry")
	}
	acknowledgeCommand(42, 1)
	if pendingCommand != "" || pendingCommandID != 0 || pendingCommandSent {
		t.Fatalf("delayed matching ack left pending state %q id=%d sent=%v", pendingCommand, pendingCommandID, pendingCommandSent)
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
	if pendingCommand != "/equip 123" || pendingCommandID != 10 || pendingCommandSent {
		t.Fatalf("failed send state = %q id=%d sent=%v", pendingCommand, pendingCommandID, pendingCommandSent)
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
		{name: "command", text: "/think Méme 🚀 \\u263A", want: append([]byte{'/', 't', 'h', 'i', 'n', 'k', ' ', 'M', 0x8e, 'm', 'e', ' '}, []byte(`\U0001F680 \u263A`)...)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetCommandStateForTest(t, 1)
			pendingCommand = test.text
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
	pendingCommand = strings.Repeat("x", maxPlayerCommandBytes+100)
	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got := len(extractCommandText(t, conn)); got != maxPlayerCommandBytes {
		t.Fatalf("encoded command length = %d, want %d", got, maxPlayerCommandBytes)
	}
}

func TestCommandReplyRequiresMatchingAcknowledgement(t *testing.T) {
	resetCommandStateForTest(t, 10)
	commandReplyMu.Lock()
	oldReply := commandReplyTime
	commandReplyTime = 0
	commandReplyMu.Unlock()
	frameMu.Lock()
	oldFrameJitter := serverFrameJitter
	serverFrameJitter = 0
	frameMu.Unlock()
	t.Cleanup(func() {
		commandReplyMu.Lock()
		commandReplyTime = oldReply
		commandReplyMu.Unlock()
		frameMu.Lock()
		serverFrameJitter = oldFrameJitter
		frameMu.Unlock()
	})

	pendingCommand = "/test"
	if err := sendPlayerInput(&bufConn{}, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	commandMu.Lock()
	originalSentAt := time.Now().Add(-80 * time.Millisecond)
	pendingCommandSentAt = originalSentAt
	commandMu.Unlock()

	acknowledgeCommand(10, 1)
	if reply, jitter := networkTimingSnapshot(); reply != 0 || jitter != 0 {
		t.Fatalf("unmatched ack recorded reply %v jitter %v", reply, jitter)
	}
	if err := sendPlayerInput(&bufConn{}, 0, 0, false, false); err != nil {
		t.Fatal(err)
	}
	commandMu.Lock()
	retrySentAt := pendingCommandSentAt
	commandMu.Unlock()
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
	commandReplyMu.Lock()
	originalReply := commandReplyTime
	commandReplyTime = 0
	commandReplyMu.Unlock()
	pnaControllerMu.Lock()
	originalController := pnaController
	pnaController = pnaControllerState{initialized: true, lead: 50 * time.Millisecond, consecutiveHits: 2}
	pnaControllerMu.Unlock()
	t.Cleanup(func() {
		gs.AltNetMode = originalEnabled
		commandReplyMu.Lock()
		commandReplyTime = originalReply
		commandReplyMu.Unlock()
		pnaControllerMu.Lock()
		pnaController = originalController
		pnaControllerMu.Unlock()
	})

	sentAt := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	pendingCommand = "/test"
	pendingCommandID = 11
	pendingCommandSent = true
	pendingCommandSentAt = sentAt
	pendingCommandSentFrame = 10
	pendingCommandSentPhase = 0
	pendingCommandSentInterval = 200 * time.Millisecond
	pendingCommandSentPredictively = false
	acknowledgeCommandAt(11, 11, sentAt.Add(200*time.Millisecond))

	reply, _ := networkTimingSnapshot()
	if reply != 200*time.Millisecond {
		t.Fatalf("warmup command reply = %v, want 200ms", reply)
	}
	pnaControllerMu.Lock()
	controller := pnaController
	pnaControllerMu.Unlock()
	if controller.lead != 50*time.Millisecond || controller.consecutiveHits != 2 {
		t.Fatalf("warmup command changed PNA controller: %+v", controller)
	}
}

func TestResetLiveNetworkSessionUsesClassicBootstrap(t *testing.T) {
	resetCommandStateForTest(t, commandNum)
	oldAck, oldResend := ackFrame, resendFrame
	oldLastAck, oldNumFrames, oldLostFrames := lastAckFrame, numFrames, lostFrames
	oldFrameBuckets, oldLostBuckets, oldBucketTimes := frameBuckets, lostBuckets, bucketTimes
	inputMu.Lock()
	oldInputQueue := append([]inputState(nil), inputQueue...)
	oldKeyStopFrames := keyStopFrames
	inputMu.Unlock()
	commandReplyMu.Lock()
	oldReply := commandReplyTime
	commandReplyMu.Unlock()
	frameMu.Lock()
	oldLastFrameTime, oldFrameInterval, oldFrameJitter := lastFrameTime, frameInterval, serverFrameJitter
	oldLastTimingFrame := lastTimingFrame
	oldTimingSamples := append([]timedDurationSample(nil), frameTimingSamples...)
	oldUpdatesPerSecond := serverUpdatesPerSecond
	frameMu.Unlock()
	pnaControllerMu.Lock()
	oldPNAController := pnaController
	pnaControllerMu.Unlock()
	pnaFallbackMu.Lock()
	oldPNAFallback := pnaFallback
	pnaFallbackMu.Unlock()
	t.Cleanup(func() {
		ackFrame, resendFrame = oldAck, oldResend
		lastAckFrame, numFrames, lostFrames = oldLastAck, oldNumFrames, oldLostFrames
		frameBuckets, lostBuckets, bucketTimes = oldFrameBuckets, oldLostBuckets, oldBucketTimes
		inputMu.Lock()
		inputQueue = oldInputQueue
		keyStopFrames = oldKeyStopFrames
		inputMu.Unlock()
		commandReplyMu.Lock()
		commandReplyTime = oldReply
		commandReplyMu.Unlock()
		frameMu.Lock()
		lastFrameTime, frameInterval, serverFrameJitter = oldLastFrameTime, oldFrameInterval, oldFrameJitter
		lastTimingFrame = oldLastTimingFrame
		frameTimingSamples = oldTimingSamples
		serverUpdatesPerSecond = oldUpdatesPerSecond
		frameMu.Unlock()
		pnaControllerMu.Lock()
		pnaController = oldPNAController
		pnaControllerMu.Unlock()
		pnaFallbackMu.Lock()
		pnaFallback = oldPNAFallback
		pnaFallbackMu.Unlock()
	})

	ackFrame, resendFrame = 42, 43
	lastAckFrame, numFrames, lostFrames = 42, 20, 5
	inputMu.Lock()
	inputQueue = []inputState{{mouseX: 1}}
	keyStopFrames = 2
	inputMu.Unlock()
	commandReplyMu.Lock()
	commandReplyTime = 50 * time.Millisecond
	commandReplyMu.Unlock()
	frameMu.Lock()
	lastFrameTime = time.Now()
	lastTimingFrame = 42
	frameInterval = 200 * time.Millisecond
	frameTimingSamples = []timedDurationSample{{at: time.Now(), value: 200 * time.Millisecond}}
	serverFrameJitter = 10 * time.Millisecond
	serverUpdatesPerSecond = 5
	frameMu.Unlock()
	pnaControllerMu.Lock()
	pnaController = pnaControllerState{initialized: true, lead: 50 * time.Millisecond}
	pnaControllerMu.Unlock()
	pnaFallbackMu.Lock()
	pnaFallback = pnaFallbackState{activeUntil: time.Now().Add(time.Minute), reason: "recent packet loss"}
	pnaFallbackMu.Unlock()
	commandMu.Lock()
	pendingCommand = "/test"
	pendingCommandID = 9
	pendingCommandSent = true
	pendingCommandSentAt = time.Now()
	pendingCommandSentFrame = 42
	pendingCommandSentPhase = 150 * time.Millisecond
	pendingCommandSentInterval = 200 * time.Millisecond
	pendingCommandSentPredictively = true
	commandMu.Unlock()
	select {
	case frameCh <- struct{}{}:
	default:
	}

	resetLiveNetworkSession()
	if ackFrame != 0 || resendFrame != -1 {
		t.Fatalf("bootstrap ack/resend = %d/%d, want 0/-1", ackFrame, resendFrame)
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
	if lastAckFrame != 0 || numFrames != 0 || lostFrames != 0 {
		t.Fatalf("frame statistics not reset: last=%d total=%d lost=%d", lastAckFrame, numFrames, lostFrames)
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
	frameMu.Lock()
	newLastFrameTime, newUpdatesPerSecond := lastFrameTime, serverUpdatesPerSecond
	timingSamples := len(frameTimingSamples)
	newLastTimingFrame := lastTimingFrame
	frameMu.Unlock()
	if !newLastFrameTime.IsZero() || newUpdatesPerSecond != 0 || timingSamples != 0 || newLastTimingFrame != 0 {
		t.Fatalf("PNA measurements survived session reset: frame=%v id=%d rate=%v timingSamples=%d", newLastFrameTime, newLastTimingFrame, newUpdatesPerSecond, timingSamples)
	}
	pnaControllerMu.Lock()
	controllerReset := pnaController == (pnaControllerState{})
	pnaControllerMu.Unlock()
	if !controllerReset {
		t.Fatal("PNA controller survived session reset")
	}
	if usePNA, reason := pnaTimingStatus(0, time.Now()); !usePNA || reason != "" {
		t.Fatalf("new session PNA status = use:%v reason:%q, want immediate-learning mode", usePNA, reason)
	}
	select {
	case <-frameCh:
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
	oldCommandNum := commandNum
	oldPending := pendingCommand
	oldPendingID := pendingCommandID
	oldPendingSent := pendingCommandSent
	oldQueue := commandQueue
	oldAck := ackFrame
	oldResend := resendFrame
	oldSentAt := pendingCommandSentAt
	oldSentFrame := pendingCommandSentFrame
	oldSentPhase := pendingCommandSentPhase
	oldSentInterval := pendingCommandSentInterval
	oldSentPredictively := pendingCommandSentPredictively
	defer func() {
		commandNum = oldCommandNum
		pendingCommand = oldPending
		pendingCommandID = oldPendingID
		pendingCommandSent = oldPendingSent
		commandQueue = oldQueue
		ackFrame = oldAck
		resendFrame = oldResend
		pendingCommandSentAt = oldSentAt
		pendingCommandSentFrame = oldSentFrame
		pendingCommandSentPhase = oldSentPhase
		pendingCommandSentInterval = oldSentInterval
		pendingCommandSentPredictively = oldSentPredictively
	}()

	commandNum = 1
	pendingCommand = ""
	pendingCommandID = 0
	pendingCommandSent = false
	commandQueue = nil
	ackFrame = 0
	resendFrame = 0
	resetPendingCommandTimingLocked()

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
	oldCommandNum := commandNum
	oldPending := pendingCommand
	oldPendingID := pendingCommandID
	oldPendingSent := pendingCommandSent
	oldQueue := commandQueue
	oldAck := ackFrame
	oldResend := resendFrame
	oldSentAt := pendingCommandSentAt
	oldSentFrame := pendingCommandSentFrame
	oldSentPhase := pendingCommandSentPhase
	oldSentInterval := pendingCommandSentInterval
	oldSentPredictively := pendingCommandSentPredictively
	defer func() {
		commandNum = oldCommandNum
		pendingCommand = oldPending
		pendingCommandID = oldPendingID
		pendingCommandSent = oldPendingSent
		commandQueue = oldQueue
		ackFrame = oldAck
		resendFrame = oldResend
		pendingCommandSentAt = oldSentAt
		pendingCommandSentFrame = oldSentFrame
		pendingCommandSentPhase = oldSentPhase
		pendingCommandSentInterval = oldSentInterval
		pendingCommandSentPredictively = oldSentPredictively
	}()

	commandNum = 1
	pendingCommand = ""
	pendingCommandID = 0
	pendingCommandSent = false
	commandQueue = nil
	ackFrame = 0
	resendFrame = 0
	resetPendingCommandTimingLocked()

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
