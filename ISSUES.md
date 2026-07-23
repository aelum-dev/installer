# AestheticOS Installer — Code Audit Report

## CRITICAL BUGS

### 1. ~~`CanGoNext()` validation is completely disabled~~ — **RESOLVED**
Removed `CanGoNext()` from backend, frontend, and test mocks. The frontend validation in `navigation.ts::goNext()` handles all checks; the backend function was a dead path that always returned `true`.

### 2. ~~`CanGoBack()` — unused~~ — **DEAD CODE**
Imported in `navigation.ts` but never called. The `goBack()` function calls `SetCurrentPage` directly without checking. Removed from backend, frontend, and test mocks.

### 3. `DefaultCLIPassword` is a hardcoded plaintext password
**File:** `backend/internal/shared/config.go:111`
```go
const DefaultCLIPassword = "aesthetic"  // CHANGE THIS before any production use.
```
Still has the default password. The comment says "CHANGE THIS" but it hasn't been changed.

---

## BUGS

### 4. `cmdBtrfsSetCompression` doesn't validate argument count for the algorithm
**File:** `backend/cmd/helper/main.go` — the function checks `len(args) < 2` but then accesses `args[2]` for the algorithm parameter without checking `len(args) >= 3`.

### 5. `cmdEfibootmgr delete` — no subcommand validation for the delete case
**File:** `backend/cmd/helper/main.go` — when `subcmd == "delete"`, it accesses `args[1]` without checking that `len(args) >= 2` first (the `len(args) < 1` check at the top only guards `args[0]`).

### 6. `cmdFormat` — extra args passed to both wipefs AND mkfs
**File:** `backend/cmd/helper/main.go` — `extraArgs := args[2:]` is appended to the wipefs command and then appended again to the mkfs command. wipefs shouldn't receive those extra arguments.

### 7. `Install()` clears the log file on every call
**File:** `backend/internal/core/installer.go:78` and `RunStep` at line 153 — both call `os.WriteFile(logPath, nil, shared.LogFileMode)` which truncates the log. If `RunStep` is called after `Install()`, the installation log is erased.

### 8. `unmountAllPartitions` error is silently swallowed
**File:** `backend/internal/install/format_disk.go:82` — `if err := unmountAllPartitions(...); err != nil { /* best effort — continue */ }` — if unmounting fails, the disk may still be locked when partitioning starts, causing silent failures.

### 9. `FormatDiskErrorCleanup` called twice before partitioning
**File:** `backend/internal/install/format_disk.go` — called once at line 90, then called again at line 95 right before clearing the partition table. The second call is redundant — nothing has changed between them.

---

## DEAD CODE

### 10. `GetSubvolumeMountPoints()` — defined but never called
**File:** `backend/internal/shared/btrfs.go:40`

### 11. `HasSubvolumeWithMountPoint()` — defined but never called
**File:** `backend/internal/shared/btrfs.go:47`

### 12. `ListRegionCodes()` — defined but never called
**File:** `backend/internal/shared/regions.go:48`

### 13. `FormatPartitionSize()` — defined but never called
**File:** `backend/internal/shared/disks.go:224`

### 14. `TestDiscoverKernelFiles` — skipped test, does nothing
**File:** `backend/internal/install/install_test.go` — `t.Skip()` followed by no assertions.

### 15. `TestMountFilesystem_AutoDiscover` — simulated logic, not real test
**File:** `backend/internal/install/install_test.go` — manually simulates what the code would do; doesn't test anything real.

### 16. `//go:build ignore` on mirrorlist_gen.go is redundant
**File:** `backend/internal/shared/mirrorlist_gen.go:1` — `package main` already prevents it from being imported.

---

## MAGIC NUMBERS & HARDCODED STRINGS

### 17. Magic number `200` — dry-run sleep duration
**File:** `backend/internal/install/config.go:175` — `SimulateStepSleepDuration = 200` — the unit (milliseconds) is implied by the variable name, but the value `200` appears nowhere else documented.

### 18. Magic number `1000` — log channel buffer
**File:** `backend/internal/install/config.go:162` — `LogChannelBufferSize = 1000`

### 19. Magic number `3` — retry count
**File:** `backend/internal/install/config.go:165` — `LogWriteRetries = 3`

### 20. `MinFstabContentLengthBytes = 50` — magic number
**File:** `backend/internal/shared/config.go:105` — no explanation for why 50 bytes is the threshold.

