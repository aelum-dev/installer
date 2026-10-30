package install

import (
	"testing"
)

// TestSteps_SnapshotCopyPrecedesTheFactorySnapshot is an ordering the installer no
// longer enforces inside one step, so it has to be enforced here.
//
// The restore path for a booted snapshot - the helper, its polkit action and grant,
// the dialog and its user unit - is source in the live ISO and reaches the target
// through the SetupUsers copy jobs. The "after installation" snapshot is taken in
// ConfigureSystem. A snapshot boots the binaries, rules and prompt it contains, so
// the copy has to happen in a step that runs before the step that takes it; swap
// the two and every state the boot menu offers would come up without a way to
// restore it.
func TestSteps_SnapshotCopyPrecedesTheFactorySnapshot(t *testing.T) {
	steps := Steps()
	copyAt, configAt := -1, -1
	for i, step := range steps {
		switch step.Name() {
		case "setup_users":
			copyAt = i
		case "configure_system":
			configAt = i
		}
	}
	if copyAt < 0 || configAt < 0 {
		t.Fatalf("steps = %v; both setup_users and configure_system are expected", stepNames(steps))
	}
	if copyAt > configAt {
		t.Errorf("setup_users runs at %d, configure_system at %d: the snapshot would be taken before the restore path exists", copyAt, configAt)
	}
}

func stepNames(steps []Step) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.Name())
	}
	return names
}
