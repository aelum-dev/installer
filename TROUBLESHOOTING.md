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

### "bootloader verification failed" (rEFInd)

**Cause**: The Boot partition is not mounted correctly, the `refind` package is
missing from the offline mirror, or firmware rejected the NVRAM boot entry.

**Fix**:
```bash
# Check the Boot partition is mounted and rEFInd was copied
ls /mnt/boot/EFI/refind/
cat /mnt/boot/EFI/refind/refind.conf

# Verify EFI firmware / boot entries
efibootmgr -v
```

A missing NVRAM entry is logged as a warning (the install still completes); the
system remains bootable through the firmware boot menu or by selecting
`EFI/refind/refind_x64.efi` manually.

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
