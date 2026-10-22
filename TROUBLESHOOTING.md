# Troubleshooting

Common issues and how to resolve them.

## Installation Fails — How to Read the Log

The installer writes a detailed log to:

```
/var/log/aelum-install-<timestamp>.log
```

When the live ISO runs in the QEMU test guest, `/shared` exists and a second
copy is written there too:

```
/shared/aelum-install-<timestamp>.log
```

Both files have the same name; the GUI always shows the `/var/log/...` path.

### Log Format

```
[14:32:01] Starting step: Formatting disk
[14:32:02] Running: sgdisk -Z /dev/sda
[14:32:02] Running: sgdisk -n 1:2048:+512M -t 1:C12A7328... /dev/sda
[14:32:03] ERROR: Failed to wipe /dev/sda1: exit status 1 (Unknown command: wipefs ...)
```

- `[HH:MM:SS]` — timestamp
- `Starting step:` / `Step completed:` — step boundaries
- `Running:` — command output
- `ERROR:` — failure point with command and output

### Quick Log Inspection

```bash
# Last 50 lines
tail -50 "$(ls -t /var/log/aelum-install-*.log | head -1)"

# Search for errors
grep -i "error\|failed" /var/log/aelum-install-*.log

# See only a specific step
sed -n '/Starting step: Formatting disk/,/Starting step: Mounting filesystem/p' /var/log/aelum-install-*.log
```

## Common Failures

### "parted failed" / "sgdisk failed"

**Cause**: Disk is in use, locked, or has an incompatible partition table.

**Fix**:
```bash
# Check what's using the disk
lsblk /dev/sda
mount | grep /dev/sda

# If partitions are mounted, unmount them
sudo umount -R /mnt 2>/dev/null

# Try rollback to clean up
sudo ./aelum-installer-cli --rollback /dev/sda
```

### "pacstrap failed" / "no mirror list available"

**Cause**: Mirrorlist is missing or corrupted for the selected region.

**Fix**:
1. Verify the mirrorlist exists: `ls /etc/pacman.d/mirrorlist-*`
2. Rebuild mirrorlists: `go run ./backend/shared/mirrorlist_gen.go`
3. Rebuild the installer: `./build.sh`

### "conflicting files" when reusing an existing ESP

**Cause**: The reused ESP already contains `memtest86+` binaries from another
installation. pacman refuses to overwrite files it does not own in the target
database:

```text
memtest86+-efi: /mnt/boot/memtest86+/memtest86x64.efi exists in filesystem
```

**Fix**: The installer handles this automatically by renaming the existing
files to `*.aelum-backup` before pacstrap runs, then installing the packaged
version. The old binaries remain on the ESP for recovery. If the install failed
before this fix was applied, remove or rename `/mnt/efi/memtest86+` manually and
retry.

### "bootloader verification failed" (rEFInd)

**Cause**: The Boot partition is not mounted correctly, the `refind` package is
missing from the offline mirror, the Aelum rEFInd theme was not shipped in the
ISO, or firmware rejected the NVRAM boot entry.

**Fix**:
```bash
# Check the ESP is mounted and rEFInd was copied
ls /mnt/efi/EFI/refind/
cat /mnt/efi/EFI/refind/refind.conf

# The Aelum menu entry is a per-install fragment included by refind.conf.
# Its name is the first eight characters of the root filesystem UUID.
ls /mnt/efi/EFI/refind/aelum/
cat /mnt/efi/EFI/refind/aelum/*.conf

# The branded menu lives in EFI/refind/themes/aelum (background, font, icons).
# If it is missing, rebuild the ISO with live-iso/.../aelum/refind/themes/aelum.
ls /mnt/efi/EFI/refind/themes/aelum/

# This install's kernel and initramfs live in their own ESP directory.
ls /mnt/efi/aelum/*/

# Verify EFI firmware / boot entries
efibootmgr -v
```

A missing NVRAM entry is logged as a warning (the install still completes); the
system remains bootable through the firmware boot menu or by selecting
`EFI/refind/refind_x64.efi` manually.

### rEFInd does not show Arch (or another OS) from a shared ESP

rEFInd scans every readable ESP and volume, so all bootable OSes should appear.
Aelum's generated config suppresses only the duplicate fallback loaders
(`dont_scan_files + BOOTX64.EFI,bootx64.efi`); it must not hide
`EFI/systemd` or `systemd-bootx64.efi`, otherwise a systemd-boot Arch install
on the same or another ESP disappears from the menu. If Arch is still missing,
confirm its loader exists on the ESP (often mounted at `/boot` or `/efi` in the
Arch install) at `EFI/systemd/systemd-bootx64.efi` (or a `linux*` loader entry
under `loader/entries`), and check that `refind.conf` has no
`dont_scan_dirs`/`dont_scan_files` line covering it.

