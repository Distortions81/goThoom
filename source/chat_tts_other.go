//go:build !windows

package main

import "syscall"

func piperSysProcAttr() *syscall.SysProcAttr {
	return nil
}
