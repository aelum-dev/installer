# Manual Partitioning Page — Plan

Goal: let users install Aelum **without damaging their existing partitions**, enabling **dualboot**. Only the two partitions the user designates (`/boot` and `/`) are formatted and installed into; every other partition is left exactly as-is.

Status: **plan — not implemented yet.**

---

## 1. Design summary

Manual partitioning is added as a **mode toggle on the Disk Selection page** (coexisting with the existing automatic mode), not a new wizard page. Page IDs, navigation, and step weights stay unchanged.

- Under the filesystem pills on the Disk Selection page, add a second pill row: **Automatic / Manual**.
- Selecting a disk + **Manual** opens the **partition editor** (the "sub-page") for that disk.
- The editor shows a full-width list of partitions, a free/unallocated-space indicator, and controls to add/delete partitions and assign roles.

### Role model (dualboot-safe)

Each partition gets exactly one role:

| Role | Installer action |
|------|------------------|
| **Boot** | format **vfat**, mount at `/boot` |
| **Root** | format **ext4/btrfs** (mode fs), mount at `/`; btrfs → automatic subvolumes |
| **Preserve** *(default)* | **no action at all** — not formatted, not mounted, not in fstab |

`Preserve` is what makes dualboot safe: Windows, other Linux, recovery partitions, etc. are untouched. Only the one partition marked **Boot** and the one marked **Root** get wiped and installed into.

### Roles vs. filesystem

Filesystem is **role-locked**:
- **Boot** → vfat (ESP)
- **Root** → ext4 (automatic mode) or btrfs (manual mode with this fs selected)
- **Preserve** → no filesystem touched

The only ext4/btrfs difference: in **btrfs** mode the Root partition gets the automatic subvolume scheme (`@`, `@home`, `@srv`, `@cache`, `@tmp`, `@log`) applied. No manual subvolume selection anywhere.

### Actions

- **Designate** — set a row's role to Boot or Root (exactly one of each required). Existing partitions default to Preserve.
- **Add** — carve a new partition from free unallocated space, sized by the user, **immediately designated Boot or Root**. No throwaway rows: a created partition is always created to install into.
- **Delete** — remove an existing partition and return its space to free. Disallowed on a designated Boot/Root; guarded by a loss-warning dialog.
- **Free space** — derived (total − Σall partition sizes − ~1 MiB GPT overhead); it gates the Add control. Root can be created with "use all remaining" (`SizeMB == 0`).

The user's *only* real decisions: reshape the table (delete/add within free space) and mark one partition **Boot** and one **Root**. UX hint: assign **Boot first, then Root**. Mountpoints are assigned automatically from role — users never see or set mountpoints.

---

## 2. Data model

Backend (`backend/shared/types.go`):

```go
type PartitionRole string

const (
    RoleBoot     PartitionRole = "boot"
    RoleRoot     PartitionRole = "root"
    RolePreserve PartitionRole = "preserve"
)

// ManualPartition is one row in the manual layout plan.
type ManualPartition struct {
    Index          int            `json:"index"`
    Path           string         `json:"path"`           // existing device (e.g. /dev/sda1), "" for newly created
    SizeMB         int64          `json:"sizeMB"`         // 0 = consume all remaining free space
    Role           PartitionRole  `json:"role"`           // boot | root | preserve
    ExistingFstype string         `json:"existingFstype,omitempty"`
}

// ManualLayout is the user-designed plan for one disk.
type ManualLayout struct {
    Disk       string            `json:"disk"`
    DiskSizeMB int64             `json:"diskSizeMB"`
    Partitions []ManualPartition `json:"partitions"`
    FreeMB     int64             `json:"freeMB"`   // derived, never stored
}
```

`FreeMB` is always recomputed from the disk total (from `DiskInfo.Size`), never stored, so it stays correct after every edit. Frontend mirrors both interfaces in `frontend/src/lib/types.ts`; the layout lives in `installState.manualLayout`.

---

## 3. Backend — `backend/install/partition` package

