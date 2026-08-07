//go:build ignore

// This file is meant to be run with: go run ./backend/shared/mirrorlist_gen.go
// It regenerates the mirrorlist files that are embedded in the binary.
//
// Usage:
//   go run ./backend/shared/mirrorlist_gen.go
//
// The generated files are written to backend/shared/mirrorlist/ and are
// embedded into the binary via go:embed.

package main

import (
	"fmt"
	"os"

	"aestheticos-installer/backend/shared"
)

func main() {
	// Change to the shared package directory so relative paths resolve correctly
	if err := os.Chdir("backend/shared"); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Refreshing mirrorlists from Arch Linux...")
	if err := shared.EnsureMirrorLists(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Mirrorlists updated successfully.")
}
