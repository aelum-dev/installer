package app

import (
	"aesthetic-os-installer/backend/shared"
)

// GetDisks returns a list of available block devices (disks) for installation.
// Wails binding: frontend calls this.
func (a *App) GetDisks() ([]shared.DiskInfo, error) {
	return shared.GetDisks()
}
