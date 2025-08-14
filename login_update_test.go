package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

// fakeServer emulates the minimal Clan Lord server behavior needed
// for testing the auto-update login flow.
type fakeServer struct {
	ln    net.Listener
	udp   *net.UDPConn
	port  int
	mu    sync.Mutex
	tries int
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("resolve udp: %v", err)
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	fs := &fakeServer{ln: ln, udp: udpConn, port: port}
	go fs.serve()
	return fs
}

func (s *fakeServer) addr() string { return fmt.Sprintf("127.0.0.1:%d", s.port) }

func (s *fakeServer) close() {
	s.ln.Close()
	s.udp.Close()
}

func (s *fakeServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.tries++
		attempt := s.tries
		s.mu.Unlock()
		go s.handle(conn, attempt)
	}
}

func (s *fakeServer) handle(conn net.Conn, attempt int) {
	defer conn.Close()
	id := []byte{1, 2, 3, 4}
	if _, err := conn.Write(id); err != nil {
		return
	}
	buf := make([]byte, 6)
	if _, _, err := s.udp.ReadFrom(buf); err != nil {
		return
	}
	conn.Write([]byte{0, 0})
	var szBuf [2]byte
	if _, err := io.ReadFull(conn, szBuf[:]); err != nil {
		return
	}
	sz := binary.BigEndian.Uint16(szBuf[:])
	if _, err := io.CopyN(io.Discard, conn, int64(sz)); err != nil {
		return
	}
	challenge := make([]byte, 16)
	payload := make([]byte, 32)
	binary.BigEndian.PutUint16(payload[0:2], 18)
	copy(payload[16:], challenge)
	binary.BigEndian.PutUint16(szBuf[:], uint16(len(payload)))
	conn.Write(szBuf[:])
	conn.Write(payload)
	if _, err := io.ReadFull(conn, szBuf[:]); err != nil {
		return
	}
	sz = binary.BigEndian.Uint16(szBuf[:])
	if _, err := io.CopyN(io.Discard, conn, int64(sz)); err != nil {
		return
	}
	res := int16(-30972)
	if attempt > 1 {
		res = 0
	}
	resp := make([]byte, 4)
	binary.BigEndian.PutUint16(resp[0:2], 13)
	binary.BigEndian.PutUint16(resp[2:4], uint16(res))
	binary.BigEndian.PutUint16(szBuf[:], uint16(len(resp)))
	conn.Write(szBuf[:])
	conn.Write(resp)
}

func TestLoginTriggersAutoUpdate(t *testing.T) {
	fs := newFakeServer(t)
	defer fs.close()
	host = fs.addr()
	name = "test"
	pass = "pw"
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	calls := 0
	orig := downloadGZ
	downloadGZ = func(url, dest string) error {
		calls++
		return nil
	}
	defer func() { downloadGZ = orig }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- login(ctx, 1) }()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("login: %v", err)
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatalf("login did not return")
	}
	if calls == 0 {
		t.Fatalf("downloadGZ not called")
	}
}
