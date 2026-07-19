package shared

// Language represents a selectable language.
type Language struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

// UserAccountData holds user account form fields.
type UserAccountData struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// DiskSelectionData holds disk selection info.
type DiskSelectionData struct {
	DiskID string `json:"diskId"`
}

// DiskInfo represents a block device discovered on the system.
type DiskInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Size       string `json:"size"`
	Type       string `json:"type"`
	Model      string `json:"model"`
	Vendor     string `json:"vendor"`
	Rotational bool   `json:"rotational"`
	Readonly   bool   `json:"ro"`
	Fstype     string `json:"fstype"`
	Mountpoint string `json:"mountpoint"`
	Serial     string `json:"serial"`
	Trans      string `json:"trans"`
}
