<script lang="ts">
	/**
	 * Scrollable content container using the browser's default scrollbar.
	 * Used for language lists, disk lists, and other scrollable content.
	 *
	 * Props:
	 *   children   - Snippet for scrollable content
	 *   persistKey - optional stable key; when set, the scroll position is
	 *                saved while scrolling and on unmount, and restored on the
	 *                next mount. This survives the {#key} remount that
	 *                App.svelte applies to pages, so a list keeps its scroll
	 *                position when you switch pages (or modes) and come back.
	 */

	import { onMount, onDestroy } from 'svelte';

	// Scroll positions keyed by persistKey, surviving the component remounts
	// caused by the {#key} wrapper around pages in App.svelte. Kept on globalThis
	// (not a module-level const) because the running app re-evaluates this module
	// on each page remount — a module-level Map was observed empty on return, while
	// the window object persists for the app's lifetime.
	type ScrollStore = { __AELUM_SCROLL_POSITIONS?: Map<string | number, number> };
	const g = globalThis as unknown as ScrollStore;
	if (!g.__AELUM_SCROLL_POSITIONS) g.__AELUM_SCROLL_POSITIONS = new Map<string | number, number>();
	const SCROLL_POSITIONS = g.__AELUM_SCROLL_POSITIONS;

	let { children, persistKey }: {
		children: import('svelte').Snippet;
		persistKey?: string | number;
	} = $props();
	let box: HTMLDivElement | undefined;
	let saveScheduled = false;
	let restoring = false;

	function save(): void {
		// Use !== undefined (not truthiness): PAGE_IDS.WELCOME is 0, a valid key
		// that a boolean check would wrongly treat as "no key". Only capture while
		// the box is still in the document: during teardown the element is detached
		// and scrollTop reads 0, which would clobber the last live position the
		// scroll handler already saved.
		if (persistKey !== undefined && box && box.isConnected) {
			SCROLL_POSITIONS.set(persistKey, box.scrollTop);
		}
	}

	function scheduleSave(): void {
		if (saveScheduled || restoring) return;
		saveScheduled = true;
		requestAnimationFrame(() => {
			saveScheduled = false;
			save();
		});
	}

	onMount(() => {
		if (persistKey === undefined) return;
		const saved = SCROLL_POSITIONS.get(persistKey);
		if (saved === undefined) return;
		// The box (or its content) may not have finished laying out when onMount
		// runs, which would clamp scrollTop back to 0. Re-apply on subsequent
		// ticks until layout settles. Skip if the user has already scrolled, so
		// we never clobber an intentional scroll.
		let attempts = 0;
		let timer: ReturnType<typeof setTimeout> | undefined;
		const run = () => {
			if (box && box.scrollTop === 0) box.scrollTop = saved;
			if (++attempts < 3) timer = setTimeout(run, 0);
		};
		run();
		return () => {
			if (timer !== undefined) clearTimeout(timer);
		};
	});

	onDestroy(() => {
		save();
	});
</script>

<div class="scroll-box" bind:this={box} onscroll={scheduleSave}>
	{@render children()}
</div>

<style>
	.scroll-box {
		flex: 1;
		min-height: 0;
		height: 100%;
		overflow-y: auto;
		overflow-x: visible;
		display: flex;
		flex-direction: column;
		gap: var(--spacing-md);
		padding-inline-end: var(--spacing-lg);
	}
</style>
