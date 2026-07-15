import { writable } from 'svelte/store';
import type { DiskInfo } from '$lib/types';

export const userAccountData = writable<{ username: string; password: string; confirmPassword: string }>({
	username: '',
	password: '',
	confirmPassword: '',
});

export const diskSelectionData = writable<{ diskId: string; disk: DiskInfo | null }>({
	diskId: '',
	disk: null,
});

export const navigationRequest = writable<{ pending: boolean; page: number }>({
	pending: false,
	page: -1,
});
