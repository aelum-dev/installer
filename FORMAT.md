# Disk Formatting — Reference Guide

> **Purpose:** This file contains everything needed to understand, modify, or debug the disk formatting pipeline in AestheticOS Installer. Point this to a new session and it will have full context.

---

## Project Context

### Files Involved in Disk Formatting

| File | Role |
|------|------|
| `backend/steps/format_disk.go` | **Main orchestrator** — `FormatDisk()` function, partition creation, formatting, btrfs subvolumes, UUID discovery |
| `backend/helper/main.go` | **Privileged helper** — `cmdFormat`, `cmdBtrfsSubvolCreate`, `cmdBtrfsSubvolSetDefault`, `cmdBtrfsSetCompression`, `cmdUdevSettle`, `cmdParted` |
| `backend/shared/layouts.go` | `PartitionLayout`, `LayoutPartition`, `GetLayout()` — defines EFI + root partition schemes |
| `backend/shared/types.go` | `PartitionInfo`, `PartitionEntry`, `DiskInfo` — data structures |
| `backend/steps/context.go` | `InstallContext` — shared state passed between steps |
| `backend/steps/cleanup.go` | `CleanupMounts()`, `FormatDiskErrorCleanup()` — unmount helpers |
| `backend/steps/mount_filesystem.go` | `MountFilesystem()` — mounts root/EFI/subvolumes after formatting |
| `backend/core/installer.go` | `Installer.Install()` — calls `FormatDisk` as step 1 of 7 |

### Architecture: Helper Binary Pattern

All privileged operations go through `aestheticos-helper` (runs via sudo whitelist):

```
aestheticos-installer (GUI/CLI)
    ──sudo──> aestheticos-helper <command> [args...]
                    │
                    ├── cmdParted(disk)          → sgdisk -Z disk
                    ├── cmdFormat(part, fstype)  → wipefs + mkfs.*
                    ├── cmdMount / cmdUmount     → mount / umount
                    ├── cmdBtrfsSubvolCreate     → btrfs subvolume create -p
                    ├── cmdBtrfsSubvolSetDefault → btrfs subvolume set-default
                    ├── cmdBtrfsSetCompression   → btrfs property set compression
                    └── cmdUdevSettle            → udevadm settle
```

**Calling convention from Go:**
```go
cmd := exec.Command("sudo", ctx.Helper, "format", "/dev/sda2", "ext4")
cmd := exec.Command("sudo", ctx.Helper, "run", "sgdisk", "-Z", "/dev/sda")
cmd := exec.Command("sudo", ctx.Helper, "udev-settle")
```

The `run` subcommand wraps arbitrary root commands: `helper run <cmd> [args...]`.

---

## The Full FormatDisk Flow (format_disk.go)

```
FormatDisk(ctx)
  │
  ├─ 1. Validate: diskID + filesystem (ext4|btrfs) + layout name
  ├─ 2. Get layout from shared.GetLayout(layoutName, fsType)
  ├─ 3. unmountAllPartitions(diskID)     ← archinstall: umount_all_existing
  ├─ 4. FormatDiskErrorCleanup()         ← unmount /mnt, /mnt/boot
  ├─ 5. helper parted <disk>             ← sgdisk -Z (clear GPT data)
  ├─ 6. refreshPartitionTable()          ← blockdev --rereadpt + 500ms sleep
  ├─ 7. For each partition:
  │     sgdisk -n <num>:2048:+<size>M    ← create partition
  │     sgdisk -t <num>:<GUID>           ← set GPT type GUID
  │     sgdisk -c <num>:<name>           ← set partition name
  ├─ 8. refreshPartitionTable()          ← re-read after partitioning
  ├─ 9. For each partition:
  │     wipefs --all -f <partPath>       ← archinstall: wipe signatures
  ├─ 10. sync
  ├─ 11. helper format <partPath> <fstype>  ← wipefs + mkfs.*
  ├─ 12. sync
  ├─ 13. helper udev-settle             ← archinstall: udev_sync()
  ├─ 14. refreshPartitionTable()
  ├─ 15. [if btrfs] create subvolumes   ← see btrfs section below
  ├─ 16. shared.GetPartitions(diskID)   ← lsblk JSON
  ├─ 17. readPartitionUUID() via blkid  ← UUID for each partition
  └─ 18. Build PartitionInfo, store in ctx.Partitions
```

---

## GPT Partition Type GUIDs (Constants)

Defined in `format_disk.go`:

```go
const (
    gptESP             = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
    gptLinuxRootX86_64 = "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709"
)
```

| Partition | GUID | sgdisk arg |
|-----------|------|------------|
| EFI (ESP) | `C12A7328-F81F-11D2-BA4B-00A0C93EC93B` | `-t 1:C12A7328-F81F-11D2-BA4B-00A0C93EC93B` |
| Root (ext4/btrfs) | `4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709` | `-t 2:4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709` |

