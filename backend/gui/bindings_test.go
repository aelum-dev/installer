package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

func TestBindings_GetLanguages(t *testing.T) {
	a := NewApp(false)
	langs := a.GetLanguages()
	if len(langs) == 0 {
		t.Fatal("GetLanguages() returned no languages")
	}
}

func TestBindings_SelectionRoundTrip(t *testing.T) {
	a := NewApp(false)

	if a.GetSelectedLanguage() != "" {
		t.Errorf("fresh selected language = %q, want empty", a.GetSelectedLanguage())
	}
	if err := a.SetSelectedLanguage("de_DE"); err != nil {
		t.Fatalf("SetSelectedLanguage: %v", err)
	}
	if got := a.GetSelectedLanguage(); got != "de_DE" {
		t.Errorf("GetSelectedLanguage() = %q, want de_DE", got)
	}
	if a.region.Code != "de_DE" {
		t.Errorf("region = %+v, want de_DE", a.region)
	}

	a.SaveDiskSelection(shared.DiskSelectionData{DiskID: "/dev/vda", FileSystem: shared.FileSystemBtrfs})
	if got := a.GetFileSystem(); got != shared.FileSystemBtrfs {
		t.Errorf("GetFileSystem() = %q, want btrfs", got)
	}
	if got := a.GetDiskID(); got != "/dev/vda" {
		t.Errorf("GetDiskID() = %q, want /dev/vda", got)
	}
}

func TestBindings_ManualMode(t *testing.T) {
	a := NewApp(false)
	if a.GetManualMode() {
		t.Error("manual mode should default to false")
	}

	layout := shared.ManualLayout{Disk: "/dev/vda"}
	a.SaveManualLayout(layout)
	if !a.GetManualMode() {
		t.Error("SaveManualLayout should enable manual mode")
	}
	if a.manualLayout.Disk != "/dev/vda" {
		t.Errorf("stored layout = %+v", a.manualLayout)
	}
}

func TestBindings_ValidateManualLayoutUsesLvmFlag(t *testing.T) {
	a := NewApp(false)
	a.lvm = true

	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    20480,
		DiskSizeBytes: 20480 * 1024 * 1024,
		Partitions: []shared.ManualPartition{
			{Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * 1024 * 1024, Type: shared.TypeESP},
			{Path: "/dev/vda2", SizeMB: 8192, SizeBytes: 8192 * 1024 * 1024, Type: shared.TypeSystem},
		},
	}
	if err := a.ValidateManualLayout(layout); err == nil {
		t.Fatal("ValidateManualLayout() = nil, want LVM floor error")
	}
}

func TestBindings_SaveManualLayoutStoresCryptoFlags(t *testing.T) {
	a := NewApp(false)
	a.lvm = true
	a.luks = true

	a.SaveManualLayout(shared.ManualLayout{Disk: "/dev/vda"})
	if !a.manualLayout.Lvm || !a.manualLayout.Luks {
		t.Errorf("stored layout flags = lvm:%v luks:%v, want both true", a.manualLayout.Lvm, a.manualLayout.Luks)
	}

	// Disabling a toggle must clear a stale layout flag, not leave it enabled.
	a.lvm = false
	a.luks = false
	a.SaveManualLayout(shared.ManualLayout{Disk: "/dev/vda", Lvm: true, Luks: true})
	if a.manualLayout.Lvm || a.manualLayout.Luks {
		t.Errorf("stale flags survived disable: lvm:%v luks:%v", a.manualLayout.Lvm, a.manualLayout.Luks)
	}
}

func TestBindings_ValidateManualLayoutClearsDisabledLvm(t *testing.T) {
	a := NewApp(false)
	a.lvm = false

	layout := shared.ManualLayout{
		Disk:          "/dev/vda",
		DiskSizeMB:    20480,
		DiskSizeBytes: 20480 * 1024 * 1024,
		Lvm:           true, // stale value from a layout loaded before the toggle was turned off
		Partitions: []shared.ManualPartition{
			{Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * 1024 * 1024, Type: shared.TypeESP, ExistingFstype: "vfat"},
			{Path: "/dev/vda2", SizeMB: 8192, SizeBytes: 8192 * 1024 * 1024, Type: shared.TypeSystem},
		},
	}
	if err := a.ValidateManualLayout(layout); err != nil {
		t.Fatalf("ValidateManualLayout() = %v, want nil with LVM disabled", err)
	}
}

