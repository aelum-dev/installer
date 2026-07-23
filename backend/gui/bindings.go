package gui

import (
	"aestheticos-installer/backend/internal/shared"
)

// --- GUI methods (delegate to shared package) ---

// GetDisks returns a list of available block devices (disks) for installation.
// Wails binding: frontend calls this.
func (a *App) GetDisks() ([]shared.DiskInfo, error) {
	return shared.GetDisks()
}

// GetDiskPartitions returns detailed partition information for a disk.
// Wails binding: frontend calls this when a disk is selected.
func (a *App) GetDiskPartitions(diskPath string) ([]shared.PartitionDetail, error) {
	return shared.GetDiskPartitions(diskPath)
}



// GetLanguages returns the list of available languages/regions.
// Wails binding: frontend calls this.
func (a *App) GetLanguages() []shared.Language {
	return shared.GetLanguages()
}

// GetSelectedLanguage returns the currently selected language/region code.
func (a *App) GetSelectedLanguage() string {
	return a.selectedLanguage
}

// SetSelectedLanguage sets the selected language/region.
func (a *App) SetSelectedLanguage(code string) error {
	a.selectedLanguage = code
	a.region = shared.GetRegion(code)
	return nil
}

// GetFileSystem returns the currently selected filesystem type.
func (a *App) GetFileSystem() string {
	return a.fileSystem
}
