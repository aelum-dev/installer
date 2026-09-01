# Manual Partitioning — What Was Built

Dualboot-safe manual partitioning implemented per `manual_partition_plan.md`.
Users can now install Aelum **without damaging existing partitions**: only the
two partitions they designate (`/boot` and `/`) are formatted and installed
into; every other partition is left exactly as-is.

Added as a **mode toggle on the Disk Selection page** (Automatic / Manual), not
a new wizard page. Page IDs, navigation, and step weights are unchanged.

---

## Backend (Go)

### New package: `backend/install/partition/partition.go`
- **`ComputeLayout(diskID)`** — reuses the `lsblk` path from `shared.GetDiskPartitions`;
  returns existing rows (path, `ExistingFstype`) all preset to `Preserve`, plus
  derived `FreeMB` (total − Σsizes − ~1 MiB GPT overhead, never stored).
- **`Validate(layout)`** — exactly one Boot + one Root; `Σsizes + 1 MiB ≤ diskSize`;
  at most one `SizeMB == 0` ("use all remaining"); every role must be known.
- **`Apply(ctx, layout)`** — generalization of `format.Run` for the manual path.
  For Boot/Root rows only: `sgdisk` create/recreate at offset+size with the
  correct type GUID (vfat → ESP, ext4/btrfs → LinuxRootX86_64), then
  `wipefs` + matching `mkfs.*` (reuses `format.FormatPartition`); btrfs root →
  automatic subvolumes (`format.SetupBtrfsSubvolumes`). Populates
  `ctx.Partitions` (`EFIPart`=Boot, `RootPart`=Root, UUIDs) keyed by role.
  **`Preserve` rows are skipped entirely** — never formatted, never mounted.

### `backend/install/installer.go` + `step.go`
- `Installer.SetManualLayout(layout)`; `InstallContext` gains `ManualMode` +
  `ManualLayout`; layout validated up front before any step runs.
- The `format_disk` step dispatches to `partition.Apply` when `ctx.ManualMode`,
  otherwise `format.Run`.

### `backend/cli/main.go`
- `--manual-layout <json>` flag: parses and validates the layout, then
  `installer.SetManualLayout(layout)`.

### `backend/gui/bindings.go` + `installation.go`
- Wails bindings: `GetManualLayout`, `ValidateManualLayout`, `SaveManualLayout`,
  `GetManualMode`.
- At install time, when manual mode is on, the layout is JSON-marshalled and
  passed to the CLI as `--manual-layout <json>`.

### Wails bindings regenerated
`frontend/wailsjs/go/gui/App.js` (4 new functions) and
`frontend/wailsjs/go/models.ts` (`ManualLayout`, `ManualPartition` classes).

---

## Frontend (Svelte 5)

### `frontend/src/pages/ManualPartitionPage.svelte` (new)
- Header: disk name + model + total size, dualboot hint.
- Live allocated-vs-free space bar (MB labels).
- Full-width editor table: partition (path/size), role pills
  (Preserve / Boot / Root), role-locked filesystem column (Boot → vfat,
  Root → selected mode fs), delete (×).
- Add control: size input (MB) + "Use remaining" checkbox + role (Boot/Root);
  disabled when free space is 0. New partitions are created already designated.
- Delete blocked on designated Boot/Root rows; guarded by a loss-warning dialog.

### `frontend/src/pages/DiskSelectionPage.svelte`
- Second pill row (below filesystem pills): **Automatic / Manual**.
- In manual mode with a selected disk, renders `<ManualPartitionPage>`.

### `frontend/src/lib/store.svelte.ts`
- `manualMode` + `manualLayout` state; `computeFreeMB`, `roleCount`,
  `hasRequiredRoles`, `setPartitionRole` helpers.
- `isPageValid` for Disk Selection requires one Boot + one Root in manual mode.

### `frontend/src/lib/navigation.ts`
- Manual branch: `ValidateManualLayout` on validate, `SaveManualLayout` on commit;
  returns a mode-specific error (`validation.selectDiskManual`) on failure.

### `frontend/src/lib/types.ts`
- `PartitionRole` type + `ROLE_BOOT`/`ROLE_ROOT`/`ROLE_PRESERVE` constants;
  `ManualPartition` / `ManualLayout` interfaces mirroring the backend.

### `frontend/src/translations/en-US.json`
- New keys: `manual.role.{preserve,boot,root}`, `manual.{modeAutomatic,modeManual,
  dualbootHint,noPartitions,allocated,of,free,role,filesystem,untouched,existing,
  newPart,add,sizeMB,useRemaining,addTooSmall,delete,deleteTitle,deleteMessage,
  deleteConfirm,assignHint}`, `unit.mb`, `page.diskSelection.mode`,
  `validation.selectDiskManual`.

---

## Verification
- `go build ./...` and full `go test ./backend/...` pass.
- `svelte-check`: 0 errors (2 a11y warnings, identical to the existing
  `ConfirmDialog` pattern).
- Vitest: 38/38 pass.
- CLI manual path exercised end-to-end in dry-run: valid layout completes;
  over-size and multi-Root layouts are correctly rejected.

## Known gaps (per plan, out of this pass)
- **i18n:** keys added to `en-US.json` only; the `t()` fallback chain keeps other
  locales functional. The other 27 locale translations for the new `manual.*`
  keys are not yet written.
- **VM end-to-end:** real-install testing belongs in the QEMU VM (host cannot
  format disks). The `partition.Apply` dualboot invariant — that `Preserve`
  partitions are left untouched — still needs the VM run from plan step 6.
