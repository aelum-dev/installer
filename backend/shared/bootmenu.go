package shared

import (
	"path/filepath"
	"strings"
)

// InstallID derives a short, stable identifier for one Aelum installation from
// its root filesystem UUID. The identifier names the installation's rEFInd menu
// fragment and kernel directory, so each install keeps its own entry
// (reinstalling the same root UUID deliberately reuses the same entry).
//
// Only [a-z0-9] characters survive, and at most the first eight: UUIDs are
// already hexadecimal, so this is the familiar short-UUID form. Non-UUID input
// (tests, unusual callers) is sanitized the same way; empty input yields
// "install" so the caller never builds a path ending in "/.conf".
func InstallID(rootUUID string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(rootUUID) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			if b.Len() == 8 {
				break
			}
		}
	}
	if b.Len() == 0 {
		return "install"
	}
	return b.String()
}

// RefindEntryPath is the ESP-relative path of one installation's rEFInd menu
// fragment, e.g. EFI/refind/aelum/1a2b3c4d.conf.
func RefindEntryPath(id string) string {
	return RefindEntriesDir + "/" + id + ".conf"
}

// RefindEntryInclude is the value of the "include" token that pulls an
// installation's menu fragment into refind.conf. rEFInd resolves includes
// relative to its own directory (EFI/refind), so this omits RefindDir.
func RefindEntryInclude(id string) string {
	return RefindEntriesDirName + "/" + id + ".conf"
}

// RefindEntryTitle is the menu title for one installation. The root partition
// suffix (e.g. "Aelum (sda2)") keeps two side-by-side installs distinguishable,
// and lets default_selection target the install that was just written.
func RefindEntryTitle(rootPart string) string {
	base := filepath.Base(strings.TrimSpace(rootPart))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return BootEntryTitle
	}
	return BootEntryTitle + " (" + base + ")"
}

// InstallBootDir is the ESP-relative directory holding one installation's
// kernel and initramfs (e.g. aelum/1a2b3c4d). The installed system bind-mounts
// it at /boot, so pacman and mkinitcpio keep writing to /boot as usual while
// the files physically stay inside this installation's directory.
func InstallBootDir(id string) string {
	return EspInstallDirName + "/" + id
}

// InstallBootFilePath is the ESP-relative path of a kernel or initramfs file
// belonging to one installation. rEFInd resolves loader/initrd paths against
// the ESP root, so these are the paths written into the menu fragment.
func InstallBootFilePath(id, name string) string {
	return InstallBootDir(id) + "/" + name
}
