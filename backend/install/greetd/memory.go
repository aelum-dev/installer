package greetd

import (
	_ "embed"
)

//go:embed data/greetd/memory.json
var defaultMemoryJSON string

// MemoryJSON returns the embedded memory.json template with {{USERNAME}} placeholder.
func MemoryJSON() string {
	return defaultMemoryJSON
}
