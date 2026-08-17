<script lang="ts">
	/**
	 * Scrollable content container with consistent styling.
	 * Used for language lists, disk lists, and other scrollable content.
	 *
	 * Slots:
	 *   default - Content to scroll
	 */
</script>

<div class="scroll-box">
	<div class="scroll-inner">
		<slot />
	</div>
</div>

<style>
	.scroll-box {
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow: hidden;
		flex: 1;
	}

	.scroll-inner {
		flex: 1;
		overflow-y: auto;
		overflow-x: hidden;
		scrollbar-width: thin;
		scrollbar-color: var(--color-border) transparent;
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: var(--spacing-md);
	}

	.scroll-inner::-webkit-scrollbar {
		width: 6px;
	}

	.scroll-inner::-webkit-scrollbar-track {
		background: transparent;
	}

	.scroll-inner::-webkit-scrollbar-thumb {
		background: var(--color-border-light);
		border-radius: 3px;
	}

	.scroll-inner::-webkit-scrollbar-thumb:hover {
		background: var(--color-border);
	}
</style>
