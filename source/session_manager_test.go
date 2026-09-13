package main

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionManagerStartsWithOnlyPrimarySession(t *testing.T) {
	primary := mustNewSession(primarySessionID)
	manager := newSessionManager(primary)

	selected := manager.selectedSession()
	if selected != primary {
		t.Fatalf("selected session = %p, want primary %p", selected, primary)
	}
	if _, ok := manager.session(primarySessionID); !ok {
		t.Fatal("primary session is missing")
	}
	if _, ok := manager.session(2); ok {
		t.Fatal("secondary session exists before multi-session is enabled")
	}
	if manager.selectSession(2) {
		t.Fatal("selected an inactive session slot")
	}
}

func TestSessionManagerEnablesFourStableSlots(t *testing.T) {
	primary := mustNewSession(primarySessionID)
	manager := newSessionManager(primary)
	slots := manager.enableMulti()

	if slots[0] != primary {
		t.Fatal("enabling multi-session replaced the primary session")
	}
	for slot, session := range slots {
		if session == nil {
			t.Fatalf("slot %d was not created", slot)
		}
		id, _ := sessionIDForSlot(slot)
		if session.ID() != id {
			t.Fatalf("slot %d has session ID %d, want %d", slot, session.ID(), id)
		}
	}

	if !manager.selectSession(3) || manager.selectedSession() != slots[2] {
		t.Fatal("selection did not switch to session three")
	}
	again := manager.enableMulti()
	for slot := range slots {
		if again[slot] != slots[slot] {
			t.Fatalf("enabling multi-session twice replaced slot %d", slot)
		}
	}
}

func TestSessionManagerDisconnectAllTargetsEveryActiveSlot(t *testing.T) {
	primary := mustNewSession(primarySessionID)
	manager := newSessionManager(primary)
	slots := manager.enableMulti()
	var peers []net.Conn
	for _, session := range slots[:2] {
		tcp, tcpPeer := net.Pipe()
		udp, udpPeer := net.Pipe()
		peers = append(peers, tcpPeer, udpPeer)
		if _, ok := session.transport.attach(tcp, udp); !ok {
			t.Fatal("could not attach test transport")
		}
	}
	t.Cleanup(func() {
		for _, peer := range peers {
			_ = peer.Close()
		}
	})

	if got := manager.disconnectAll(); got != 2 {
		t.Fatalf("disconnect count = %d, want 2", got)
	}
	for index, session := range slots[:2] {
		if session.transport.busy() {
			t.Fatalf("slot %d transport is still busy", index+1)
		}
	}
}

func TestSessionManagerRejectsInvalidLoginBeforeStarting(t *testing.T) {
	manager := newSessionManager(mustNewSession(primarySessionID))
	if _, err := manager.startLogin(context.Background(), 1, sessionLoginRequest{}, 1); err == nil {
		t.Fatal("empty login request started")
	}
	if manager.anyBusy() {
		t.Fatal("invalid login left the session busy")
	}
}

func TestSessionTransportCannotReconnectUntilDisconnectJoins(t *testing.T) {
	session := mustNewSession(2)
	if !session.transport.begin(func() {}) {
		t.Fatal("could not begin transport")
	}
	session.transport.mu.RLock()
	generation := session.transport.generation
	done := session.transport.done
	session.transport.mu.RUnlock()
	if !session.transport.disconnect() || !session.transport.busy() {
		t.Fatal("disconnect did not retain teardown ownership")
	}
	if session.transport.begin(func() {}) {
		t.Fatal("reconnected before old transport teardown completed")
	}
	if !session.transport.finish(generation) {
		t.Fatal("old transport could not finish after disconnect")
	}
	select {
	case <-done:
	default:
		t.Fatal("transport completion was not closed")
	}
	if session.transport.busy() || !session.transport.begin(func() {}) {
		t.Fatal("transport was not reusable after teardown")
	}
	session.transport.failConnect()
}

func TestSessionManagerReconnectSupervisorsAreIndependent(t *testing.T) {
	manager := newSessionManager(mustNewSession(primarySessionID))
	slots := manager.enableMulti()
	var attempts [maxSessions]atomic.Int32
	manager.login = func(session *Session, ctx context.Context, _ int, _ []string) error {
		attempt := attempts[session.ID()-1].Add(1)
		if attempt == 1 {
			return io.EOF
		}
		<-ctx.Done()
		return ctx.Err()
	}
	manager.wait = func(context.Context, time.Duration) error { return nil }
	request := sessionLoginRequest{host: "example.invalid:5010", character: "Test", password: "secret"}
	first, err := manager.startLogin(context.Background(), slots[0].ID(), request, 1)
	if err != nil {
		t.Fatalf("start first login: %v", err)
	}
	second, err := manager.startLogin(context.Background(), slots[1].ID(), request, 1)
	if err != nil {
		t.Fatalf("start second login: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for (attempts[0].Load() < 2 || attempts[1].Load() < 2) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := attempts[0].Load(); got < 2 {
		t.Fatalf("first session attempts = %d, want at least 2", got)
	}
	if got := attempts[1].Load(); got < 2 {
		t.Fatalf("second session attempts = %d, want at least 2", got)
	}
	if !manager.disconnectSession(slots[0].ID()) {
		t.Fatal("first session supervisor was not cancelled")
	}
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("first session result = %v, want cancellation", err)
	}
	if !slots[1].login.supervisorActive() {
		t.Fatal("cancelling the first session also stopped the second")
	}
	if !manager.disconnectSession(slots[1].ID()) {
		t.Fatal("second session supervisor was not cancelled")
	}
	if err := <-second; !errors.Is(err, context.Canceled) {
		t.Fatalf("second session result = %v, want cancellation", err)
	}
}