**Mapped from archinstall's `PartitionGUID` enum** (`lib/models/device.py`).
Our project uses x86-64 specific GUIDs (more specific than generic `8300`).

---

## Formatting Commands (helper/main.go → cmdFormat)

### ext4
```bash
wipefs --all -f /dev/sdX2          # wipe signatures (inside cmdFormat)
mkfs.ext4 -F -L aestheticos /dev/sdX2   # force create + label
```
- `-F` = force (archinstall: `mkfs.ext4 -F`)
- `-L aestheticos` = volume label (our addition, not in archinstall)
- No other options — system defaults for journal, inodes, etc.

### btrfs
```bash
wipefs --all -f /dev/sdX2          # wipe signatures
mkfs.btrfs -f -L aestheticos /dev/sdX2   # force overwrite + label
```
- `-f` = force overwrite (archinstall: `mkfs.btrfs -f`)
- `-L aestheticos` = volume label

### vfat (EFI)
```bash
wipefs --all -f /dev/sdX1
mkfs.fat -F 32 -n EFI /dev/sdX1    # 32-bit FAT + name "EFI"
```

---

## btrfs Subvolume Setup (after formatting)

### What We Create

| Subvolume | Mount Point | Properties |
|-----------|-------------|------------|
| `@` | `/` (default) | `compression=zstd` (persistent property) |
| `@home` | `/home` | `nodatacow` (chattr +C) + `compression=zstd` |

### Commands Executed

```bash
# 1. Mount top-level subvolume (subvolid=5) to temp location
mount -o subvolid=5 /dev/sdX2 /tmp/btrfs-setup

# 2. Create subvolumes with -p (parent reference)
btrfs subvolume create -p /tmp/btrfs-setup/@
btrfs subvolume create -p /tmp/btrfs-setup/@home

# 3. Set @ as default subvolume
btrfs subvolume set-default <id_of_@> /tmp/btrfs-setup

# 4. Set persistent compression via btrfs property
btrfs property set /tmp/btrfs-setup/@ compression zstd
btrfs property set /tmp/btrfs-setup/@home compression zstd

# 5. Set nodatacow on @home (chattr +C)
chattr +C /tmp/btrfs-setup/@home

# 6. Unmount
umount -l /tmp/btrfs-setup
rmdir /tmp/btrfs-setup
```

### Mapped from archinstall

| Our Command | archinstall Equivalent | File |
|-------------|----------------------|------|
| `btrfs subvolume create -p` | `btrfs subvolume create -p` | `device_handler.py` → `create_btrfs_volumes()` |
| `btrfs property set compression zstd` | `chattr +c` | `device_handler.py` → `create_lvm_btrfs_subvolumes()` |
| `chattr +C` on @home | `chattr +C` | `device_handler.py` → `create_lvm_btrfs_subvolumes()` |
| `btrfs subvolume set-default` | (implicit via default subvol) | archinstall sets `@` as default |

**Key difference:** archinstall uses `chattr +c/+C` for compression/nodatacow.
We use `btrfs property set compression zstd` which is the proper btrfs API
and achieves the same persistent result.

---

## Partition Layouts (shared/layouts.go)

### Default Layout (ext4 or btrfs)
```go
Partitions: [
    { Name: "EFI",    SizeMB: 128, Fstype: "vfat",  MountPoint: "/boot" },
    { Name: "Root",   SizeMB: 0,   Fstype: "ext4",  MountPoint: "/" },
]
```
- 128MB EFI (our choice; archinstall uses 512MB in examples)
- Rest of disk for root

### Btrfs Subvolumes Layout
```go
Partitions: [
    { Name: "EFI",    SizeMB: 128, Fstype: "vfat",  MountPoint: "/boot" },
    { Name: "Root",   SizeMB: 0,   Fstype: "btrfs", MountPoint: "/" },
]
// Subvolumes (@, @home) created after formatting, not as separate partitions
```

---

## Helper Functions (format_disk.go)

| Function | Purpose |
|----------|---------|
| `FormatDisk(ctx)` | Main entry point — full partition + format + subvolume pipeline |
| `unmountAllPartitions(disk, helper, logger)` | Discovers all partitions on disk, unmounts each (archinstall: `umount_all_existing`) |
| `derivePartitionPath(diskID, partNum)` | `/dev/sda` → `/dev/sda1`, `/dev/nvme0n1` → `/dev/nvme0n1p1` |
| `findPartitionsByFstype(partitions)` | Matches EFI (vfat) and root (ext4/btrfs) by fstype |
| `findPartitionsByIndex(partitions)` | Fallback: index 0 = EFI, index 1 = root |
| `readPartitionUUID(helper, partition)` | `blkid -s UUID -o value <path>` |
| `refreshPartitionTable(ctx, diskID)` | `blockdev --rereadpt` + 500ms sleep (or partprobe fallback) |
| `FormatDiskErrorCleanup(disk, helper)` | `umount -lR /mnt` + `umount -l <all partitions>` |