### Installing Aelum twice on the same ESP

Every install is fully isolated, including its kernel files:

- The ESP is mounted at `/efi` in the installed system.
- Each install owns a directory `aelum/<id>/` on the ESP, where `<id>` is the
  first eight alphanumeric characters of its root filesystem UUID.
- That directory is bind-mounted at `/boot`, so pacman, mkinitcpio, and kernel
  updates keep writing to `/boot` as usual, but the files physically stay in
  this install's own directory.
- The rEFInd fragment `EFI/refind/aelum/<id>.conf` loads
  `/aelum/<id>/vmlinuz-…` and `/aelum/<id>/initramfs-….img`, and the shared
  `refind.conf` includes every install's fragment.

Two installs can therefore share one ESP without overwriting each other's menu
entry, kernel, or initramfs. The menu shows both as
`Aelum (<root partition>)`, e.g. `Aelum (sda2)` and `Aelum (sda3)`.

### Boot menu is stretched or at a low resolution

The installer writes the live display's preferred (native) mode as an explicit
`resolution <width> <height>` in `refind.conf`. If no DRM mode can be detected,
it falls back to `resolution max`. This is not limited to 1080p and 4K: 2K
(2560×1440), ultrawide, and other native modes are written as-is. The theme
background is 3840×2160 with `banner_scale fillscreen`, so it scales to the
chosen mode; the 2× icons are native on 2K/4K and downscaled on 1080p.

`resolution max` is not an aspect-ratio choice. rEFInd only switches to a
larger mode when both dimensions increase, so firmware that exposes non-native
VESA modes can select a 4:3 or 1:1 mode instead of 16:9. QEMU's emulated VGA is
a common example. If the menu looks stretched or square, set an explicit mode
in `/boot/EFI/refind/refind.conf`, e.g. `resolution 1920 1080` or
`resolution 3840 2160`.

If the explicit mode is not in the firmware's GOP list, rEFInd pauses at boot
with `Error setting graphics mode ...`. Boot the live ISO and change the line
to `resolution max` to recover.

rEFInd does not scale custom fonts. The installer therefore writes
`font themes/aelum/font-2x.png` on HiDPI displays (the same 1921px threshold
at which rEFInd doubles icon sizes) and leaves the 1× font from `theme.conf` on
1080p. If the labels are too small or too large, switch between
`themes/aelum/font.png` and `themes/aelum/font-2x.png` with a `font` line in
`/boot/EFI/refind/refind.conf`.

### "root UUID is empty" / "partitions not initialized"

**Cause**: `FormatDisk` was skipped or failed before populating partition info.

**Fix**: Run the format step first:
```bash
./aelum-installer-cli --format-disk /dev/sda
```

### Rollback Leaves Disk in Unknown State

**Cause**: Rollback partially failed (e.g., `wipefs` succeeded on one partition but not the other).

**Fix**:
```bash
# Full wipe — removes partition table and all signatures
sudo sgdisk -Z /dev/sda
```

## Hardware-Specific Issues

### NVMe Disks

NVMe devices use `nvme0n1p1` naming (not `sda1`). The installer handles this automatically via `DerivePartitionPath()`. If you see path errors, verify the disk name:

```bash
lsblk -d -o NAME,TYPE,SIZE /dev/nvme*
```

### TPM / Secure Boot

The installer does not configure Secure Boot. If Secure Boot is enabled, the kernel modules and bootloader may need to be signed. This is not handled by the installer.

### Small Disks (< 8 GB)

The installer rejects disks smaller than 8 GB. This is a hard limit because the base system + desktop require approximately 6–7 GB. Swap is provided by zram (compressed RAM) rather than an on-disk swap file, so no disk space is reserved for swap.

## Recovery: Starting Over

If an installation is partially complete and you want to start fresh:

```bash
# 1. Roll back the disk
sudo ./aelum-installer-cli --rollback /dev/sda

# 2. Verify it's clean
lsblk /dev/sda
# Should show partitions but no mountpoints

# 3. Re-run installation (password entered interactively)
sudo ./aelum-installer-cli --install /dev/sda --filesystem btrfs
# Or pipe it for non-interactive use:
# echo 'mypassword' | sudo ./aelum-installer-cli --install /dev/sda --filesystem btrfs
```

## Reporting Bugs

When reporting a bug, include:

1. **What you expected** vs **what actually happened**
2. **Steps to reproduce** (exact commands, disk type, filesystem choice)
3. **Log file** — attach `/var/log/aelum-install-<timestamp>.log` (or the copy in `/shared`)
4. **Environment**:
   - Live ISO version
   - Target disk model and size
   - Filesystem selected (ext4 / btrfs)
   - Any custom flags used

Report issues on [GitHub](https://github.com/aelum-dev/installer/issues).
