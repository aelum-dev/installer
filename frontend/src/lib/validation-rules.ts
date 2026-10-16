// Single source of truth for user-account validation rules.
//
// Both the inline field validator (`validate.ts`) and the shared store's modal
// validation reasons (`store.svelte.ts`) enforce the same rules; before this
// module each kept its own copy of the regexes and the reserved-name list, so
// they could silently drift apart. The rules mirror the Go backend
// (`backend/shared/validators.go` and `config.go`).

// A Linux login name starts with a lowercase letter or underscore.
export const USERNAME_FIRST_CHAR = /^[a-z_]/;

// ...and otherwise contains only lowercase letters, digits, hyphens, underscores.
export const USERNAME_ALLOWED_CHARS = /^[a-z0-9_-]+$/;

// System accounts that cannot be used as the installer's user name.
// Mirrors backend/shared/config.go ReservedUsernames.
export const RESERVED_USERNAMES = new Set([
	'root', 'bin', 'daemon', 'adm', 'lp', 'sync', 'shutdown', 'halt',
	'mail', 'news', 'uucp', 'operator', 'games', 'gopher', 'ftp',
	'nobody', 'dbus', 'systemd', 'systemd-journal', 'systemd-network',
	'systemd-resolve', 'systemd-timesync', 'systemd-bus-proxy',
	'systemd-coredump', 'tss', 'ntp', 'sshd', 'polkitd', 'colord',
	'gdm', 'gnome-initial-setup', 'gdm3', 'lightdm', 'pulse', 'rtkit',
	'avahi', 'cups', 'usbmux', 'kernoops', 'messagebus', 'statd',
	'rpcuser', 'rpc', 'nfsnobody', 'radvd', 'chrony', 'qemu', 'libvirt',
	'smmsp', 'postfix', 'www-data', 'proxy', 'bind', 'staff', 'man',
	'printcap', 'input', 'mailnull', 'list', 'irc', 'gnats', 'backup',
]);
