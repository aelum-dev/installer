import { describe, it, expect, afterEach } from 'vitest';
import { toggleLvm, setFileSystem, installState } from './store.svelte';

afterEach(() => {
	// Reset LVM/filesystem state between tests.
	installState.lvmEnabled = false;
	installState.fileSystem = 'btrfs';
});

describe('toggleLvm', () => {
	it('defaults to off', () => {
		expect(installState.lvmEnabled).toBe(false);
	});

	it('flips enabled on without changing the filesystem', () => {
		installState.fileSystem = 'ext4';
		toggleLvm();
		expect(installState.lvmEnabled).toBe(true);
		expect(installState.fileSystem).toBe('ext4');
	});

	it('flips enabled back off', () => {
		installState.lvmEnabled = true;
		toggleLvm();
		expect(installState.lvmEnabled).toBe(false);
	});
});

describe('setFileSystem', () => {
	it('keeps LVM on when btrfs is selected', () => {
		installState.lvmEnabled = true;
		setFileSystem('btrfs');
		expect(installState.fileSystem).toBe('btrfs');
		expect(installState.lvmEnabled).toBe(true);
	});

	it('keeps LVM on when ext4 is selected (ext4 is supported on LVM)', () => {
		installState.lvmEnabled = true;
		setFileSystem('ext4');
		expect(installState.fileSystem).toBe('ext4');
		expect(installState.lvmEnabled).toBe(true);
	});
});
