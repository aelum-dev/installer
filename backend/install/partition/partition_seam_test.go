package partition

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/install/primitives/primitivestest"
	"aelum-installer/backend/shared"
)

// fakeTools writes executable shims onto PATH so the partition step's
// os/exec-based disk discovery (shared.GetDisks / GetDiskPartitions /
// LogicalSectorSize) sees scripted output.
func fakeTools(t *testing.T, scripts map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const (
	fakeDiskJSON = `{"blockdevices":[{"name":"vda","type":"disk","size":"21474836480","model":"FakeDisk","vendor":"","rota":"0","ro":"0","fstype":"ext4","mountpoint":"","serial":"","tran":"sata"}]}`
	fakePartJSON = `{"blockdevices":[{"children":[` +
		`{"name":"vda1","type":"part","size":"134217728","fstype":"vfat","uuid":"EFI-UUID","mountpoint":""},` +
		`{"name":"vda2","type":"part","size":"10000000000","fstype":"ext4","uuid":"ROOT-UUID","mountpoint":""}]}]}`
)

const lsblkShimBody = `for a in "$@"; do
  if [ "$a" = "-d" ]; then
    cat <<'JSON'
` + fakeDiskJSON + `
JSON
    exit 0
  fi
done
cat <<'JSON'
` + fakePartJSON + `
JSON
`

// emptyPartJSON is what lsblk returns for an unpartitioned (fresh) disk.
const emptyPartJSON = `{"blockdevices":[{"name":"vda","type":"disk","size":"21474836480","children":[]}]}`

const lsblkEmptyShimBody = `for a in "$@"; do
  if [ "$a" = "-d" ]; then
    cat <<'JSON'
` + fakeDiskJSON + `
JSON
    exit 0
  fi
done
cat <<'JSON'
` + emptyPartJSON + `
JSON
`

// --- ComputeLayout -----------------------------------------------------------

func TestComputeLayout(t *testing.T) {
	fakeTools(t, map[string]string{
		"lsblk":    lsblkShimBody,
		"blockdev": "echo 512\n",
	})

	layout, err := ComputeLayout("/dev/vda")
	if err != nil {
		t.Fatalf("ComputeLayout() = %v", err)
	}
	if layout.Disk != "/dev/vda" || layout.DiskSizeBytes != 21474836480 {
		t.Errorf("disk fields wrong: %+v", layout)
	}
	if layout.SectorSizeBytes != 512 {
		t.Errorf("SectorSizeBytes = %d, want 512", layout.SectorSizeBytes)
	}
	if len(layout.Partitions) != 2 {
		t.Fatalf("partitions = %d, want 2", len(layout.Partitions))
	}
	if layout.Partitions[0].Path != "/dev/vda1" || layout.Partitions[0].ExistingFstype != "vfat" {
		t.Errorf("partition[0] = %+v", layout.Partitions[0])
	}
	if layout.Partitions[1].Path != "/dev/vda2" || layout.Partitions[1].SizeBytes != 10000000000 {
		t.Errorf("partition[1] = %+v", layout.Partitions[1])
	}
	// Existing rows must be Preserve (empty role): left untouched.
	for _, p := range layout.Partitions {
		if p.Type != "" {
			t.Errorf("existing partition %s has role %q, want empty", p.Path, p.Type)
		}
	}
}

func TestComputeLayout_NoDiskID(t *testing.T) {
	if _, err := ComputeLayout(""); err == nil || !strings.Contains(err.Error(), "disk ID not set") {
		t.Fatalf("ComputeLayout(\"\") error = %v", err)
	}
}

// An unpartitioned disk must yield an empty (non-nil) partition slice. A nil
// slice marshals to JSON null, and the frontend's free-space math iterates the
// list directly — that crashed the manual page on "Scanning partitions...".
func TestComputeLayout_EmptyDiskReturnsEmptySlice(t *testing.T) {
	fakeTools(t, map[string]string{
		"lsblk":    lsblkEmptyShimBody,
		"blockdev": "echo 512\n",
	})

	layout, err := ComputeLayout("/dev/vda")
	if err != nil {
		t.Fatalf("ComputeLayout() = %v", err)
	}
	if layout.Partitions == nil {
		t.Fatal("Partitions is nil, want an empty non-nil slice")
	}
	if len(layout.Partitions) != 0 {
		t.Fatalf("partitions = %d, want 0", len(layout.Partitions))
	}

	encoded, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"partitions":[]`) {
		t.Errorf("layout JSON = %s, want an empty partitions array", encoded)
	}
}

func TestComputeLayout_DiskNotFound(t *testing.T) {
	fakeTools(t, map[string]string{"lsblk": lsblkShimBody, "blockdev": "echo 512\n"})

	if _, err := ComputeLayout("/dev/sdz"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ComputeLayout(missing) error = %v", err)
	}
}

// --- roleLabel / reconcileLuks / ensureSectorSize ----------------------------

func TestRoleLabel(t *testing.T) {
	if got := roleLabel(shared.FileSystemVfat); got != "EFI" {
		t.Errorf("roleLabel(vfat) = %q, want EFI", got)
	}
	if got := roleLabel(shared.FileSystemExt4); got != "aelum" {
		t.Errorf("roleLabel(ext4) = %q, want aelum", got)
	}
}

func TestReconcileLuks(t *testing.T) {
	tests := []struct {
		name           string
		ctx, layout    bool
		wantCtxLuks    bool
		wantLayoutLuks bool
	}{
		{"neither", false, false, false, false},
		{"ctx only", true, false, true, true},
		{"layout only", false, true, true, true},
		{"both", true, true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &shared.InstallContext{Luks: tt.ctx}
			layout := &shared.ManualLayout{Luks: tt.layout}
			reconcileLuks(ctx, layout)
			if ctx.Luks != tt.wantCtxLuks || layout.Luks != tt.wantLayoutLuks {
				t.Errorf("reconcileLuks() = ctx:%v layout:%v, want ctx:%v layout:%v", ctx.Luks, layout.Luks, tt.wantCtxLuks, tt.wantLayoutLuks)
			}
		})
	}
}

func TestEnsureSectorSize(t *testing.T) {
	t.Setenv("FAKE_DISKS", "1") // LogicalSectorSize falls back to 512 without a real device

	unset := &shared.ManualLayout{}
	ensureSectorSize(unset, "/dev/vda")
	if unset.SectorSizeBytes != 512 {
		t.Errorf("unset sector size = %d, want 512", unset.SectorSizeBytes)
	}

	preset := &shared.ManualLayout{SectorSizeBytes: 4096}
	ensureSectorSize(preset, "/dev/vda")
	if preset.SectorSizeBytes != 4096 {
		t.Errorf("preset sector size overwritten to %d, want 4096", preset.SectorSizeBytes)
	}
}

// --- partitionSectors --------------------------------------------------------

func TestPartitionSectors(t *testing.T) {
	const table = "Number  Start (sector)    End (sector)  Size       Code  Name\n" +
		"   1            2048         2099199   1024.0 MiB  EF00  EFI\n" +
		"   2         2099200        41943006   19.0 GiB   8300  Root\n"
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"sgdisk", "-p", "/dev/vda"}, table)
	defer primitives.SetCommandRunner(rec)()

	sectors, err := partitionSectors("/dev/vda")
	if err != nil {
		t.Fatalf("partitionSectors() = %v", err)
	}
	if len(sectors) != 2 {
		t.Fatalf("sectors = %v, want 2 rows", sectors)
	}
	if sectors[0] != (partitionSector{num: 1, start: 2048, end: 2099199}) {
		t.Errorf("sectors[0] = %+v", sectors[0])
	}
}

func TestPartitionSectors_Error(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"sgdisk", "-p"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	if _, err := partitionSectors("/dev/vda"); err == nil || !strings.Contains(err.Error(), "sgdisk -p") {
		t.Fatalf("partitionSectors() error = %v", err)
	}
}

// --- createPartition / deletePartitions --------------------------------------

func TestCreatePartition(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	a := formatAction{num: 3, startSector: 2048, sizeMB: 1024, guid: shared.GPTTypeGUIDs.LinuxRootX86_64, fstype: shared.FileSystemExt4}
	if err := createPartition("/dev/vda", a); err != nil {
		t.Fatalf("createPartition() = %v", err)
	}
	if !hasArgvPrefix(rec.CallsFor("sgdisk"), "sgdisk", "-n", "3:2048:+1024M", "-t", "3:"+shared.GPTTypeGUIDs.LinuxRootX86_64, "-c", "3:aelum", "/dev/vda") {
		t.Errorf("sgdisk args wrong: %v", rec.CallsFor("sgdisk"))
	}
}

func TestCreatePartition_Error(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"sgdisk", "-n"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	a := formatAction{num: 1, startSector: 2048, sizeMB: 128, guid: shared.GPTTypeGUIDs.ESP, fstype: shared.FileSystemVfat}
	if err := createPartition("/dev/vda", a); err == nil || !strings.Contains(err.Error(), "sgdisk create partition 1") {
		t.Fatalf("createPartition() error = %v", err)
	}
}

func TestDeletePartitions_OrderAndProbe(t *testing.T) {
	rec := primitivestest.NewRecorder()
	defer primitives.SetCommandRunner(rec)()

	deleted := []shared.ManualPartition{{Path: "/dev/vda1"}, {Path: "/dev/vda2"}}
	if err := deletePartitions("/dev/vda", deleted); err != nil {
		t.Fatalf("deletePartitions() = %v", err)
	}

	var dels [][]string
	for _, argv := range rec.CallsFor("sgdisk") {
		if len(argv) > 1 && argv[1] == "-d" {
			dels = append(dels, argv)
		}
	}
	if len(dels) != 2 || dels[0][2] != "2" || dels[1][2] != "1" {
		t.Errorf("deletions not highest-first: %v", dels)
	}
	if !rec.Called("partprobe") {
		t.Error("partprobe not called after deletions")
	}
}

func TestDeletePartitions_DeleteError(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"sgdisk", "-d"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	err := deletePartitions("/dev/vda", []shared.ManualPartition{{Path: "/dev/vda2"}})
	if err == nil || !strings.Contains(err.Error(), "sgdisk delete partition 2") {
		t.Fatalf("deletePartitions() error = %v", err)
	}
}

func TestDeletePartitions_ProbeError(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsError([]string{"partprobe"}, errors.New("exit status 1"))
	defer primitives.SetCommandRunner(rec)()

	if err := deletePartitions("/dev/vda", nil); err == nil || !strings.Contains(err.Error(), "partprobe") {
		t.Fatalf("deletePartitions() error = %v", err)
	}
}

// --- hasNewPartitions / ValidateFeasible -------------------------------------

func TestHasNewPartitions(t *testing.T) {
	if hasNewPartitions(shared.ManualLayout{Partitions: []shared.ManualPartition{{Path: "/dev/vda1", Type: shared.TypeESP}}}) {
		t.Error("existing-only layout reported new partitions")
	}
	if !hasNewPartitions(shared.ManualLayout{Partitions: []shared.ManualPartition{{Type: shared.TypeSystem}}}) {
		t.Error("brand-new System row not detected")
	}
	if hasNewPartitions(shared.ManualLayout{Partitions: []shared.ManualPartition{{Type: ""}}}) {
		t.Error("untouched row reported as new")
	}
}

func newDiskLayout() shared.ManualLayout {
	return shared.ManualLayout{
		Disk:            "/dev/vda",
		DiskSizeMB:      20480,
		DiskSizeBytes:   21474836480,
		SectorSizeBytes: 512,
		Partitions: []shared.ManualPartition{
			{Type: shared.TypeESP, SizeMB: 512},
			{Type: shared.TypeSystem, SizeMB: 0},
		},
	}
}

func TestValidateFeasible(t *testing.T) {
	t.Run("no disk skips placement", func(t *testing.T) {
		layout := newDiskLayout()
		layout.Disk = ""
		if err := ValidateFeasible(layout); err != nil {
			t.Fatalf("ValidateFeasible() = %v, want nil", err)
		}
	})

	t.Run("no new partitions skips placement", func(t *testing.T) {
		layout := shared.ManualLayout{
			Disk:            "/dev/vda",
			DiskSizeMB:      20480,
			DiskSizeBytes:   21474836480,
			SectorSizeBytes: 512,
			Partitions: []shared.ManualPartition{
				{Path: "/dev/vda1", Type: shared.TypeESP, SizeMB: 512, SizeBytes: 512 * 1024 * 1024, ExistingFstype: "vfat"},
				{Path: "/dev/vda2", Type: shared.TypeSystem, SizeMB: 10000, SizeBytes: 10000 * 1024 * 1024},
			},
		}
		if err := ValidateFeasible(layout); err != nil {
			t.Fatalf("ValidateFeasible() = %v, want nil", err)
		}
	})

	t.Run("fits", func(t *testing.T) {
		rec := primitivestest.NewRecorder().OnArgsOutput([]string{"sgdisk", "-p", "/dev/vda"}, "")
		defer primitives.SetCommandRunner(rec)()
		if err := ValidateFeasible(newDiskLayout()); err != nil {
			t.Fatalf("ValidateFeasible() = %v, want nil", err)
		}
	})

	t.Run("geometry read failure is not fatal", func(t *testing.T) {
		rec := primitivestest.NewRecorder().OnArgsError([]string{"sgdisk", "-p"}, errors.New("exit status 1"))
		defer primitives.SetCommandRunner(rec)()
		if err := ValidateFeasible(newDiskLayout()); err != nil {
			t.Fatalf("ValidateFeasible() = %v, want nil on geometry failure", err)
		}
	})

	t.Run("invalid layout fails Validate first", func(t *testing.T) {
		layout := newDiskLayout()
		layout.Partitions = nil // no ESP/System
		if err := ValidateFeasible(layout); err == nil {
			t.Fatal("ValidateFeasible() = nil, want Validate error")
		}
	})
}

// --- Apply -------------------------------------------------------------------

func TestApply_PlainExt4BrandNewPartitions(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"sgdisk", "-p", "/dev/vda"}, "").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda1"}, "EFI-UUID\n").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	layout := newDiskLayout()
	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	if err := Apply(ctx, &layout); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	if ctx.Partitions == nil {
		t.Fatal("ctx.Partitions not populated")
	}
	if ctx.Partitions.EFIPart != "/dev/vda1" || ctx.Partitions.RootPart != "/dev/vda2" {
		t.Errorf("partition paths wrong: %+v", ctx.Partitions)
	}
	if ctx.Partitions.RootUUID != "ROOT-UUID" || ctx.Partitions.EFIUUID != "EFI-UUID" {
		t.Errorf("UUIDs wrong: %+v", ctx.Partitions)
	}
	// Both brand-new partitions are created and then formatted.
	if got := len(callsForArg(rec, "sgdisk", "-n")); got != 2 {
		t.Errorf("sgdisk -n calls = %d, want 2", got)
	}
	if !rec.Called("mkfs.fat") || !rec.Called("mkfs.ext4") {
		t.Error("partitions not formatted")
	}
}

// TestApply_ExistingPartitionsFormatted is the manual-mode contract: the
// designated Boot (ESP) and System partitions are formatted even when they are
// existing partitions, so no other OS's boot files can survive on Aelum's ESP.
func TestApply_ExistingPartitionsFormatted(t *testing.T) {
	rec := primitivestest.NewRecorder().
		OnArgsOutput([]string{"sgdisk", "-p", "/dev/vda"}, "").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda1"}, "EFI-UUID\n").
		OnArgsOutput([]string{"blkid", "-s", "UUID", "-o", "value", "/dev/vda2"}, "ROOT-UUID\n")
	defer primitives.SetCommandRunner(rec)()

	layout := shared.ManualLayout{
		Disk:            "/dev/vda",
		DiskSizeMB:      20480,
		DiskSizeBytes:   21474836480,
		SectorSizeBytes: 512,
		Partitions: []shared.ManualPartition{
			{Path: "/dev/vda1", SizeMB: 512, SizeBytes: 512 * 1024 * 1024, Type: shared.TypeESP, ExistingFstype: "ext4"},
			{Path: "/dev/vda2", SizeMB: 10000, SizeBytes: 10000 * 1024 * 1024, Type: shared.TypeSystem, ExistingFstype: "ext4"},
		},
	}
	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}
	if err := Apply(ctx, &layout); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	if ctx.Partitions == nil {
		t.Fatal("ctx.Partitions not populated")
	}
	if ctx.Partitions.EFIPart != "/dev/vda1" || ctx.Partitions.RootPart != "/dev/vda2" {
		t.Errorf("partition paths wrong: %+v", ctx.Partitions)
	}
	if got := len(callsForArg(rec, "sgdisk", "-n")); got != 0 {
		t.Errorf("sgdisk -n calls = %d, want 0 (no partition created)", got)
	}
	if !rec.Called("mkfs.fat") {
		t.Error("existing Boot partition was not formatted; it must be formatted as vfat")
	}
	if !rec.Called("mkfs.ext4") {
		t.Error("System partition was not formatted")
	}
	// The existing partitions keep their slots and get the role's GPT type.
	if got := len(callsForArg(rec, "sgdisk", "-t")); got != 2 {
		t.Errorf("sgdisk -t calls = %d, want 2 (per existing ESP/System row)", got)
	}
}

func TestApply_NoDisk(t *testing.T) {
	layout := newDiskLayout()
	layout.Disk = ""
	ctx := &shared.InstallContext{}

	err := Apply(ctx, &layout)
	if err == nil || !strings.Contains(err.Error(), "no disk") {
		t.Fatalf("Apply() error = %v, want no-disk", err)
	}
}

func TestApply_LuksWithoutPassphrase(t *testing.T) {
	rec := primitivestest.NewRecorder().OnArgsOutput([]string{"sgdisk", "-p", "/dev/vda"}, "")
	defer primitives.SetCommandRunner(rec)()

	layout := newDiskLayout()
	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4, Luks: true}
	err := Apply(ctx, &layout)
	if err == nil || !strings.Contains(err.Error(), "no passphrase") {
		t.Fatalf("Apply() error = %v, want no-passphrase", err)
	}
}

func TestApply_ValidationFails(t *testing.T) {
	layout := newDiskLayout()
	layout.Partitions = []shared.ManualPartition{{Type: shared.TypeESP, SizeMB: 512}} // no System
	ctx := &shared.InstallContext{DiskID: "/dev/vda", FileSystem: shared.FileSystemExt4}

	err := Apply(ctx, &layout)
	if err == nil || !strings.Contains(err.Error(), "exactly one System") {
		t.Fatalf("Apply() error = %v, want System-partition error", err)
	}
}

// --- helpers -----------------------------------------------------------------

func callsForArg(rec *primitivestest.Recorder, prog, arg string) [][]string {
	var out [][]string
	for _, argv := range rec.CallsFor(prog) {
		for _, a := range argv[1:] {
			if a == arg {
				out = append(out, argv)
				break
			}
		}
	}
	return out
}

func hasArgvPrefix(calls [][]string, want ...string) bool {
	for _, argv := range calls {
		if len(argv) < len(want) {
			continue
		}
		match := true
		for i := range want {
			if argv[i] != want[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
