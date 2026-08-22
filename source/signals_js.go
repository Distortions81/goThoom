//go:build js

package main

import "os"

func shutdownSignals() []os.Signal {
	return nil
}
