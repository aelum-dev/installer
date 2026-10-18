<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * Full-height, horizontally-and-vertically centered page shell, shared by the
	 * Installation and Finish screens (which have no two-column layout).
	 */
	let { children }: { children: Snippet } = $props();
</script>

<div class="centered-page">
	<div class="centered-inner">
		{@render children()}
	</div>
</div>

<style>
	.centered-page {
		height: 100%;
		width: 100%;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		overflow: hidden;
	}

	.centered-inner {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--spacing-lg);
		width: 100%;
		padding-inline: var(--spacing-xl);
		box-sizing: border-box;
	}
</style>
