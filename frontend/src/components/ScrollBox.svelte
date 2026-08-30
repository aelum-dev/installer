<script lang="ts">
	/**
	 * Scrollable content container using the browser's default scrollbar.
	 * Used for language lists, disk lists, and other scrollable content.
	 *
	 * Props:
	 *   default - Snippet for scrollable content
	 */

	let { children }: { children: import('svelte').Snippet } = $props();
</script>

<div class="scroll-box">
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