func TestBindings_GetDisksAndPartitions(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1")
	a := NewApp(false)

	disks, err := a.GetDisks()
	if err != nil {
		t.Fatalf("GetDisks: %v", err)
	}
	if len(disks) == 0 {
		t.Fatal("GetDisks returned no disks")
	}

	parts, err := a.GetDiskPartitions("/dev/vda")
	if err != nil {
		t.Fatalf("GetDiskPartitions: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("GetDiskPartitions(/dev/vda) = %d entries, want 2", len(parts))
	}
}

func TestBindings_GetManualLayout(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1")
	// The sgdisk geometry probe must not run for real.
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"sgdisk", "-p"}, "")
	defer primitives.SetCommandRunner(rec)()

	a := NewApp(false)
	a.lvm = true
	a.luks = true
	layout, err := a.GetManualLayout("/dev/vda")
	if err != nil {
		t.Fatalf("GetManualLayout: %v", err)
	}
	if layout.Disk != "/dev/vda" {
		t.Errorf("layout.Disk = %q", layout.Disk)
	}
	if !layout.Lvm || !layout.Luks {
		t.Errorf("layout flags = lvm:%v luks:%v, want both true", layout.Lvm, layout.Luks)
	}
	if len(layout.Partitions) != 2 {
		t.Errorf("layout partitions = %d, want 2", len(layout.Partitions))
	}
	if layout.SectorSizeBytes != 512 {
		t.Errorf("SectorSizeBytes = %d, want 512", layout.SectorSizeBytes)
	}
}

func TestBindings_ValidateManualLayout(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"sgdisk", "-p"}, "")
	defer primitives.SetCommandRunner(rec)()
	a := NewApp(false)

	valid := shared.ManualLayout{
		Disk:            "/dev/vda",
		DiskSizeMB:      40960,
		DiskSizeBytes:   40960 * 1024 * 1024,
		SectorSizeBytes: 512,
		Partitions: []shared.ManualPartition{
			{Type: shared.TypeESP, SizeMB: 512},
			{Type: shared.TypeSystem, SizeMB: 0},
		},
	}
	if err := a.ValidateManualLayout(valid); err != nil {
		t.Errorf("ValidateManualLayout(valid) = %v, want nil", err)
	}

	invalid := valid
	invalid.Partitions = []shared.ManualPartition{{Type: shared.TypeESP, SizeMB: 512}}
	if err := a.ValidateManualLayout(invalid); err == nil {
		t.Error("ValidateManualLayout(invalid) = nil, want error")
	}
}

func TestBindings_SetWallpaper(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	writeShim(t, "hyprctl", "echo \"$@\" > "+argsFile+"\n")

	a := NewApp(false)
	if err := a.SetWallpaper("dark"); err != nil {
		t.Fatalf("SetWallpaper(dark): %v", err)
	}
	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read hyprctl args: %v", err)
	}
	line := strings.TrimSpace(string(got))
	if !strings.Contains(line, shared.LiveIsoBgDark) || !strings.Contains(line, ",cover") {
		t.Errorf("hyprctl args = %q, want dark wallpaper with cover fit", line)
	}

	if err := a.SetWallpaper("light"); err != nil {
		t.Fatalf("SetWallpaper(light): %v", err)
	}
	got, _ = os.ReadFile(argsFile)
	if !strings.Contains(string(got), shared.LiveIsoBgLight) {
		t.Errorf("hyprctl args = %q, want light wallpaper", strings.TrimSpace(string(got)))
	}
}

func TestBindings_SetWallpaper_Failure(t *testing.T) {
	writeShim(t, "hyprctl", "echo 'no compositor' >&2\nexit 1\n")

	a := NewApp(false)
	err := a.SetWallpaper("dark")
	if err == nil || !strings.Contains(err.Error(), "failed to switch wallpaper") {
		t.Fatalf("SetWallpaper() error = %v, want wallpaper failure", err)
	}
}
