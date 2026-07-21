# Disk Detection & Mounting — Issues & Improvements

## Current Issues

### 1. **No Device Filtering**
The `GetDisks()` function returns ALL block devices including:
- Loop devices (`loop0`, `loop1`, ...)
- RAM disks (`ram0`, `ram1`, ...)
- CD-ROMs (`sr0`, `sr1`, ...)
- LVM volumes (`dm-*`)
- Software RAID (`md*`)
- Virtual devices

This could confuse users with irrelevant devices.

### 2. **No Partition Information Display**
When a disk is selected, users don't see what partitions currently exist. The `DiskSelectionPage` shows `fstype` and `mountpoint` of the disk itself, but not its children.

### 3. **No Pre-flight Mount Check**
`FormatDisk` unmounts partitions before formatting, but there's no GUI warning if the selected disk has mounted partitions.

### 4. ~~**Device Path Assumptions**~~ ✅ Fixed

> ~~The code assumed partition numbering works by simple string concatenation (`diskID + "1"`).~~
>
> ~~This worked for `/dev/sda` → `/dev/sda1`, but broke for:~~
> - ~~NVMe devices: `/dev/nvme0n1` → `/dev/nvme0n11` (note the `p`)~~
> - ~~Other naming conventions~~

**Fix:** Added `GetPartitions()` in `backend/shared/disks.go` — queries `lsblk` for actual partition paths. Used in `FormatDisk` and `MountFilesystem` instead of string concatenation. Includes two-tier matching: fstype first, partition index as fallback (handles case where FSTYPE is empty before superblock sync).

### 5. **No Disk Type Detection**
No distinction between:
- Physical vs virtual disks
- Removable media (USB, SD card)
- Minimum size validation

### 6. **No Mount Verification**
After `mount` commands in `MountFilesystem`, there's no verification that the mount actually succeeded.

### 7. **No LUKS/LVM Detection**
The system doesn't detect or warn about encrypted (LUKS) or logical volume (LVM) devices.

---

## Recommended Improvements

### **High Priority**

1. **Filter block devices in `GetDisks()`**
   - Only show physical disks (`/dev/sd*`, `/dev/nvme*`, `/dev/vd*`)
   - Exclude loop, ram, sr, dm, md devices
   - Add a `IsRemovable` field to `DiskInfo`

2. **Show partition information**
   - Add a helper command to list partitions with their types/sizes
   - Display existing partitions in `DiskSelectionPage`
   - Warn if disk has existing data

3. ~~**Fix partition path calculation**~~ ✅ Fixed
   - ~~Use `lsblk` to get actual partition paths instead of string concatenation~~
   - ~~Handle NVMe naming convention (`nvme0n1p1` not `nvme0n11`)~~

4. **Add mount verification**
   - After mounting, verify with `findmnt` or `mountpoint`
   - Add error handling for failed mounts

### **Medium Priority**

5. **Add pre-flight checks**
   - Check if disk has mounted partitions before allowing selection
   - Warn about existing data
   - Show disk health status if possible

6. **Add disk size validation**
   - Minimum size check (e.g., 8GB for Arch Linux)
   - Show available space after partitions

7. **Detect and warn about LUKS/LVM**
   - Check for encrypted or logical volume devices
   - Show in disk info

### **Low Priority**

8. **UUID-based operations**
   - Use UUIDs instead of device paths for more reliable mounting
   - Device paths can change between reboots

9. **Handle existing EFI partitions**
   - Detect if disk already has ESP
   - Offer to reuse instead of recreate