func TestSessionManagerReconnectDelayResetsAfterConnectedLifetime(t *testing.T) {
	manager := newSessionManager(mustNewSession(primarySessionID))
	var attempts atomic.Int32
	manager.login = func(_ *Session, ctx context.Context, _ int, _ []string) error {
		switch attempts.Add(1) {
		case 1, 2:
			return io.EOF
		case 3:
			return nil
		default:
			<-ctx.Done()
			return ctx.Err()
		}
	}
	delays := make(chan time.Duration, 3)
	manager.wait = func(_ context.Context, delay time.Duration) error {
		delays <- delay
		return nil
	}
	request := sessionLoginRequest{host: "example.invalid:5010", character: "Test", password: "secret"}
	result, err := manager.startLogin(context.Background(), primarySessionID, request, 1)
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	for index, want := range []time.Duration{time.Second, 2 * time.Second, time.Second} {
		select {
		case got := <-delays:
			if got != want {
				t.Fatalf("delay %d = %v, want %v", index, got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for delay %d", index)
		}
	}
	manager.disconnectSession(primarySessionID)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("login result = %v, want cancellation", err)
	}
}

func TestSessionManagerDisconnectAllWaitsForSupervisorShutdown(t *testing.T) {
	manager := newSessionManager(mustNewSession(primarySessionID))
	entered := make(chan struct{})
	release := make(chan struct{})
	manager.login = func(_ *Session, ctx context.Context, _ int, _ []string) error {
		close(entered)
		<-ctx.Done()
		<-release
		return ctx.Err()
	}
	request := sessionLoginRequest{host: "example.invalid:5010", character: "Test", password: "secret"}
	result, err := manager.startLogin(context.Background(), primarySessionID, request, 1)
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	<-entered
	disconnected := make(chan error, 1)
	go func() {
		count, err := manager.disconnectAllAndWait(context.Background())
		if count != 1 && err == nil {
			err = errors.New("disconnect count did not include supervisor")
		}
		disconnected <- err
	}()
	select {
	case err := <-disconnected:
		t.Fatalf("shutdown returned before supervisor cleanup: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-disconnected; err != nil {
		t.Fatalf("disconnect all: %v", err)
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("login result = %v, want cancellation", err)
	}
}

func TestSessionManagerDisconnectCancelsReconnectBackoff(t *testing.T) {
	manager := newSessionManager(mustNewSession(primarySessionID))
	attempted := make(chan struct{})
	manager.login = func(_ *Session, _ context.Context, _ int, _ []string) error {
		close(attempted)
		return io.EOF
	}
	request := sessionLoginRequest{host: "example.invalid:5010", character: "Test", password: "secret"}
	result, err := manager.startLogin(context.Background(), primarySessionID, request, 1)
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	<-attempted
	deadline := time.Now().Add(time.Second)
	for {
		status, _ := manager.slots[0].login.statusSnapshot()
		if strings.HasPrefix(status, "Reconnecting in ") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("session never entered reconnect backoff")
		}
		time.Sleep(time.Millisecond)
	}
	started := time.Now()
	manager.disconnectSession(primarySessionID)
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("login result = %v, want cancellation", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("disconnect did not cancel reconnect backoff promptly")
	}
	if elapsed := time.Since(started); elapsed >= sessionReconnectInitialDelay {
		t.Fatalf("backoff cancellation took %v", elapsed)
	}
}

func TestMultiSessionSelectionAndShutdownStress(t *testing.T) {
	manager := newSessionManager(mustNewSession(primarySessionID))
	slots := manager.enableMulti()
	started := make(chan SessionID, maxSessions)
	manager.login = func(session *Session, ctx context.Context, _ int, _ []string) error {
		started <- session.ID()
		<-ctx.Done()
		return ctx.Err()
	}
	request := sessionLoginRequest{host: "example.invalid:5010", character: "Stress", password: "secret"}
	results := make([]<-chan error, 0, maxSessions)
	for _, session := range slots {
		result, err := manager.startLogin(context.Background(), session.ID(), request, 1)
		if err != nil {
			t.Fatalf("start session %d: %v", session.ID(), err)
		}
		results = append(results, result)
	}
	for range maxSessions {
		<-started
	}
	var selectors sync.WaitGroup
	for worker := range maxSessions {
		worker := worker
		selectors.Add(1)
		go func() {
			defer selectors.Done()
			for iteration := range 500 {
				id := slots[(worker+iteration)%len(slots)].ID()
				if !manager.selectSession(id) {
					t.Errorf("could not select session %d", id)
					return
				}
			}
		}()
	}
	selectors.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if count, err := manager.disconnectAllAndWait(ctx); err != nil || count != maxSessions {
		t.Fatalf("disconnect all = %d, %v; want %d, nil", count, err, maxSessions)
	}
	for index, result := range results {
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("session %d result = %v, want cancellation", index+1, err)
		}
	}
	if manager.anyBusy() {
		t.Fatal("manager remained busy after all supervisors joined")
	}
}
