package demoserver

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestFrameBoundAndCommandRetry(t *testing.T) {
	s := &Server{slots: 16, peers: map[uint32]*player{}}
	online := []*player{}
	for i := 1; i <= 16; i++ {
		p := &player{slot: byte(i), frame: 1}
		s.peers[uint32(i)] = p
		online = append(online, p)
	}
	command := make([]byte, 20)
	binary.BigEndian.PutUint16(command, 3)
	binary.BigEndian.PutUint32(command[16:], 1)
	command = append(command, bytes.Repeat([]byte("x"), 511)...)
	command = append(command, 0)
	s.input(online[0], command)
	s.input(online[0], command)
	for _, p := range online {
		if len(p.messages) != 1 {
			t.Fatal("command retry duplicated delivery")
		}
		packet := s.frame(p, online)
		if len(packet) > maxPacket {
			t.Fatalf("oversized frame: %d", len(packet))
		}
	}
	for _, data := range [][]byte{{}, {0}, {0, 1}, {5, 117}} {
		if _, err := readPacket(bytes.NewReader(data)); err == nil {
			t.Fatalf("accepted malformed frame %v", data)
		}
	}
}
func TestCloseReleasesPendingHandshake(t *testing.T) {
	s, err := Listen("127.0.0.1:0", 2)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(context.Background()) }()
	conn, err := net.Dial("tcp", s.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	id := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err = conn.Read(id); err != nil {
		t.Fatal(err)
	}
	s.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending handshake prevented shutdown")
	}
}
