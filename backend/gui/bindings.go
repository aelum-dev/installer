package gui

import "aelum-installer/backend/shared"

// GetDisks returns a list of available block devices. Wails binding.
func (a *App) GetDisks() ([]shared.DiskInfo, error) {
	return shared.GetDisks()
}

// GetDiskPartitions returns detailed partition information for a disk. Wails binding.
func (a *App) GetDiskPartitions(diskPath string) ([]shared.PartitionDetail, error) {
	return shared.GetDiskPartitions(diskPath)
}

// GetLanguages returns the list of available languages/regions. Wails binding.
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
