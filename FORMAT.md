# archinstall Disk Formatting — Detailed Plan (ext4 & btrfs)

## Architecture Overview

The formatting pipeline in archinstall is split across these key files:

| File | Role |
|------|------|
| `lib/disk/filesystem.py` | `FilesystemHandler` — orchestrates the full format workflow |
| `lib/disk/device_handler.py` | `DeviceHandler` — low-level `format()`, `partition()`, `create_btrfs_volumes()` |
| `lib/models/device.py` | `FilesystemType` enum, `PartitionModification`, `DiskLayoutConfiguration` |
| `lib/disk/subvolume_menu.py` | btrfs subvolume management UI |
| `scripts/guided.py` | Entry point — calls `FilesystemHandler.perform_filesystem_operations()` |

---

## Phase 1: Partitioning (before formatting)

### Step 1.1 — Wipe & create partition table:

```bash
# Wipe existing signatures (MBR/GPT, filesystem, LVM, etc.)
wipefs --all /dev/sdX

# Create GPT partition table (UEFI) or MBR (BIOS)
# Uses libparted via Python: freshDisk(device, 'gpt' or 'msdos')
# No direct CLI equivalent — parted does this internally
```

### Step 1.2 — Create partitions with parted:

```python
# Python via libparted (what archinstall actually does):
# disk = freshDisk('/dev/sdX', 'gpt')
# partition = Partition(disk, PARTITION_PRIMARY,
#     Geometry(device, start_sector, length_sectors),
#     FileSystem(type='ext4' or 'btrfs', geometry=...))
# disk.addPartition(partition)
```

### Step 1.3 — Set partition type GUIDs (GPT only):

```bash
# For root partition (X86-64 Linux):
# t → 4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709
# For home partition flag:
# t → 4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709 + flag linux-home
```

### Step 1.4 — Wipe signatures on new partitions:

```bash
wipefs --all /dev/sdX1
wipefs --all /dev/sdX2
# ...
```

---

## Phase 2: Formatting — ext4

**Command used by archinstall** (from `device_handler.py` → `format()` method):

```bash
mkfs.ext4 -F /dev/sdX1
```

| Parameter | Meaning |
|-----------|---------|
| `mkfs.ext4` | The mkfs command |
| `-F` | Force create (overwrites existing signatures) |

**No additional options** — archinstall uses the bare minimum. The default ext4 options (journal size, inode count, etc.) are all system defaults.

---

## Phase 3: Formatting — btrfs

### Step 3.1 — Create filesystem:

```bash
mkfs.btrfs -f /dev/sdX1
```

| Parameter | Meaning |
|-----------|---------|
| `mkfs.btrfs` | The mkfs command |
| `-f` | Force overwrite (btrfs-specific flag) |

### Step 3.2 — Create default subvolumes:

```bash
# Mount the btrfs partition
mount /dev/sdX1 /mnt/arch_btrfs

# Create default root subvolume
btrfs subvolume create -p /mnt/arch_btrfs/@

# Create @home subvolume
btrfs subvolume create -p /mnt/arch_btrfs/@home

# Create @var_log subvolume (optional, for snapshots)
btrfs subvolume create -p /mnt/arch_btrfs/@var_log

# Unmount
umount /mnt/arch_btrfs
```

### Step 3.3 — Apply mount options (if configured via `BtrfsMountOption`):

```bash
# For nodatacow subvolumes:
chattr +C /mnt/@home

# For compressed subvolumes:
chattr +c /mnt/@home
```

**Mount options enum values** (from `FilesystemType.BTRFS` configuration):
```python
BtrfsMountOption.compress  → 'compress=zstd'
BtrfsMountOption.nodatacow → 'nodatacow'
```

---

## Phase 4: Full Automated Script (ext4 example)

