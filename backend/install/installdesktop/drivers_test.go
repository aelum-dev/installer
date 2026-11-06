package installdesktop

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aelum-installer/backend/shared"
)

// --- detection fixtures ------------------------------------------------------

// writePCIDevice creates one sysfs device directory under root, mirroring
// /sys/bus/pci/devices/<slot>/. class/vendor/device are written verbatim (the
// "0x…" form sysfs uses); driver is the bound driver's directory name — empty
// leaves the driver symlink absent, exactly like a device nothing has bound.
func writePCIDevice(t *testing.T, root, slot, class, vendor, device, driver string) {
	t.Helper()
	dir := filepath.Join(root, "bus", "pci", "devices", slot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"class": class, "vendor": vendor, "device": device}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if driver != "" {
		// sysfs points the driver link at the driver directory; only the base
		// name is read back, so the target need not exist.
		if err := os.Symlink(filepath.Join(root, "bus", "pci", "drivers", driver), filepath.Join(dir, "driver")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDetectGPUs(t *testing.T) {
	root := t.TempDir()
	writePCIDevice(t, root, "0000:01:00.0", "0x030000", "0x1002", "0x73ff", "amdgpu")
	writePCIDevice(t, root, "0000:00:02.0", "0x030000", "0x8086", "0x7d55", "i915")
	writePCIDevice(t, root, "0000:02:00.0", "0x030200", "0x10de", "0x1e04", "") // 3D controller, nothing bound
	// Not display devices: an audio function and a bridge.
	writePCIDevice(t, root, "0000:01:00.1", "0x040300", "0x1002", "0x1a37", "snd_hda_intel")
	writePCIDevice(t, root, "0000:00:01.0", "0x060100", "0x8086", "0x7a04", "")
	// Malformed: no parseable class.
	writePCIDevice(t, root, "0000:fe:ff.f", "", "", "", "")

	devices := detectGPUs(root)
	if len(devices) != 3 {
		t.Fatalf("detectGPUs() found %d devices, want 3: %+v", len(devices), devices)
	}

	bySlot := map[string]GPUDevice{}
	for _, d := range devices {
		bySlot[d.Slot] = d
	}
	amd := bySlot["0000:01:00.0"]
	if amd.Vendor != 0x1002 || amd.Device != 0x73ff || amd.Class != pciClassVGA || amd.Driver != "amdgpu" {
		t.Errorf("AMD device = %+v", amd)
	}
	nv := bySlot["0000:02:00.0"]
	if nv.Vendor != 0x10de || nv.Device != 0x1e04 || nv.Class != pciClass3D || nv.Driver != "" {
		t.Errorf("unbound NVIDIA device = %+v, want class 0302 and no driver", nv)
	}
}

func TestDetectGPUs_MissingTree(t *testing.T) {
	if got := detectGPUs(filepath.Join(t.TempDir(), "absent")); got != nil {
		t.Errorf("detectGPUs(absent) = %v, want none", got)
	}
}

// --- the table ---------------------------------------------------------------

func TestSelectDriverPackages(t *testing.T) {
	tests := []struct {
		name    string
		devices []GPUDevice
		want    []string
	}{
		{
			name:    "AMD",
			devices: []GPUDevice{{Vendor: 0x1002, Device: 0x73ff}},
			want:    []string{"vulkan-radeon", "lib32-vulkan-radeon", "lib32-mesa"},
		},
		{
			name:    "Intel",
			devices: []GPUDevice{{Vendor: 0x8086, Device: 0x7d55}},
			want:    []string{"vulkan-intel", "lib32-vulkan-intel", "lib32-mesa"},
		},
		{
			name:    "NVIDIA Turing",
			devices: []GPUDevice{{Vendor: 0x10de, Device: 0x1e04}},
			want:    []string{"nvidia-open-lts", "nvidia-utils", "lib32-nvidia-utils"},
		},
		{
			// The floor is exactly the first Turing id: 1e02 is TITAN RTX and
			// everything below it in pci.ids is Volta (1df6 tops out) or older.
			name:    "NVIDIA at the floor",
			devices: []GPUDevice{{Vendor: 0x10de, Device: 0x1e02}},
			want:    []string{"nvidia-open-lts", "nvidia-utils", "lib32-nvidia-utils"},
		},
		{
			name:    "NVIDIA Pascal",
			devices: []GPUDevice{{Vendor: 0x10de, Device: 0x1d01}},
			want:    []string{"vulkan-nouveau", "lib32-vulkan-nouveau", "lib32-mesa"},
		},
		{
			name:    "NVIDIA Volta",
			devices: []GPUDevice{{Vendor: 0x10de, Device: 0x1df6}},
			want:    []string{"vulkan-nouveau", "lib32-vulkan-nouveau", "lib32-mesa"},
		},
		{
			name:    "virtio GPU",
			devices: []GPUDevice{{Vendor: 0x1af4, Device: 0x1050}},
			want:    []string{"vulkan-swrast", "lib32-vulkan-swrast", "vulkan-virtio", "lib32-mesa"},
		},
		{
			name:    "VMware SVGA",
			devices: []GPUDevice{{Vendor: 0x15ad, Device: 0x0405}},
			want:    []string{"vulkan-swrast", "lib32-vulkan-swrast", "vulkan-virtio", "lib32-mesa"},
		},
		{
			// QEMU's std VGA (0x1234) is claimed by nobody: the fallback set.
			name:    "unrecognised vendor",
			devices: []GPUDevice{{Vendor: 0x1234, Device: 0x1111}},
			want:    []string{"vulkan-swrast", "lib32-vulkan-swrast", "vulkan-virtio", "lib32-mesa"},
		},
		{
			name:    "no display device at all",
			devices: nil,
			want:    []string{"vulkan-swrast", "lib32-vulkan-swrast", "vulkan-virtio", "lib32-mesa"},
		},
		{
			// PRIME: both drivers, and lib32-mesa exactly once.
			name: "hybrid AMD + Intel",
			devices: []GPUDevice{
				{Vendor: 0x1002, Device: 0x73ff},
				{Vendor: 0x8086, Device: 0x7d55},
			},
			want: []string{"vulkan-radeon", "lib32-vulkan-radeon", "lib32-mesa", "vulkan-intel", "lib32-vulkan-intel"},
		},
		{
			// Hybrid NVIDIA (Turing iGPU-less laptop style) + Intel.
			name: "hybrid Intel + NVIDIA",
			devices: []GPUDevice{
				{Vendor: 0x8086, Device: 0x7d55},
				{Vendor: 0x10de, Device: 0x2a05},
			},
			want: []string{"vulkan-intel", "lib32-vulkan-intel", "lib32-mesa", "nvidia-open-lts", "nvidia-utils", "lib32-nvidia-utils"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectDriverPackages(tt.devices)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectDriverPackages() = %v, want %v", got, tt.want)
			}
			// The invariant behind the whole feature: every set answers both
			// Vulkan virtuals, so pacman's provider question never appears.
			if len(got) == 0 {
				t.Fatal("the set is empty; the provider question would be reachable")
			}
		})
	}
}

func TestDriverCandidatePackages_CoversEverySet(t *testing.T) {
	for _, set := range [][]string{
		amdDriverPackages,
		intelDriverPackages,
		nvidiaDriverPackages,
		nouveauDriverPackages,
		fallbackDriverPackages,
	} {
		for _, name := range set {
			found := false
			for _, c := range DriverCandidatePackages {
				if c == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%q is selectable but not in DriverCandidatePackages; the offline mirror could not resolve it", name)
			}
		}
	}
	seen := map[string]bool{}
	for _, name := range DriverCandidatePackages {
		if name == "" {
			t.Fatal("DriverCandidatePackages contains an empty entry")
		}
		if seen[name] {
			t.Errorf("DriverCandidatePackages contains duplicate %q", name)
		}
		seen[name] = true
	}
}

// TestNvidiaTuringFloorAgainstPciIDs documents where the floor came from: the
// highest device id below it in pci.ids is Volta (1df6) and the first one at
// or above it is Turing (1e02). The constant is the boundary itself.
func TestNvidiaTuringFloor(t *testing.T) {
	if got := driverSetFor(GPUDevice{Vendor: 0x10de, Device: nvidiaTuringDeviceFloor - 1}); !reflect.DeepEqual(got, nouveauDriverPackages) {
		t.Errorf("device just below the floor got %v, want nouveau", got)
	}
	if got := driverSetFor(GPUDevice{Vendor: 0x10de, Device: nvidiaTuringDeviceFloor}); !reflect.DeepEqual(got, nvidiaDriverPackages) {
		t.Errorf("device at the floor got %v, want nvidia-open", got)
	}
}

// --- the decision ------------------------------------------------------------

func TestChooseDriverPackages_RecordsAndReuses(t *testing.T) {
	ctx := &shared.InstallContext{GPUDriver: DriverOverrideAMD}
	first := ChooseDriverPackages(ctx)
	if !reflect.DeepEqual(first, amdDriverPackages) {
		t.Fatalf("override amd = %v, want %v", first, amdDriverPackages)
	}
	if !reflect.DeepEqual(ctx.DriverPackages, amdDriverPackages) {
		t.Errorf("the choice was not recorded on the context: %v", ctx.DriverPackages)
	}

	// A recorded set is reused without re-detecting — even a context whose
	// override says otherwise keeps what the first run chose.
	ctx2 := &shared.InstallContext{DriverPackages: []string{"vulkan-radeon"}}
	if got := ChooseDriverPackages(ctx2); !reflect.DeepEqual(got, []string{"vulkan-radeon"}) {
		t.Errorf("reused set = %v, want the recorded one", got)
	}
}

func TestChooseDriverPackages_Overrides(t *testing.T) {
	tests := map[string][]string{
		DriverOverrideAMD:    amdDriverPackages,
		DriverOverrideIntel:  intelDriverPackages,
		DriverOverrideNvidia: nvidiaDriverPackages,
		DriverOverrideMesa:   fallbackDriverPackages,
	}
	for override, want := range tests {
		ctx := &shared.InstallContext{GPUDriver: override}
		if got := ChooseDriverPackages(ctx); !reflect.DeepEqual(got, want) {
			t.Errorf("--gpu-driver %s = %v, want %v", override, got, want)
		}
	}
}

func TestChooseDriverPackages_FakeGPU(t *testing.T) {
	// The FAKE_GPU knob feeds synthetic devices through the same selection as
	// real detection — the Playwright harness has no PCI bus to read.
	for kind, want := range map[string][]string{
		"amd":    amdDriverPackages,
		"intel":  intelDriverPackages,
		"nvidia": nvidiaDriverPackages,
		"mesa":   fallbackDriverPackages,
	} {
		t.Setenv("FAKE_GPU", kind)
		ctx := &shared.InstallContext{}
		got := ChooseDriverPackages(ctx)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("FAKE_GPU=%s = %v, want %v", kind, got, want)
		}
	}
}

func TestChooseDriverPackages_DryRunUsesFixedSet(t *testing.T) {
	t.Setenv("AELUM_DRY_RUN", "1")
	ctx := &shared.InstallContext{}
	if got := ChooseDriverPackages(ctx); !reflect.DeepEqual(got, fallbackDriverPackages) {
		t.Errorf("dry-run set = %v, want the fixed fallback set", got)
	}
}

func TestChooseDriverPackages_AutoNeverEmpty(t *testing.T) {
	// Real detection against the host's bus: the result depends on the host,
	// the guarantee does not — it is never empty.
	ctx := &shared.InstallContext{}
	if got := ChooseDriverPackages(ctx); len(got) == 0 {
		t.Fatal("auto detection produced an empty set")
	}
}

func TestValidDriverOverride(t *testing.T) {
	for _, ok := range []string{"auto", "amd", "intel", "nvidia", "mesa"} {
		if !ValidDriverOverride(ok) {
			t.Errorf("ValidDriverOverride(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "nvidia-open", "AMD", "radeon"} {
		if ValidDriverOverride(bad) {
			t.Errorf("ValidDriverOverride(%q) = true, want false", bad)
		}
	}
}

// --- the log line ------------------------------------------------------------

// recordingLogger captures Info lines so the log shape can be asserted.
type recordingLogger struct {
	shared.Logger
	infos []string
}

func (l *recordingLogger) Info(format string, args ...interface{}) {
	l.infos = append(l.infos, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func TestChooseDriverPackages_LogsTheDecision(t *testing.T) {
	t.Setenv("FAKE_GPU", "amd")
	logger := &recordingLogger{}
	ctx := &shared.InstallContext{Logger: logger}
	ChooseDriverPackages(ctx)

	if len(logger.infos) != 1 || !strings.HasPrefix(logger.infos[0], "GPU 0x1002:73ff → ") {
		t.Fatalf("log = %v, want one GPU line", logger.infos)
	}
	if !strings.Contains(logger.infos[0], "vulkan-radeon lib32-vulkan-radeon lib32-mesa") {
		t.Errorf("GPU line does not name the set: %q", logger.infos[0])
	}
}

func TestChooseDriverPackages_LogsFallback(t *testing.T) {
	t.Setenv("FAKE_GPU", "unrecognised-kind") // no synthetic devices → the empty-bus path
	logger := &recordingLogger{}
	ctx := &shared.InstallContext{Logger: logger}
	ChooseDriverPackages(ctx)

	if len(logger.infos) != 1 || !strings.HasPrefix(logger.infos[0], "GPU none recognised → ") {
		t.Fatalf("log = %v, want the none-recognised line", logger.infos)
	}
}
