//go:build !linux

package gui

func isRoot() bool {
	return true // skip root check on non-Linux platforms
}
