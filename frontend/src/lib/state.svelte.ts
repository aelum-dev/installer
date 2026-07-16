import type { DiskInfo } from '$lib/types';

// User account state
export let userAccountData = $state({
	username: '',
	password: '',
	confirmPassword: '',
});

// Disk selection state
export let diskSelectionData = $state({
	diskId: '',
	disk: null as DiskInfo | null,
});

// Confirm dialog state
let _showConfirmDialog = $state(false);

export const showConfirmDialog = {
	get value() { return _showConfirmDialog; },
	set value(v: boolean) { _showConfirmDialog = v; },
};
