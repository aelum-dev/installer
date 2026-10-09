import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { getTheme, setTheme, toggleTheme, STORAGE_KEY } from './theme.svelte';

const ROOT = document.documentElement;

function setDomTheme(theme: 'light' | 'dark') {
	ROOT.setAttribute('data-theme', theme);
}

beforeEach(() => {
	localStorage.clear();
	setDomTheme('light');
});

afterEach(() => {
	localStorage.clear();
	ROOT.removeAttribute('data-theme');
});

describe('theme', () => {
	it('setTheme updates internal state, <html> and storage', () => {
		setTheme('dark');
		expect(getTheme()).toBe('dark');
		expect(ROOT.getAttribute('data-theme')).toBe('dark');
		expect(localStorage.getItem(STORAGE_KEY)).toBe('dark');
	});

	it('toggleTheme flips light <-> dark', () => {
		setTheme('light');
		toggleTheme();
		expect(getTheme()).toBe('dark');
		toggleTheme();
		expect(getTheme()).toBe('light');
	});

	it('setTheme overwrites a previously stored choice', () => {
		setTheme('dark');
		expect(localStorage.getItem(STORAGE_KEY)).toBe('dark');
		setTheme('light');
		expect(localStorage.getItem(STORAGE_KEY)).toBe('light');
	});
});
