package configure

import (
	"fmt"
	"os"
	"strings"

	"aelum-installer/backend/install/primitives"
	"aelum-installer/backend/shared"
)

// ============================================================================
// LUKS unlock feedback (installed system, LUKS only)
// ============================================================================
//
// A plain sd-encrypt boot gives the user nothing but a bare passphrase field
// for every attempt, and if the passphrase tries are exhausted it drops to an
// emergency shell. This wires up three things the Aelum theme can act on:
//
//  1. luks:locked     a plymouth-start.service drop-in, emitted into the
//                     initramfs only. Tells the theme a passphrase prompt is
//                     coming, so it holds the boot bar back instead of showing
//                     progress before the root volume is unlocked.
//  2. luks:unlocked   ExecStartPost on the root volume's systemd-cryptsetup
//                     instance. The only point at which the installed system
//                     may show boot progress.
//  3. a retry message and a poweroff on exhausted tries: systemd-cryptsetup
//                     re-asks for a rejected passphrase without any unit event,
//                     so a small initramfs watcher watches the ask-password
//                     directory and the OnFailure= hook shuts the machine down
//                     when the root instance gives up.
//
// All of it rides in the initramfs (the installer adds the aelum-luks-hint hook
// to HOOKS and bakes these units/fragments in), so the installed system has no
// long-lived watcher and no lingering units.

// luksHintHookName is the mkinitcpio hook that bakes this feature into the
// installed system's initramfs. It must sit after "systemd" (which defines
// add_systemd_unit/add_systemd_drop_in) and after "plymouth".
const luksHintHookName = "aelum-luks-hint"

// Paths written inside the installed system.
const (
	luksHintScriptPath      = "/usr/lib/aelum/luks-hint.sh"
	luksHintServicePath     = "/usr/lib/systemd/system/aelum-luks-hint.service"
	luksPoweroffUnitPath    = "/usr/lib/systemd/system/aelum-luks-poweroff.service"
	luksHintInstallHookPath = "/usr/lib/initcpio/install/aelum-luks-hint"
)

// luksCryptsetupDropInPath is the per-instance drop-in for the encrypted root
// volume. It is scoped to the root instance (build-time constant
// LuksMapperName), not the systemd-cryptsetup@ template, so a secondary volume
// can never power the machine off on its own.
func luksCryptsetupDropInPath() string {
	return fmt.Sprintf("/etc/systemd/system/systemd-cryptsetup@%s.service.d/aelum-luks.conf", shared.LuksMapperName)
}

// installLuksHint writes the initramfs units, the ask-password watcher and the
// mkinitcpio install hook into the installed system. Best-effort callers: the
// initramfs hook is only added to HOOKS when every file is in place.
func installLuksHint(ctx *shared.InstallContext) error {
	files := []struct {
		path    string
		content string
		mode    os.FileMode
	}{
		{luksHintScriptPath, luksHintScript, 0o755},
		{luksHintServicePath, luksHintService, 0o644},
		{luksPoweroffUnitPath, luksPoweroffUnit, 0o644},
		{luksCryptsetupDropInPath(), luksCryptsetupDropIn, 0o644},
		{luksHintInstallHookPath, fmt.Sprintf(luksHintInstallHook, shared.LuksMapperName), 0o644},
	}
	for _, f := range files {
		if err := writeSystemFile(f.path, f.content, f.mode); err != nil {
			return err
		}
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("Installed LUKS unlock feedback (held-back boot bar, retry message, poweroff on exhausted tries)")
	}
	return nil
}

// writeSystemFile writes a file inside the installed system with an explicit
// mode. primitives.WriteFile always uses 0644, but the watcher must be
// executable so add_binary preserves that bit when baking the initramfs.
func writeSystemFile(targetPath, content string, mode os.FileMode) error {
	if err := primitives.WriteFile(targetPath, content); err != nil {
		return err
	}
	hostPath := targetPath
	if !strings.HasPrefix(targetPath, shared.MountRoot) {
		hostPath = shared.MountRoot + targetPath
	}
	if err := primitives.Chmod(hostPath, mode); err != nil {
		return fmt.Errorf("failed to set mode %#o on %s: %w", mode, targetPath, err)
	}
	return nil
}

// luksHintService starts the ask-password watcher in the initramfs. It is
// pulled in by the plymouth-start.service drop-in below (not enabled in the
// real root), and is killed when the initramfs switches root.
const luksHintService = `# SPDX-License-Identifier: GPL-3.0
#
# Aelum LUKS unlock retry hint. Started only in the initramfs, by a
# plymouth-start.service drop-in, and killed when the initramfs switches root.
# See /usr/lib/aelum/luks-hint.sh.

[Unit]
Description=Aelum LUKS unlock retry hint
DefaultDependencies=no
After=plymouth-start.service

[Service]
Type=simple
ExecStart=/usr/lib/aelum/luks-hint.sh
Restart=no
`

// luksPoweroffUnit is pulled in by OnFailure= on the root volume's
// systemd-cryptsetup instance when its passphrase tries are exhausted
// (systemd-cryptsetup's default tries=3). It shows why, then powers off.
const luksPoweroffUnit = `# SPDX-License-Identifier: GPL-3.0
#
# Aelum LUKS unlock failure. Pulled in by OnFailure= on the encrypted root
# volume's systemd-cryptsetup instance when its passphrase tries are exhausted:
# show why, then power the machine off instead of dropping to an emergency
# shell. The 3s pause below is a courtesy so the message can be read; drop it
# for an immediate poweroff.

[Unit]
Description=Aelum LUKS unlock failure: power off
DefaultDependencies=no
After=plymouth-start.service

[Service]
Type=oneshot
ExecStart=-/usr/bin/plymouth display-message --text="Too many incorrect passphrases. Powering off..."
ExecStartPost=-/usr/bin/sleep 3
ExecStartPost=/usr/bin/systemctl poweroff
`

