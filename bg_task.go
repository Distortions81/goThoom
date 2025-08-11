package main

import "time"

func init() {
	go func() {
		for {
			time.Sleep(time.Second)
			if debugWin != nil && debugWin.IsOpen() {
				updateDebugStats()
			}
		}
	}()
}