```bash
#!/bin/bash
# === EXT4 Partitioning & Formatting ===

DISK="/dev/sdX"

# 1. Wipe existing signatures
wipefs --all "$DISK"

# 2. Create GPT partition table
parted -s "$DISK" mklabel gpt

# 3. Create EFI partition (512M)
parted -s "$DISK" mkpart EFI 1MiB 513MiB
parted -s "$DISK" set 1 esp on

# 4. Create root partition (rest of disk)
parted -s "$DISK" mkpart root ext4 513MiB 100%

# 5. Set GPT type GUIDs
# ESP = C12A7328-F81F-11D2-BA4B-00A0C93EC93B
# Root = 4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709
sgdisk -t 1:C12A7328-F81F-11D2-BA4B-00A0C93EC93B "$DISK"
sgdisk -t 2:4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709 "$DISK"

# 6. Wipe signatures on new partitions
wipefs --all "${DISK}1"
wipefs --all "${DISK}2"

# 7. Format EFI (FAT32)
mkfs.vfat -F 32 "${DISK}1"

# 8. Format root (ext4)  ← THIS IS THE KEY COMMAND
mkfs.ext4 -F "${DISK}2"
```

---

## Phase 5: Full Automated Script (btrfs example)

```bash
#!/bin/bash
# === BTRFS Partitioning & Formatting ===

DISK="/dev/sdX"

# 1. Wipe existing signatures
wipefs --all "$DISK"

# 2. Create GPT partition table
parted -s "$DISK" mklabel gpt

# 3. Create EFI partition (512M)
parted -s "$DISK" mkpart EFI 1MiB 513MiB
parted -s "$DISK" set 1 esp on

# 4. Create root partition (rest of disk) — btrfs
parted -s "$DISK" mkpart root 513MiB 100%

# 5. Set GPT type GUIDs
sgdisk -t 1:C12A7328-F81F-11D2-BA4B-00A0C93EC93B "$DISK"
sgdisk -t 2:4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709 "$DISK"

# 6. Wipe signatures on new partitions
wipefs --all "${DISK}1"
wipefs --all "${DISK}2"

# 7. Format EFI (FAT32)
mkfs.vfat -F 32 "${DISK}1"

# 8. Format root (btrfs)  ← THIS IS THE KEY COMMAND
mkfs.btrfs -f "${DISK}2"

# 9. Create default btrfs subvolumes
MOUNT="/mnt/arch_btrfs"
mkdir -p "$MOUNT"
mount "${DISK}2" "$MOUNT"

btrfs subvolume create -p "$MOUNT"/@
btrfs subvolume create -p "$MOUNT"/@home
btrfs subvolume create -p "$MOUNT"/@var_log

# Optional: set nodatacow on @home (for performance)
chattr +C "$MOUNT"/@home

# Optional: set compression on @ (for space savings)
chattr +c "$MOUNT"/@

umount "$MOUNT"
rmdir "$MOUNT"
```

---

## Phase 6: Btrfs Subvolume Defaults (archinstall standard)

archinstall's default btrfs subvolume layout:

| Subvolume | Mount Point | Notes |
|-----------|-------------|-------|
| `@` | `/` | Default root |
| `@home` | `/home` | User data |
| `@var_log` | `/var/log` | For snapshot support |

These are created via `btrfs subvolume create -p` (parent reference for efficiency) and optional `chattr` flags for `nodatacow` / `compress=zstd`.

---

## Key Differences: ext4 vs btrfs in archinstall

| Aspect | ext4 | btrfs |
|--------|------|-------|
| `mkfs` command | `mkfs.ext4 -F` | `mkfs.btrfs -f` |
| Force flag | `-F` | `-f` |
| Subvolumes | Not applicable | Required (default: `@`, `@home`, `@var_log`) |
| Mount options | Standard | Optional `compress=zstd`, `nodatacow` |
| Extra post-format step | None | `btrfs subvolume create` + optional `chattr` |
| Installation package needed | `e2fsprogs` (always present) | `btrfs-progs` |

---

## Summary: The Core Commands

For **ext4**, the only formatting command is:
```bash
mkfs.ext4 -F /dev/sdX1
```

For **btrfs**, the formatting + subvolume commands are:
```bash
mkfs.btrfs -f /dev/sdX1
mount /dev/sdX1 /mnt/arch_btrfs
btrfs subvolume create -p /mnt/arch_btrfs/@
btrfs subvolume create -p /mnt/arch_btrfs/@home
btrfs subvolume create -p /mnt/arch_btrfs/@var_log
chattr +C /mnt/arch_btrfs/@home   # optional
chattr +c /mnt/arch_btrfs/@       # optional
umount /mnt/arch_btrfs
```
