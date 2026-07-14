import { writable } from 'svelte/store';

export const userAccountData = writable<{ username: string; password: string; confirmPassword: string }>({
	username: '',
	password: '',
	confirmPassword: '',
});
