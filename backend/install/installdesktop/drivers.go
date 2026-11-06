package installdesktop

// GPU driver selection.
//
// steam and friends depend on the virtuals vulkan-driver and lib32-vulkan-driver,
// never on a driver by name — and a virtual with an already-installed provider is
// never asked about. So an install that carries the right driver set for its own
// hardware never shows the "choose between 13 providers" question, in the package
// manager or in a terminal. With no provider installed, pacman silently takes
// provider #1 — lib32-nvidia-utils — which on an AMD or Intel machine is ~430
// MiB of wrong-hardware userspace.
//
// This file picks the set from the PCI bus: detectGPUs reads sysfs,
// selectDriverPackages is the pure table, and ChooseDriverPackages is what the
// desktop step calls at its start. The chosen set is recorded on
// ctx.DriverPackages so the verify step checks exactly what was installed.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aelum-installer/backend/shared"
)

// PCI vendor IDs of the vendors the table claims (pci.ids).
const (
	pciVendorAMD    = 0x1002
	pciVendorIntel  = 0x8086
	pciVendorNvidia = 0x10de
	pciVendorVirtio = 0x1af4
	pciVendorVMware = 0x15ad
)

// nvidiaTuringDeviceFloor is the lowest NVIDIA PCI device id the open kernel
// module supports: the open module covers Turing and newer, and the first Turing
// id is 1e02 (TU102 [TITAN RTX]) — checked against pci.ids. Everything older
// (Pascal tops out at 1d56, Volta at 1df6) sits below the floor and gets
// nouveau, because current Arch has no proprietary nvidia module at all.
// The floor is a heuristic, not a list: the professional 0x10xx/0x11xx ids are
// the known blind spot, and --gpu-driver nvidia is the escape hatch.
const nvidiaTuringDeviceFloor = 0x1E00

// Display-controller PCI base classes: 03/00 VGA, 03/01 XGA, 03/02 3D
// controller. A device with no driver bound still has class/vendor/device,
// which is why detection needs no driver and works in any live session.
const (
	pciClassVGA = 0x0300
	pciClassXGA = 0x0301
	pciClass3D  = 0x0302
)

// DriverOverride* are the accepted --gpu-driver values. "auto" detects; the
// rest force one row of the table. The QEMU matrix uses the forced values to
// exercise the non-VM paths on hardware it does not have.
const (
	DriverOverrideAuto   = "auto"
	DriverOverrideAMD    = "amd"
	DriverOverrideIntel  = "intel"
	DriverOverrideNvidia = "nvidia"
	DriverOverrideMesa   = "mesa"
)

// ValidDriverOverride reports whether s is an accepted --gpu-driver value.
func ValidDriverOverride(s string) bool {
	switch s {
	case DriverOverrideAuto, DriverOverrideAMD, DriverOverrideIntel, DriverOverrideNvidia, DriverOverrideMesa:
		return true
	}
	return false
}

// The driver sets. Every set provides both the 64-bit and the lib32 Vulkan
// virtuals — that is the invariant, and what makes the provider question
// unreachable on every machine. The lib32-* names resolve because the ISO
// build enables [multilib] in its resolution config (docker/scripts/
// build-iso.sh); the target gets [multilib] from configure/pacman_repo.go.
var (
	amdDriverPackages     = []string{"vulkan-radeon", "lib32-vulkan-radeon", "lib32-mesa"}
	intelDriverPackages   = []string{"vulkan-intel", "lib32-vulkan-intel", "lib32-mesa"}
	nvidiaDriverPackages  = []string{shared.NvidiaDriverPackage, "nvidia-utils", "lib32-nvidia-utils"}
	nouveauDriverPackages = []string{"vulkan-nouveau", "lib32-vulkan-nouveau", "lib32-mesa"}
	// fallbackDriverPackages is the set for VMs and for hardware the table
	// does not claim: the Mesa software driver plus virtio-gpu (the
	// paravirtual device every QEMU/KVM guest has).
	fallbackDriverPackages = []string{"vulkan-swrast", "lib32-vulkan-swrast", "vulkan-virtio", "lib32-mesa"}
)

