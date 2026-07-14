<script lang="ts">
	import { PAGE_NAMES } from '$lib/types';

	let { currentPage }: { currentPage: number } = $props();
</script>

<header class="header">
	<span class="brand">AestheticOS</span>
</header>

<style>
	.header {
		display: flex;
		align-items: center;
		padding: var(--spacing-sm) var(--spacing-xl);
		background-color: var(--color-bg-header);
		border-bottom: 1px solid var(--color-border-light);
		min-height: var(--header-height);
	}

	.brand {
		font-size: 0.95rem;
		font-weight: var(--font-weight-700);
		color: var(--color-text);
		text-transform: uppercase;
		letter-spacing: 1px;
	}
</style>
