//go:build !linux

package app

func isRoot() bool {
	return true // skip root check on non-Linux platforms
}
