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
		justify-content: space-between;
		min-height: var(--header-height);
		background-color: var(--color-bg-light);
	}

	.brand {
		font-size: 0.95rem;
		font-weight: var(--font-weight-700);
		color: var(--color-text);
		text-transform: uppercase;
		letter-spacing: 1px;
		margin-left: var(--spacing-lg);
	}
</style>
