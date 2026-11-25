<script lang="ts">
	/**
	 * Scrollable content container using the browser's default scrollbar.
	 * Used for language lists, disk lists, and other scrollable content.
	 *
	 * Props:
	 *   children     - Snippet for scrollable content
	 *   persistKey   - optional stable key; when set, the scroll position is
	 *                  saved while scrolling and on unmount, and restored on the
	 *                  next mount. This survives the {#key} remount that
	 *                  App.svelte applies to pages, so a list keeps its scroll
	 *                  position when you switch pages (or modes) and come back.
	 *   scrollToCode - optional data-code of a row; on mount, when there is no
	 *                  saved position to restore, that row is brought to the top
	 *                  of the view — the selected language should be the first
	 *                  row a person sees, not something they hunt for. A saved
	 *                  position wins over this, so a revisit still lands where
	 *                  the person left the list.
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

	let { children, persistKey, scrollToCode }: {
		children: import('svelte').Snippet;
		persistKey?: string | number;
		scrollToCode?: string;
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
		const saved = persistKey === undefined ? undefined : SCROLL_POSITIONS.get(persistKey);
		if (saved === undefined && scrollToCode === undefined) return;
		// The box (or its content) may not have finished laying out when onMount
		// runs, which would clamp scrollTop back to 0. Re-apply on subsequent
		// ticks until layout settles. Skip if the user has already scrolled, so
		// we never clobber an intentional scroll.
		let attempts = 0;
		let timer: ReturnType<typeof setTimeout> | undefined;
		const run = () => {
			if (box) {
				if (saved !== undefined) {
					if (box.scrollTop === 0) box.scrollTop = saved;
				} else {
					scrollToRow();
				}
			}
			if (++attempts < 3) timer = setTimeout(run, 0);
		};
		run();
		return () => {
			if (timer !== undefined) clearTimeout(timer);
		};
	});

	// scrollToRow brings the row whose data-code matches scrollToCode to the top
	// of the view. The offset is recomputed on every attempt because a late font
	// load can move the row between them.
	function scrollToRow(): void {
		if (!box || scrollToCode === undefined) return;
		const row = box.querySelector<HTMLElement>(`[data-code="${scrollToCode}"]`);
		if (!row) return;
		const delta = row.getBoundingClientRect().top - box.getBoundingClientRect().top;
		if (delta !== 0) box.scrollTop += delta;
	}

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
		padding-inline-end: var(--spacing-sm);
	}
</style>
