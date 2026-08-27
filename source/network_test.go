package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
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
	copy(b, p)
	return len(p), nil
}

func (c *packetConn) Write(b []byte) (int, error)        { return len(b), nil }
func (c *packetConn) Close() error                       { return nil }
func (c *packetConn) LocalAddr() net.Addr                { return dummyAddr{} }
func (c *packetConn) RemoteAddr() net.Addr               { return dummyAddr{} }
func (c *packetConn) SetDeadline(t time.Time) error      { return nil }
func (c *packetConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *packetConn) SetWriteDeadline(t time.Time) error { return nil }

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
	oldQueue := commandQueue
	t.Cleanup(func() {
		commandNum = oldCommandNum
		pendingCommand = oldPending
		pendingCommandID = oldPendingID
		pendingCommandSent = oldPendingSent
		commandQueue = oldQueue
	})
	commandNum = number
	pendingCommand = ""
	pendingCommandID = 0
	pendingCommandSent = false
	commandQueue = nil
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

	acknowledgeCommand(2)
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

	acknowledgeCommand(41)
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

func TestReadUDPMessageFragmented(t *testing.T) {
	udpBuffer = nil
	msg := []byte{0x00, 0x03, 0xde, 0xad, 0xbe, 0xef}
	datagrams := [][]byte{{0x00}, append([]byte{0x06}, msg...)}
	conn := &packetConn{packets: datagrams}

	got, err := readUDPMessage(conn)
	if err != nil {
		t.Fatalf("readUDPMessage: %v", err)
	}
	if !bytes.Equal(got, msg) {
		t.Fatalf("got %x want %x", got, msg)
	}
	if conn.idx != 2 {
		t.Fatalf("expected 2 reads, got %d", conn.idx)
	}
}

func TestReadUDPMessageMultiple(t *testing.T) {
	udpBuffer = nil
	msg1 := []byte{0x00, 0x01}
	msg2 := []byte{0x00, 0x02, 0xff}
	d := append([]byte{0x00, byte(len(msg1))}, msg1...)
	d = append(d, []byte{0x00, byte(len(msg2))}...)
	d = append(d, msg2...)
	conn := &packetConn{packets: [][]byte{d}}

	got1, err := readUDPMessage(conn)
	if err != nil {
		t.Fatalf("readUDPMessage 1: %v", err)
	}
	if !bytes.Equal(got1, msg1) {
		t.Fatalf("msg1 %x want %x", got1, msg1)
	}
	got2, err := readUDPMessage(conn)
	if err != nil {
		t.Fatalf("readUDPMessage 2: %v", err)
	}
	if !bytes.Equal(got2, msg2) {
		t.Fatalf("msg2 %x want %x", got2, msg2)
	}
	if conn.idx != 1 {
		t.Fatalf("expected 1 read, got %d", conn.idx)
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
	oldLast := lastInputSent
	defer func() {
		commandNum = oldCommandNum
		pendingCommand = oldPending
		pendingCommandID = oldPendingID
		pendingCommandSent = oldPendingSent
		commandQueue = oldQueue
		ackFrame = oldAck
		resendFrame = oldResend
		lastInputSent = oldLast
	}()

	commandNum = 1
	pendingCommand = ""
	pendingCommandID = 0
	pendingCommandSent = false
	commandQueue = nil
	ackFrame = 0
	resendFrame = 0
	lastInputSent = time.Time{}

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
	oldLast := lastInputSent
	defer func() {
		commandNum = oldCommandNum
		pendingCommand = oldPending
		pendingCommandID = oldPendingID
		pendingCommandSent = oldPendingSent
		commandQueue = oldQueue
		ackFrame = oldAck
		resendFrame = oldResend
		lastInputSent = oldLast
	}()

	commandNum = 1
	pendingCommand = ""
	pendingCommandID = 0
	pendingCommandSent = false
	commandQueue = nil
	ackFrame = 0
	resendFrame = 0
	lastInputSent = time.Time{}

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
