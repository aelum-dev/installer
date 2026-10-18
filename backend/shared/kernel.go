package shared

import (
	"path/filepath"
	"strings"
)

// PickKernelFile selects the kernel image the installer should configure from a
// list of boot-directory entries (vmlinuz-*), as listed by find(1).
//
// The Aelum kernel (KernelFilePrefix+KernelPresetName, e.g. vmlinuz-linux-lts)
// is preferred over whatever find happens to list first. On a dualboot install
// that reuses an existing ESP, the ESP may also hold another distribution's
// kernel (e.g. vmlinuz-linux); picking that one would write a boot entry that
// points Aelum's root flags at an unrelated kernel, so the installed system
// would not boot as configured.
//
// When the preferred kernel is not present (custom builds, tests), the first
// entry is returned so callers keep their previous behaviour. Paths are reduced
// to their base name. Empty input yields "".
func PickKernelFile(files []string) string {
	preferred := KernelFilePrefix + KernelPresetName
	first := ""
	for _, f := range files {
		base := filepath.Base(strings.TrimSpace(f))
		if base == "" || base == "." || base == "/" {
			continue
		}
		if base == preferred {
			return base
		}
		if first == "" {
			first = base
		}
	}
	return first
}