// DriverCandidatePackages is the union of everything the table can produce.
// backend/scripts/pkg-list.go prints it into the ISO build's package list:
// a dynamically chosen name that no list carries cannot be installed offline,
// so every name detection might pick must be in the mirror.
var DriverCandidatePackages = unionStrings(
	amdDriverPackages,
	intelDriverPackages,
	nvidiaDriverPackages,
	nouveauDriverPackages,
	fallbackDriverPackages,
)

// GPUDevice is one display controller found on the PCI bus.
type GPUDevice struct {
	Slot   string // sysfs device directory name, e.g. "0000:01:00.0"
	Class  uint32 // display class: pciClassVGA, pciClassXGA or pciClass3D
	Vendor uint16
	Device uint16
	Driver string // bound driver name, e.g. "amdgpu"; empty when none is bound
}

// detectGPUs walks <sysRoot>/bus/pci/devices/* and returns every display
// controller it finds, bound or not. It reads plain sysfs files — no new
// dependency, no root needed — and takes the sysfs root as a parameter so
// tests can hand it a temp-dir tree, the same shape as configure's
// preferredDisplayMode(drmRoot).
func detectGPUs(sysRoot string) []GPUDevice {
	dirs, err := filepath.Glob(filepath.Join(sysRoot, "bus", "pci", "devices", "*"))
	if err != nil {
		return nil
	}

	var devices []GPUDevice
	for _, dir := range dirs {
		// The class file is "0x030000": base class, subclass, programming
		// interface. Only the first three nibbles identify a display device.
		class, ok := parseSysfsHex(readSysfsFile(dir, "class"))
		if !ok {
			continue
		}
		class >>= 8
		if class != pciClassVGA && class != pciClassXGA && class != pciClass3D {
			continue
		}

		vendor, ok1 := parseSysfsHex(readSysfsFile(dir, "vendor"))
		device, ok2 := parseSysfsHex(readSysfsFile(dir, "device"))
		if !ok1 || !ok2 {
			continue
		}

		gpu := GPUDevice{
			Slot:   filepath.Base(dir),
			Class:  class,
			Vendor: uint16(vendor),
			Device: uint16(device),
		}
		// The driver symlink is absent when nothing is bound; its target's
		// base name is the driver.
		if target, err := os.Readlink(filepath.Join(dir, "driver")); err == nil {
			gpu.Driver = filepath.Base(target)
		}
		devices = append(devices, gpu)
	}
	return devices
}

