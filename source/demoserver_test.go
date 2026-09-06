package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"gothoom/internal/demoserver"
)

// Uses the real client's demo discovery, challenge response, framing and draw
// parser so server/client agreement is tested across the package boundary.
func TestDemoServerClientCompatibility(t *testing.T) {
	initFont()
	server, err := demoserver.Listen("127.0.0.1:0", 2)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background()) }()
	t.Cleanup(func() {
		server.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	target := serverTarget{addr: server.Addr().String(), display: "test"}
	names, err := fetchDemoFromTarget(target, 1500, 1500<<8, 1500<<8)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || !containsDemoName(names, "Demo1") || !containsDemoName(names, "Demo2") {
		t.Fatalf("demo names: %v", names)
	}
	login := func(name, password string) (net.Conn, net.Conn, int16) {
		t.Helper()
		tcp, err := net.Dial("tcp", target.addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { tcp.Close() })
		tcp.SetDeadline(time.Now().Add(3 * time.Second))
		udp, err := net.Dial("udp", target.addr)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { udp.Close() })
		id := make([]byte, 4)
		if _, err = io.ReadFull(tcp, id); err != nil {
			t.Fatal(err)
		}
		udp.Write(append([]byte{255, 255}, id...))
		confirm := make([]byte, 2)
		if _, err = io.ReadFull(tcp, confirm); err != nil {
			t.Fatal(err)
		}
		if err = sendClientIdentifiers(tcp, 1500<<8, 1500<<8, 1500<<8); err != nil {
			t.Fatal(err)
		}
		challenge, err := readTCPMessage(tcp)
		if err != nil {
			t.Fatal(err)
		}
		answer, err := answerChallenge(password, challenge[16:])
		if err != nil {
			t.Fatal(err)
		}
		packet := make([]byte, 16)
		binary.BigEndian.PutUint16(packet, 13)
		packet = append(packet, []byte(name)...)
		packet = append(packet, 0)
		packet = append(packet, answer...)
		simpleEncrypt(packet[16:])
		if err = sendTCPMessage(tcp, packet); err != nil {
			t.Fatal(err)
		}
		response, err := readTCPMessage(tcp)
		if err != nil {
			t.Fatal(err)
		}
		return tcp, udp, int16(binary.BigEndian.Uint16(response[2:]))
	}
	a, au, result := login("Demo1", "demo")
	if result != 0 {
		t.Fatalf("login: %d", result)
	}
	b, bu, result := login("Demo2", "demo")
	if result != 0 {
		t.Fatalf("second login: %d", result)
	}
	_, _, result = login("Demo1", "demo")
	if result != loginResultCharacterAlreadyOnline {
		t.Fatalf("busy slot: %d", result)
	}
	_, _, result = login("Demo1", "wrong")
	if result == 0 {
		t.Fatal("accepted wrong demo password")
	}
	send := func(conn net.Conn, cmd byte, text string, moving bool) {
		t.Helper()
		packet := make([]byte, 20)
		binary.BigEndian.PutUint16(packet, 3)
		binary.BigEndian.PutUint16(packet[2:], 200)
		if moving {
			binary.BigEndian.PutUint16(packet[6:], 1)
		}
		binary.BigEndian.PutUint32(packet[16:], uint32(cmd))
		packet = append(packet, []byte(text)...)
		packet = append(packet, 0)
		if err := sendUDPMessage(conn, packet); err != nil {
			t.Fatal(err)
		}
	}
	send(au, 1, "hello from demo", true)
	send(bu, 0, "", false)
	oldName, oldMovie := playerName, movieMode
	playerName = "Demo1"
	movieMode = true
	t.Cleanup(func() { playerName = oldName; movieMode = oldMovie; resetDrawState() })
	// Both recipients must receive the chat exactly once, and every frame must
	// parse as real client draw data with both visible players.
	for _, conn := range []net.Conn{a, b} {
		found := false
		for i := 0; i < 20; i++ {
			packet, err := readTCPMessage(conn)
			if err != nil {
				t.Fatal(err)
			}
			if binary.BigEndian.Uint16(packet) != 2 {
				t.Fatal("not a draw frame")
			}
			if _, _, err = parseDrawState(packet[2:], false); err != nil {
				t.Fatalf("client rejected server frame: %v", err)
			}
			if bytes.Contains(packet, []byte("hello from demo")) {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("chat did not reach both players")
		}
	}
	send(au, 1, "hello from demo", true) // retry same command must not duplicate chat
	packet, err := readTCPMessage(a)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(packet, []byte("hello from demo")) {
		t.Fatal("command retry duplicated chat")
	}
	// After a few ticks, the first player's world coordinate must have advanced.
	for i := 0; i < 4; i++ {
		packet, err = readTCPMessage(a)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err = parseDrawState(packet[2:], false); err != nil {
			t.Fatal(err)
		}
	}
	stateMu.Lock()
	mobile := state.mobiles[1]
	count := len(state.mobiles)
	stateMu.Unlock()
	if count != 2 || mobile.H <= -72 {
		t.Fatalf("movement/visibility: count=%d position=%d", count, mobile.H)
	}
	a.Close()
	reused := false
	for i := 0; i < 20; i++ {
		conn, _, code := login("Demo1", "demo")
		if code == 0 {
			conn.Close()
			reused = true
			break
		}
		if code != loginResultCharacterAlreadyOnline {
			t.Fatalf("reconnect result: %d", code)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !reused {
		t.Fatal("disconnect did not release demo slot")
	}

}
func containsDemoName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
