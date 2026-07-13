export type PageID = 0 | 1 | 2 | 3 | 4;

export const PAGE_NAMES: Record<number, string> = {
	0: 'Welcome',
	1: 'User Account',
	2: 'Disk Selection',
	3: 'Installation',
	4: 'Finish',
};

// Transition direction for animations.
export type TransitionDirection = 'forward' | 'backward';
