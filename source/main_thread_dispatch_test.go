package main

import (
	"context"
	"testing"
	"time"
)

func TestMainThreadDispatcherDefersUntilDrain(t *testing.T) {
	mainThreadDispatchMu.Lock()
	originalQueue := mainThreadDispatchQueue
	mainThreadDispatchQueue = nil
	mainThreadDispatchMu.Unlock()
	t.Cleanup(func() {
		mainThreadDispatchMu.Lock()
		mainThreadDispatchQueue = originalQueue
		mainThreadDispatchMu.Unlock()
	})

	ran := false
	dispatchMainThread(func() { ran = true })
	if ran {
		t.Fatal("dispatched action ran on the calling goroutine")
	}
	drainMainThreadDispatcher()
	if !ran {
		t.Fatal("dispatched action did not run when the main-thread queue drained")
	}
}

func TestMainThreadDispatcherWaitsForDrain(t *testing.T) {
	mainThreadDispatchMu.Lock()
	originalQueue := mainThreadDispatchQueue
	mainThreadDispatchQueue = nil
	mainThreadDispatchMu.Unlock()
	t.Cleanup(func() {
		mainThreadDispatchMu.Lock()
		mainThreadDispatchQueue = originalQueue
		mainThreadDispatchMu.Unlock()
	})

	ran := false
	result := make(chan bool, 1)
	go func() {
		result <- dispatchMainThreadAndWait(context.Background(), func() { ran = true })
	}()

	deadline := time.Now().Add(time.Second)
	for {
		mainThreadDispatchMu.Lock()
		queued := len(mainThreadDispatchQueue)
		mainThreadDispatchMu.Unlock()
		if queued > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("wait action was not queued")
		}
		time.Sleep(time.Millisecond)
	}
	if ran {
		t.Fatal("wait action ran before the main-thread queue drained")
	}
	drainMainThreadDispatcher()
	if ok := <-result; !ok || !ran {
		t.Fatalf("dispatch result = %t, ran = %t; want both true", ok, ran)
	}
}
