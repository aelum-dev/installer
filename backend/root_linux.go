//go:build linux

package app

import "syscall"

func isRoot() bool {
	return syscall.Getuid() == 0
}
