import type { DiskInfo, FileSystem } from '$lib/types';

// Dry-run state (set by backend on startup if no sudo access)
let _isDryRun = $state(false);
export function setDryRun(value: boolean) { _isDryRun = value; }
export function getIsDryRun(): boolean { return _isDryRun; }

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
	fileSystem: '' as FileSystem | '',
});

// Confirm dialog state
let _showConfirmDialog = $state(false);

export const showConfirmDialog = {
	get value() { return _showConfirmDialog; },
	set value(v: boolean) { _showConfirmDialog = v; },
};

// Page validation state (true = current page is valid, Next button enabled)
export let pageValid = $state({ value: true });
