package users

import (
	"fmt"
	"strings"

	"aelum-installer/backend/install/isocopy"
	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// User setup
// ============================================================================

// checkTargetAccountName rejects a username that already exists as a user or
// group in the installed system. This is the same collision useradd enforces,
// but checked before anything is changed so the error is clear. It also covers
// package-created accounts absent from the static early-validation list.
func checkTargetAccountName(name string) error {
	passwd, err := primitives.ReadFile(shared.MountRoot + "/etc/passwd")
	if err != nil {
		return fmt.Errorf("read target passwd: %w", err)
	}
	group, err := primitives.ReadFile(shared.MountRoot + "/etc/group")
	if err != nil {
		return fmt.Errorf("read target group: %w", err)
	}
	if kind, conflict := targetNameConflict(passwd, group, name); conflict {
		return fmt.Errorf("username %q is already used by a %s on the target system; choose another", name, kind)
	}
	return nil
}

// targetNameConflict reports whether name is an existing user or group in the
// given passwd/group file contents.
func targetNameConflict(passwd, group []byte, name string) (kind string, conflict bool) {
	if hasAccountLine(passwd, name) {
		return "user", true
	}
	if hasAccountLine(group, name) {
		return "group", true
	}
	return "", false
}

// hasAccountLine reports whether data has a colon-separated record named name.
func hasAccountLine(data []byte, name string) bool {
	prefix := name + ":"
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// Run copies what the live ISO ships, then creates the user account and writes the
// sudoers rule.
//
// The copy comes first, and it is a separate package (install/isocopy) because most
// of what it carries is not user setup - the seeded skeleton, the greeter config, the
// branding files and the snapshot-restore path. It comes first because useradd -m
// seeds the home from /etc/skel: the skeleton has to be in the target before the
// first home is created out of it.
func Run(ctx *shared.InstallContext) error {
	if err := primitives.ValidateMountedSystem(); err != nil {
		return err
	}

	if ctx.Username == "" {
		return nil
	}

	if err := checkTargetAccountName(ctx.Username); err != nil {
		return err
	}
	if len(ctx.Password) > shared.MaxPasswordLength {
		return fmt.Errorf("password must be at most %d UTF-8 bytes", shared.MaxPasswordLength)
	}
	if strings.ContainsAny(string(ctx.Password), "\n\r\x00") {
		return fmt.Errorf("password cannot contain newlines or NUL characters")
	}

	if err := isocopy.Run(ctx); err != nil {
		return err
	}

	var cmd *primitives.Command
	var output []byte
	var err error

	output, err = primitives.RunChroot("useradd", "-m", "-G", "wheel", "--", ctx.Username)
	if err != nil {
		return fmt.Errorf("failed to setup user: %w: %s", err, string(output))
	}
	if ctx.Logger != nil && len(output) > 0 {
		ctx.Logger.Command(string(output))
	}

	if len(ctx.Password) > 0 {
		cmd = primitives.Run("arch-chroot", shared.MountRoot, "chpasswd")
		cmd.Stdin = strings.NewReader(ctx.Username + ":" + string(ctx.Password) + "\n")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("passwd failed: %w: %s", err, string(output))
		}
		for k := range ctx.Password {
			ctx.Password[k] = 0
		}
	}

	cmd = primitives.Run("mkdir", "-p", shared.MountRoot+"/etc/sudoers.d")
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create sudoers.d dir: %w", err)
	}
	// Write with correct permissions directly (0440) to avoid a window where
	// the file is world-readable if the process crashes between write and chmod.
	sudoersPath := shared.MountRoot + "/etc/sudoers.d/wheel"
	if err := primitives.WriteFileMode(sudoersPath, "%wheel ALL=(ALL:ALL) ALL\n", 0o440); err != nil {
		return fmt.Errorf("failed to write sudoers: %w", err)
	}

	return nil
}
