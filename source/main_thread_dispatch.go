package main

import "sync"

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

func drainMainThreadDispatcher() {
	mainThreadDispatchMu.Lock()
	queue := append([]func(){}, mainThreadDispatchQueue...)
	mainThreadDispatchQueue = mainThreadDispatchQueue[:0]
	mainThreadDispatchMu.Unlock()
	for _, action := range queue {
		action()
	}
}
