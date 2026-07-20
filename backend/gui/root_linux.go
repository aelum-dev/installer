//go:build linux

package gui

import "syscall"

func isRoot() bool {
	return syscall.Getuid() == 0
}
