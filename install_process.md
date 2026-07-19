# Aesthetic OS Installer - Installation Process

## Architecture

The installer uses a two-binary architecture:
- **aesthetic-os-installer**: Main GUI application (runs as normal user)
- **root-helper**: Privileged helper binary (runs with sudo)

### Why This Architecture?
- GTK/Wails panics when running as root on Linux
- GUI must run as normal user
- Privileged operations delegated to root-helper via sudo
- Passwordless sudo configured via `/etc/sudoers.d/90-root-helper`

## Partition Layout

Standard 2-partition layout:
```
/dev/sda1 (or /dev/vda1): 128MB EFI (FAT32) - mounted at /mnt/boot/efi
/dev/sda2 (or /dev/vda2): Rest of disk (ext4) - mounted at /mnt
```

## Installation Steps Order

1. **FormatDisk**: Create partitions and format
2. **MountFilesystem**: Mount root and EFI partitions
3. **InstallBase**: pacstrap base system + packages (runs once)
4. **GenerateFstab**: Generate /etc/fstab from genfstab
5. **ConfigureBootloader**: Setup systemd-boot
6. **SetupUsers**: Create user accounts
7. **InstallPackages**: Additional packages via arch-chroot
8. **ConfigureSystem**: System configuration (hostname, timezone, locale, services)

## Required Packages (Live ISO)

```bash
util-linux      # mount, lsblk, blkid
e2fsprogs       # mkfs.ext4
dosfstools      # mkfs.fat (FAT32)
gdisk           # sgdisk for partitioning
systemd         # bootctl for systemd-boot
```

## Configuration Files

### Sudoers Drop-in
**Location:** `/etc/sudoers.d/90-root-helper`
```
%wheel ALL=(ALL) NOPASSWD: /shared/root-helper
```
Allow passwordless sudo for root-helper binary.

### Build Script
**Location:** `build.sh`
- Builds frontend with npm
- Builds main binary with wails
- Builds root-helper binary
- Copies both to `/home/power007/Projects/shared/`

## Key Implementation Details

### Finding the Helper
`findHelper()` in `backend/installation.go`:
1. First checks `/shared/root-helper` (testing)
2. Falls back to next to main executable (production)

### Step Function Signature
All steps accept: `(diskID, helper, username, password, language string) error`
- `diskID`: User-selected disk (e.g., `/dev/vda`)
- `helper`: Path to root-helper binary
- Others: User account data

### Privileged Operations
All steps use `sudo helper <command>` pattern:
- `parted`: Create partitions
- `format`: Format partitions (FAT32 for EFI, ext4 for root)
- `mount`: Mount partitions
- `pacstrap`: Install base system
- `bootctl-install`: Install systemd-boot
- `useradd`: Create users
- `run`: Execute arbitrary commands (for arch-chroot, etc.)

## Known Issues & Solutions

### Mount Already Mounted Error
**Problem:** Partition already mounted from previous run
**Solution:** Force unmount before mounting:
```go
exec.Command("sudo", helper, "umount", "-f", "/mnt").Run()
```

### Disk Type Detection
**Problem:** Different systems use different disk naming
**Solution:** Try virtio (`/dev/vda`) first, fallback to SATA (`/dev/sda`)

### EFI Partition Mount Point
**Problem:** `/mnt/boot/efi` doesn't exist
**Solution:** Create directory before mounting:
```go
exec.Command("sudo", helper, "run", "mkdir", "-p", "/mnt/boot/efi")
```

## systemd-boot Configuration

### Boot Entry Location
`/boot/efi/boot/loader/entries/aesthetic-os.conf`

### Boot Entry Template
```
title   Aesthetic OS
linux   /vmlinuz-linux
initrd  /initramfs-linux.img
options root=UUID=<root_partition_uuid> rw
```

### Finding Root UUID
Uses `blkid -s UUID -o value /dev/vda2` to get root partition UUID.

## Testing Workflow

1. Build both binaries:
   ```bash
   ./build.sh
   ```

2. Copy to test location:
   ```bash
   cp build/bin/aesthetic-os-installer /shared/
   cp build/bin/root-helper /shared/
   ```

3. Configure sudoers (on live ISO):
   ```bash
   echo "%wheel ALL=(ALL) NOPASSWD: /shared/root-helper" > /etc/sudoers.d/90-root-helper
   ```

4. Run installer (as normal user, NOT sudo):
   ```bash
   cd /shared
   ./aesthetic-os-installer
   ```

## Future Improvements

- [ ] Add unmount step at end of installation
- [ ] Implement ConfigureSystem (hostname, timezone, locale)
- [ ] Add progress tracking for each sub-step
- [ ] Support for UEFI vs BIOS detection
- [ ] Add rollback capability if installation fails
- [ ] Support for LUKS encryption
- [ ] Support for LVM
- [ ] Add network configuration step

## CLI Installer

An automatic CLI installer is available at `cmd/installer/`:

```bash
# Build
./build.sh

# Run (as normal user, NOT root)
./build/bin/installer --install /dev/vda [--dry-run]
```

The CLI installer:
- Uses same backend as GUI (single source of truth)
- Automatic installation with no interactive prompts
- Dry-run mode for testing without making changes
- Detects root-helper automatically
- Runs all installation steps via root-helper

## File Locations

```
aesthetic-os-installer/
├── backend/
│   ├── installation.go       # Installation orchestration (GUI)
│   ├── app.go                # Main app structure
│   └── steps/                # Individual installation steps
│       ├── format_disk.go
│       ├── mount_filesystem.go
│       ├── install_base.go
│       ├── generate_fstab.go
│       ├── configure_bootloader.go
│       ├── setup_users.go
│       ├── install_packages.go
│       └── configure_system.go
├── cmd/
│   ├── root-helper/          # Privileged helper binary
│   │   └── main.go
│   └── installer/            # CLI installer
│       └── main.go
├── build.sh                  # Build script
└── install_process.md        # This file
```
