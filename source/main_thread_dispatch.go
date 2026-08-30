package main

import (
	"context"
	"sync"
)

var (
	mainThreadDispatchMu    sync.Mutex
	mainThreadDispatchQueue []func()
)

func dispatchMainThread(action func()) {
	if action == nil {
		return
	}
	mainThreadDispatchMu.Lock()
	mainThreadDispatchQueue = append(mainThreadDispatchQueue, action)
	mainThreadDispatchMu.Unlock()
}

// dispatchMainThreadAndWait hands UI/window work to the game loop and waits
// until it has run. Startup workers use this instead of racing EUI layout and
// persisted-window restoration from a background goroutine.
func dispatchMainThreadAndWait(ctx context.Context, action func()) bool {
	done := make(chan struct{})
	dispatchMainThread(func() {
		defer close(done)
		if ctx.Err() == nil && action != nil {
			action()
		}
	})
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

func drainMainThreadDispatcher() {
	mainThreadDispatchMu.Lock()
	queue := append([]func(){}, mainThreadDispatchQueue...)
	mainThreadDispatchQueue = mainThreadDispatchQueue[:0]
	mainThreadDispatchMu.Unlock()
	for _, action := range queue {
		action()
	}
}
