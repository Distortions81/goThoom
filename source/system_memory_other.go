//go:build !linux && !darwin && !windows

package main

func systemMemoryBytes() uint64 {
	return 0
}