// readSysfsFile reads one sysfs attribute, returning "" when it is missing or
// unreadable — sysfs is full of files a given kernel does not create.
func readSysfsFile(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// parseSysfsHex parses a sysfs "0x…" value.
func parseSysfsHex(s string) (uint32, bool) {
	v, err := strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

// driverSetFor maps one detected device to its driver set, or nil when the
// table does not claim its vendor.
func driverSetFor(d GPUDevice) []string {
	switch d.Vendor {
	case pciVendorAMD:
		return amdDriverPackages
	case pciVendorIntel:
		return intelDriverPackages
	case pciVendorNvidia:
		if d.Device >= nvidiaTuringDeviceFloor {
			return nvidiaDriverPackages
		}
		// Pascal, Maxwell, Kepler, Volta: no proprietary module exists in
		// current Arch, so the open-source driver is the only option.
		return nouveauDriverPackages
	case pciVendorVirtio, pciVendorVMware:
		return fallbackDriverPackages
	default:
		return nil
	}
}

// selectDriverPackages is the pure half of the decision: the union of the sets
// for every detected device. Hybrid graphics gets both sets — PRIME needs both
// drivers. The result is never empty: a machine with no display device at all
// gets the fallback set like a VM does, and that guarantee is what makes the
// provider question impossible to reach on any install.
func selectDriverPackages(devices []GPUDevice) []string {
	var out []string
	for _, d := range devices {
		out = unionStrings(out, driverSetFor(d))
	}
	if len(out) == 0 {
		return append([]string{}, fallbackDriverPackages...)
	}
	return out
}

// unionStrings concatenates the lists, dropping duplicates and keeping first
// occurrence order. It always returns a fresh slice, so a package-level set
// can never be appended into by a caller.
func unionStrings(lists ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, list := range lists {
		for _, s := range list {
			if seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ChooseDriverPackages decides the GPU driver set for this install, records it
// on ctx.DriverPackages, and returns it. A set already recorded (a re-run of
// the step, or a test) is reused unchanged — detection never runs twice.
func ChooseDriverPackages(ctx *shared.InstallContext) []string {
	if len(ctx.DriverPackages) > 0 {
		return ctx.DriverPackages
	}

	var packages []string
	switch ctx.GPUDriver {
	case DriverOverrideAMD:
		packages = append([]string{}, amdDriverPackages...)
	case DriverOverrideIntel:
		packages = append([]string{}, intelDriverPackages...)
	case DriverOverrideNvidia:
		packages = append([]string{}, nvidiaDriverPackages...)
	case DriverOverrideMesa:
		packages = append([]string{}, fallbackDriverPackages...)
	default: // "" or DriverOverrideAuto
		packages = detectAndSelect(ctx)
	}

	ctx.DriverPackages = packages
	return packages
}

// detectAndSelect runs the detection path, with the same synthetic-data knobs
// as the disk code: FAKE_GPU= names a vendor for the Playwright harness, and a
// dry run uses the fixed fallback set instead of reading the host's bus.
func detectAndSelect(ctx *shared.InstallContext) []string {
	if fake := os.Getenv("FAKE_GPU"); fake != "" {
		devices := syntheticGPUs(fake)
		packages := selectDriverPackages(devices)
		logGPUs(ctx, devices, packages)
		return packages
	}
	if os.Getenv("AELUM_DRY_RUN") == "1" {
		logGPUs(ctx, nil, append([]string{}, fallbackDriverPackages...))
		return append([]string{}, fallbackDriverPackages...)
	}

	devices := detectGPUs(shared.SysfsRoot)
	packages := selectDriverPackages(devices)
	logGPUs(ctx, devices, packages)
	return packages
}

// syntheticGPUs is the FAKE_GPU= knob, mirroring FAKE_DISKS: the harness names
// a vendor and gets the devices a machine of that kind would have, fed through
// the same selection as real detection.
func syntheticGPUs(kind string) []GPUDevice {
	switch kind {
	case DriverOverrideAMD:
		return []GPUDevice{{Slot: "0000:00:01.0", Class: pciClassVGA, Vendor: pciVendorAMD, Device: 0x73FF, Driver: "amdgpu"}}
	case DriverOverrideIntel:
		return []GPUDevice{{Slot: "0000:00:02.0", Class: pciClassVGA, Vendor: pciVendorIntel, Device: 0x7D55, Driver: "i915"}}
	case DriverOverrideNvidia:
		return []GPUDevice{{Slot: "0000:00:04.0", Class: pciClass3D, Vendor: pciVendorNvidia, Device: 0x2A05, Driver: "nvidia"}}
	case DriverOverrideMesa:
		return []GPUDevice{{Slot: "0000:00:03.0", Class: pciClassVGA, Vendor: pciVendorVirtio, Device: 0x1050, Driver: "virtio-pci"}}
	default:
		return nil
	}
}

// logGPUs records the decision: one line per detected device the table claims,
// in the shape "GPU 0x1002:73ff → vulkan-radeon lib32-vulkan-radeon
// lib32-mesa". When the bus held nothing the table claims — the QEMU guest, or
// no display device at all — a single line names the fallback set instead, and
// a hybrid machine gets both lines plus the union it installs.
func logGPUs(ctx *shared.InstallContext, devices []GPUDevice, packages []string) {
	if ctx.Logger == nil {
		return
	}
	claimed := 0
	for _, d := range devices {
		set := driverSetFor(d)
		if set == nil {
			continue
		}
		claimed++
		ctx.Logger.Info("GPU 0x%04x:%04x → %s", d.Vendor, d.Device, strings.Join(set, " "))
	}
	if claimed == 0 {
		ctx.Logger.Info("GPU none recognised → %s", strings.Join(packages, " "))
		return
	}
	if claimed > 1 {
		ctx.Logger.Info("GPU driver set (hybrid) → %s", strings.Join(packages, " "))
	}
}