// luksCryptsetupDropIn is the per-instance drop-in for the encrypted root
// volume. systemd-cryptsetup opens the root in the initramfs; its success has
// no other signal, and a failure must not fall through to the emergency shell.
const luksCryptsetupDropIn = `# SPDX-License-Identifier: GPL-3.0
#
# Aelum LUKS unlock feedback for the encrypted root volume instance.
#
#   ExecStartPost  the root volume opened with a correct passphrase: tell the
#                  Plymouth theme it may show boot progress (luks:unlocked).
#   OnFailure      systemd-cryptsetup exhausted its passphrase tries: power the
#                  machine off instead of dropping to an emergency shell.
#
# Scoped to the root instance (systemd-cryptsetup@aelum.service), not the
# template, so a secondary volume can never power the machine off on its own.

[Unit]
OnFailure=aelum-luks-poweroff.service

[Service]
ExecStartPost=-/usr/bin/plymouth update --status=luks:unlocked
`

// luksHintScript polls the ask-password directory for the rejected-passphrase
// retry. systemd-cryptsetup verifies the passphrase inside the service that
// asked for it and, on a rejected one, simply asks again: it creates a NEW ask
// file under /run/systemd/ask-password with the same Id=cryptsetup:<source> as
// the previous attempt. There is no systemd unit event for that re-ask, so this
// watcher is the only reliable signal.
//
// A fresh Id is a different volume's first prompt; the theme renders systemd's
// own prompt line for it (e.g. "Please enter passphrase for disk aelum:"), so
// this watcher only clears any message left over from another volume.
const luksHintScript = `#!/usr/bin/bash
#
# SPDX-License-Identifier: GPL-3.0
#
# Aelum LUKS unlock retry hint (initramfs).
#
# See the installer's backend/install/configure/luks_hint.go for the rationale.
# This runs only in the initramfs; the initramfs kills it when switching root.

set -u

ask_dir=/run/systemd/ask-password
declare -A seen

scan() {
    local file line id key base
    shopt -s nullglob
    for file in "$ask_dir"/ask.*; do
        id=
        while IFS= read -r line; do
            case "$line" in
                Id=*) id=${line#Id=} ;;
            esac
        done < "$file"
        [[ $id == cryptsetup:* ]] || continue

        key=${id//[^A-Za-z0-9]/_}
        base=${file##*/}

        if [[ -n ${seen[$key]:-} ]]; then
            if [[ ${seen[$key]} != "$base" ]]; then
                # Same volume, new ask file: the previous passphrase was rejected.
                seen[$key]=$base
                /usr/bin/plymouth display-message --text="Incorrect passphrase. Please try again." >/dev/null 2>&1 || :
            fi
        else
            # First prompt for this volume: drop any message left by another one.
            seen[$key]=$base
            /usr/bin/plymouth hide-message >/dev/null 2>&1 || :
        fi
    done
    shopt -u nullglob
}

while :; do
    scan
    sleep 1
done
`

// luksHintInstallHook is the mkinitcpio install hook (a %s template for the
// mapper name). It must sit after "systemd" and "plymouth" in HOOKS.
const luksHintInstallHook = `#!/bin/bash
#
# SPDX-License-Identifier: GPL-3.0
#
# Build hook for the Aelum LUKS unlock feedback on the installed system.
#
# Bakes in:
#   - aelum-luks-hint.service        retry watcher (a rejected passphrase has no
#                                    systemd event, so it polls the ask files)
#   - aelum-luks-poweroff.service    powers off once the passphrase tries are
#                                    exhausted
#   - the systemd-cryptsetup@<mapper> drop-in
#                                    signals luks:unlocked on success and pulls
#                                    in the poweroff unit on failure
#   - a plymouth-start.service drop-in
#                                    signals luks:locked so the theme holds the
#                                    boot bar back until the root is unlocked
#
# The plymouth-start drop-in is emitted straight into the initramfs
# (add_systemd_drop_in writes to stdin), so the installed system never starts
# the watcher outside of boot.
#
# Keep after "systemd" and "plymouth" in HOOKS.

build() {
    add_binary /usr/bin/bash
    add_binary /usr/lib/aelum/luks-hint.sh

    add_systemd_unit aelum-luks-hint.service
    add_systemd_unit aelum-luks-poweroff.service

    add_file /etc/systemd/system/systemd-cryptsetup@%s.service.d/aelum-luks.conf

    add_systemd_drop_in plymouth-start.service aelum-luks-hint <<'EOF'
[Unit]
Wants=aelum-luks-hint.service

[Service]
ExecStartPost=-/usr/bin/plymouth update --status=luks:locked
EOF
}

help() {
    cat <<HELPEOF
Pushes LUKS unlock feedback to Plymouth on the installed system: it holds the
boot progress bar back until the root volume is unlocked, shows a message when
a passphrase is rejected, and powers the machine off when the passphrase tries
are exhausted. Needs "systemd" and "plymouth" earlier in HOOKS.
HELPEOF
}
`
