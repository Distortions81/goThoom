package main

import "testing"

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