---

## Key Design Decisions

1. **128MB EFI** — smaller than archinstall's 512MB examples, sufficient for systemd-boot + kernels
2. **sgdisk over parted CLI** — we use sgdisk for partitioning (GUIDs, names, sectors) while archinstall uses libparted Python API. Same result, different tool.
3. **Helper binary pattern** — all privileged ops through `aestheticos-helper` via sudo, not direct execution
4. **Double wipefs** — we call `wipefs --all -f` both after partitioning (explicit loop) AND inside `cmdFormat`. archinstall only does it once in `partition()`.
5. **`btrfs property set` over `chattr`** — we use the proper btrfs property API for compression; archinstall uses `chattr +c` (works but is ext-specific attribute on btrfs)
6. **Label "aestheticos"** — we add `-L aestheticos` to mkfs commands; archinstall does not set labels
7. **`udevadm settle`** — we added this after formatting (archinstall calls `udev_sync()` which is the same thing)

---

## Known Issues / Future Work (from DISKS_ISSUES.md)

| Issue | Status | Notes |
|-------|--------|-------|
| LUKS/LVM detection | **Not implemented** | DISKS_ISSUES.md flags this as medium priority |
| UUID-based mounting | **Not implemented** | Currently use device paths; UUIDs are more reliable across reboots |
| Pre-flight mount check | **Not needed** | ConfirmationPage serves this purpose |
| Existing EFI reuse | **Not implemented** | Could detect and reuse existing ESP |

---

## Quick Reference: Commands by Filesystem

### ext4 — Minimal
```bash
# Partitioning
sgdisk -Z /dev/sdX                          # clear disk
sgdisk -n 1:2048:+128M -t 1:C12A7328... -c 1:EFI /dev/sdX
sgdisk -n 2:0:0 -t 2:4F68BCE3... -c 2:Root /dev/sdX
wipefs --all -f /dev/sdX1 /dev/sdX2         # wipe signatures

# Formatting
mkfs.fat -F 32 -n EFI /dev/sdX1
mkfs.ext4 -F -L aestheticos /dev/sdX2
```

### btrfs — With Subvolumes
```bash
# Partitioning (same as ext4)
sgdisk -Z /dev/sdX
sgdisk -n 1:2048:+128M -t 1:C12A7328... -c 1:EFI /dev/sdX
sgdisk -n 2:0:0 -t 2:4F68BCE3... -c 2:Root /dev/sdX
wipefs --all -f /dev/sdX1 /dev/sdX2

# Formatting
mkfs.fat -F 32 -n EFI /dev/sdX1
mkfs.btrfs -f -L aestheticos /dev/sdX2

# Subvolumes
mount -o subvolid=5 /dev/sdX2 /tmp/btrfs-setup
btrfs subvolume create -p /tmp/btrfs-setup/@
btrfs subvolume create -p /tmp/btrfs-setup/@home
btrfs subvolume set-default <id> /tmp/btrfs-setup
btrfs property set /tmp/btrfs-setup/@ compression zstd
btrfs property set /tmp/btrfs-setup/@home compression zstd
chattr +C /tmp/btrfs-setup/@home
umount -l /tmp/btrfs-setup
rmdir /tmp/btrfs-setup
```

---

## archinstall Source Reference

If you need to verify a pattern against the original archinstall code:

| Pattern | archinstall File | Function/Class |
|---------|-----------------|----------------|
| Format dispatch (mkfs.* -F/-f) | `lib/disk/device_handler.py` | `DeviceHandler.format()` |
| Partition creation | `lib/disk/device_handler.py` | `DeviceHandler.partition()`, `_setup_partition()` |
| Wipe signatures | `lib/disk/device_handler.py` | `DeviceHandler.partition()` → `wipefs --all` |
| udev sync | `lib/disk/device_handler.py` | `udev_sync()` from `utils.py` |
| Unmount all | `lib/disk/filesystem.py` | `FilesystemHandler.perform_filesystem_operations()` → `device_handler.umount_all_existing()` |
| btrfs subvolumes | `lib/disk/device_handler.py` | `DeviceHandler.create_btrfs_volumes()`, `create_lvm_btrfs_subvolumes()` |
| nodatacow/compression | `lib/disk/device_handler.py` | `create_lvm_btrfs_subvolumes()` → `chattr +C/+c` |
| GPT GUIDs | `lib/models/device.py` | `PartitionGUID` enum |
| FilesystemType enum | `lib/models/device.py` | `FilesystemType` (StrEnum) |
| Partition layout | `lib/default/` | Default partition suggestions |

Repository: `https://github.com/archlinux/archinstall` (branch: `master`)