- `ComputeLayout(diskID) (ManualLayout, error)` — reuse the `lsblk` path from `shared.GetDiskPartitions`; return existing rows (path, `ExistingFstype`, current mountpoints) plus derived free space.
- `Apply(ctx, layout) error` — generalization of `format.Run`. For Boot and Root rows only: `sgdisk` create/recreate at offset+size with the correct type GUID (vfat→ESP GUID, ext4/btrfs→LinuxRootX86_64), then `wipefs` + matching `mkfs.*` (reuse `format.formatPartition`). Root=btrfs → run the automatic subvolume scheme (`shared.GetBtrfsSubvolumes()`). Populate `ctx.Partitions` (`EFIPart`=Boot, `RootPart`=Root, UUIDs) and mount by role (Boot→`/mnt/boot`, Root→`/mnt`). `Preserve` rows are skipped entirely. Dry-run = log + no-op.
- `Validate(layout) error` — see §4.

`Apply` replaces `format.Run` on the manual install path; downstream mount/install/configure/verify steps are unchanged.

---

## 4. Validation (`partition.Validate`)

- exactly one `Boot`, exactly one `Root`
- `Σsizes + 1 MiB ≤ diskSize`
- at most one `SizeMB == 0` (the Root consuming remaining space)
- no partition marked with two roles

Frontend guards: delete blocked on designated Boot/Root rows (loss-warning dialog).

---

## 5. Mount step

`mount.Run` already mounts EFI + root + btrfs subvolumes. For manual mode it works unchanged as long as `EFIPart`=Boot and `RootPart`=Root (btrfs subvolumes mount themselves). Because `Preserve` rows are never mounted, **no generalization of `mount.Run` is required** — this differs from earlier drafts.

---

## 6. Frontend

- `DiskSelectionPage.svelte`: add the **Automatic / Manual** pill row; on Manual + a selected disk, render `ManualPartitionPage` as the sub-view.
- `ManualPartitionPage.svelte`:
  - Header: disk name + model + total size.
  - Full-width list: columns — partition (path/size), role selector (**Preserve / Boot / Root**), delete (🗑). Boot/Root rows show their locked fs.
  - Free-space track: live allocated-vs-free bar + MB label.
  - Add control: size input (MB) + "Use remaining" checkbox + role (Boot/Root); disabled when `FreeMB == 0`.
  - Delete → loss-warning confirm dialog.
- `store.svelte.ts`: add `manualLayout` + a derived `freeMB` getter; `isPageValid` for manual mode requires one Boot + one Root.

---

## 7. Wails bindings

Add to `backend/gui/`:
- `GetManualLayout(diskID) (ManualLayout, error)`
- `ValidateManualLayout(ManualLayout) error`
- `SaveManualLayout(ManualLayout)` — persist to app state

Wire into `navigation.ts` `commitPageData` / `validateCurrentPage` for the manual branch (mirrors `SaveDiskSelection`).

---

## 8. i18n

New keys (all 28 locales, RTL-safe table): role labels (Preserve/Boot/Root), free space label, add/delete, dualboot hint, "use remaining", and per-mode validation messages.

---

## 9. Testing

- Backend `partition_test.go`:
  - free-space math (incl. ~1 MiB GPT overhead)
  - `Validate` table cases (one-Boot/one-Root, over-size, multiple SizeMB=0)
  - `Apply` asserting the **dualboot invariant**: `Preserve` partitions are **never formatted or mounted**; Boot→vfat, Root→ext4/btrfs; btrfs root gets the automatic subvolumes
  - dry-run for both fs modes
- Frontend component test: add/delete/free recomputation + validation gating.
- VM end-to-end (QEMU): install onto a disk that also contains a non-target partition and confirm it is left untouched. Per repo rules, testing happens in the QEMU VM, not on the host.

---

## 10. Out of scope (future)

A shared/data partition in free space (e.g. a FAT32 partition used by both OSes) would need a mountpoint + format, which contradicts "no mountpoints / no actions on unused", so it is deliberately excluded. Could be a future optional role.

---

## Suggested build order

1. `shared` types + `partition.ComputeLayout` + `Validate` (+ tests)
2. `partition.Apply` (+ tests)
3. Wails bindings
4. Frontend toggle + page + store
5. i18n across locales
6. VM end-to-end
