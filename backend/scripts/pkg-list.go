//go:build ignore

// pkg-list — prints every package name the installer installs into a target,
// one per line, in a stable order.
//
// Usage: go run ./backend/scripts/pkg-list.go
//
// The ISO build (docker/scripts/build-iso.sh) feeds this into the same
// `pacman -Syw` / `pacman -S --print` pair that reads live-iso/packages.x86_64,
// so the offline mirror always holds exactly what an install asks for plus what
// the live session runs. That is what lets the live package list stay short: a
// package only the *installed* system needs does not have to be listed in the
// live profile to reach the mirror, and it cannot drift out of it either — the
// lists here are the only copy of the truth.
//
// Btrfs-only extras are printed unconditionally: the image is built once and
// installed onto both filesystems, so the mirror must carry both sets.

package main

import (
	"fmt"
	"sort"

	"aelum-installer/backend/install/installbase"
	"aelum-installer/backend/install/installdesktop"
)

func main() {
	seen := map[string]bool{}
	var names []string
	for _, list := range [][]string{
		installbase.BasePackages,
		installbase.BtrfsOnlyBasePackages,
		installdesktop.DesktopPackages,
		installdesktop.BtrfsOnlyDesktopPackages,
	} {
		for _, name := range list {
			if seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
}
