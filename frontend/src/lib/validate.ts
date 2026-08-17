import { MaxUsernameLength, MinPasswordLength, MinUsernameLength } from './constants';

export interface ValidationResult {
	username: string | null;
	password: string | null;
	confirmPassword: string | null;
}

export interface ValidationState {
	usernameTouched: boolean;
	passwordTouched: boolean;
	confirmPasswordTouched: boolean;
}

// UsernameFirstCharRegex validates the first character of a Linux login name.
// Must match backend/shared/validators.go UsernameFirstCharRegex exactly.
const UsernameFirstCharRegex = /^[a-z_]/;

// UsernameAllowedCharsRegex validates all allowed characters in a Linux login name.
// Must match backend/shared/validators.go UsernameAllowedCharsRegex exactly.
const UsernameAllowedCharsRegex = /^[a-z0-9_-]+$/;

// Reserved usernames that cannot be used.
// Mirrors backend/shared/config.go ReservedUsernames map.
const ReservedUsernames = new Set([
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

export function validateUserAccount(
	username: string,
	password: string,
	confirmPassword: string,
	state: ValidationState,
	messages?: {
		usernameRequired: string;
		usernameMinLength: string;
		usernameMaxLength: string;
		usernameInvalidFirstChar: string;
		usernameInvalidChars: string;
		usernameReserved: string;
		passwordRequired: string;
		passwordMinLength: string;
		passwordsMatch: string;
		passwordsNoMatch: string;
	}
): ValidationResult {
	const msg = messages ?? {
		usernameRequired: 'Username is required',
		usernameMinLength: `Username must be at least ${MinUsernameLength} characters`,
		usernameMaxLength: `Username must be at most ${MaxUsernameLength} characters`,
		usernameInvalidFirstChar: 'Username must start with a lowercase letter or underscore',
		usernameInvalidChars: 'Username must contain only lowercase letters, numbers, hyphens, and underscores',
		usernameReserved: 'Username \'{name}\' is reserved',
		passwordRequired: 'Password is required',
		passwordMinLength: `Password must be at least ${MinPasswordLength} characters`,
		passwordsMatch: 'Please confirm your password',
		passwordsNoMatch: 'Passwords do not match',
	};

	let usernameError: string | null = null;

	let passwordError: string | null = null;
	let confirmPasswordError: string | null = null;

	if (state.usernameTouched) {
		if (!username.trim()) {
			usernameError = msg.usernameRequired;
		} else if (username.length < MinUsernameLength) {
			usernameError = msg.usernameMinLength;
		} else if (username.length > MaxUsernameLength) {
			usernameError = msg.usernameMaxLength;
		} else if (!UsernameFirstCharRegex.test(username)) {
			usernameError = msg.usernameInvalidFirstChar;
		} else if (!UsernameAllowedCharsRegex.test(username)) {
			usernameError = msg.usernameInvalidChars;
		} else if (ReservedUsernames.has(username.toLowerCase())) {
			usernameError = msg.usernameReserved.replace('{name}', username);
		}
	}

	if (state.passwordTouched) {
		if (!password) {
			passwordError = msg.passwordRequired;
		} else if (password.length < MinPasswordLength) {
			passwordError = msg.passwordMinLength;
		}
	}

	if (state.confirmPasswordTouched) {
		if (!confirmPassword) {
			confirmPasswordError = msg.passwordsMatch;
		} else if (confirmPassword !== password) {
			confirmPasswordError = msg.passwordsNoMatch;
		}
	}

	return {
		username: usernameError,
		password: passwordError,
		confirmPassword: confirmPasswordError,
	};
}
