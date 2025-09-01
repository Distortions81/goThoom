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

func TestSendPlayerInputCommandNumIncrements(t *testing.T) {
	// Preserve globals used by sendPlayerInput.
	oldCommandNum := commandNum
	oldPending := pendingCommand
	defer func() {
		commandNum = oldCommandNum
		pendingCommand = oldPending
	}()

	commandNum = 1
	pendingCommand = ""

	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := commandNum, uint32(2); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn); cmd != 1 {
		t.Fatalf("packet command=%d, want 1", cmd)
	}

	conn2 := &bufConn{}
	if err := sendPlayerInput(conn2, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := commandNum, uint32(3); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn2); cmd != 2 {
		t.Fatalf("packet command=%d, want 2", cmd)
	}
}

func TestSendPlayerInputCommandNumIncrementsWithCommand(t *testing.T) {
	oldCommandNum := commandNum
	oldPending := pendingCommand
	defer func() {
		commandNum = oldCommandNum
		pendingCommand = oldPending
	}()

	commandNum = 10
	pendingCommand = "/test"

	conn := &bufConn{}
	if err := sendPlayerInput(conn, 0, 0, false, false); err != nil {
		t.Fatalf("sendPlayerInput: %v", err)
	}
	if got, want := commandNum, uint32(11); got != want {
		t.Fatalf("commandNum=%d, want %d", got, want)
	}
	if cmd := extractCommand(t, conn); cmd != 10 {
		t.Fatalf("packet command=%d, want 10", cmd)
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