### 21. `GPTTypeGUIDs.LinuxFilesystemGeneric = "8300"` — bare hex without explanation
**File:** `backend/internal/shared/config.go:124` — "8300" is a GPT type code but has no comment explaining what it is.

### 22. `BootEntryFileName = "aestheticos"` and `BootOptionDescription = "Aesthetic OS"` — duplicate names
**File:** `backend/internal/install/config.go:132,134` — These are used in the bootloader config written to the ESP. If the user renames their installation, these strings are hardcoded in.

### 23. `MkfsExt4LabelValue = "aestheticos"` and `MkfsBtrfsLabelValue = "aestheticos"` — duplicate label
**File:** `backend/internal/install/config.go:86,91` — Same label for both filesystems.

---

## DRY VIOLATIONS

### 24. `MIN_DISK_SIZE_BYTES` defined in both frontend and backend
- `ConfirmationPage.svelte:33`: `const MIN_DISK_SIZE_BYTES = 8 * 1024 * 1024 * 1024`
- `AppFooter.svelte:19`: `const MIN_DISK_SIZE_BYTES = 8 * 1024 * 1024 * 1024`
- `shared/config.go`: `MinDiskSizeBytes = 8 * 1024 * 1024 * 1024`
Three places, all computing the same constant. The frontend versions are bare `8 * 1024 * 1024 * 1024` without any constant reference.

### 25. `formatSize` defined in both Go and TypeScript
- `shared/disks.go:218` — `FormatSize`
- `frontend/src/lib/types.ts:107` — `formatSize`
Both do the same byte-to-human-readable conversion. The Go version uses named constants (KB, MB, GB, TB) while the TS version uses `['B', 'KB', 'MB', 'GB', 'TB']` — slightly different logic (Go uses `fmt.Sscanf`, TS uses `parseInt`).

### 26. `GetLogPath()` logic duplicated in `gui/logger.go`
- `shared/logpath.go:13` — `GetLogPath()` — does `os.Executable()` → `filepath.Dir(exe)` → `filepath.Join(dir, LogFileName)`
- `gui/logger.go:24` — `getExePath()` — does the same `os.Executable()` → `filepath.Dir(exe)` pattern, then `filepath.Join(filepath.Dir(...), "installation.log")`
The `gui/logger.go` version should use `shared.GetLogPath()` instead of reimplementing it.

### 27. `BootMountPoint` and `EFIPartitionSizeMB` referenced in both config files
- `shared/config.go` defines `EFIPartitionSizeMB = 128` and `MountRoot = "/mnt"`
- `install/config.go` defines `BootMountPoint = "/mnt/boot"` but also references `EFIPartitionSizeMB` with a comment "defined in shared/config.go"
- `format_disk.go:63` uses `shared.EFIPartitionSizeMB` — but the layout struct has `SizeMB: shared.EFIPartitionSizeMB` which is correct, so this is fine.

---

## ARCHITECTURE / DESIGN ISSUES

### 28. `GetLanguages()` returns pointers to local data that become dangling
**File:** `backend/internal/shared/locales.go:17` — `return &langs[i]` returns a pointer to an element of a local slice. Since the slice is re-created on every call, this is actually fine in Go (the slice header is on the heap after escape analysis), but it's misleading — callers might assume the pointer is stable across calls when it's not.

### 29. `SetSelectedLanguage` does a redundant check
**File:** `backend/gui/bindings.go:57` — checks `shared.FindLanguage(code)` but this is redundant because `GetRegion` would have already failed if the code were invalid. The check adds nothing.

### 30. `GetBtrfsSubvolumes()` creates a defensive copy that's never needed
**File:** `backend/internal/shared/btrfs.go:34` — The comment says "Returns a copy to prevent external mutation" but the callers in `format_disk.go`, `mount_filesystem.go`, and `fstab.go` never mutate the returned slice. The copy is wasted allocation.

### 31. `derivePartitionPath` — "nvme" string check is fragile
**File:** `backend/internal/install/format_disk.go:380` — `strings.Contains(diskID, "nvme")` would match any disk path containing "nvme" as a substring, not just `/dev/nvmeXnY`. A path like `/dev/nvme-controller-alias` would incorrectly get the `p` prefix. Should use a regex or prefix check.

---

## SUMMARY

| Category | Count |
|----------|-------|
| Critical bugs | 1 |
| Bugs | 6 |
| Dead code | 8 |
| Magic numbers / hardcoded strings | 7 |
| DRY violations | 4 |
| Architecture issues | 4 |
| **Total issues** | **31** |
