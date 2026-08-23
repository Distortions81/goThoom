//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhysical        uint64
	availablePhysical    uint64
	totalPageFile        uint64
	availablePageFile    uint64
	totalVirtual         uint64
	availableVirtual     uint64
	availableExtendedVir uint64
}

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func systemMemoryBytes() uint64 {
	status := memoryStatusEx{length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	ok, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return 0
	}
	return status.totalPhysical
}
