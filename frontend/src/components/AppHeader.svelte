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
		padding: 10px 24px;
		background-color: #e8e8e8;
		border-bottom: 1px solid #ccc;
		min-height: 44px;
	}

	.brand {
		font-size: 0.95rem;
		font-weight: 700;
		color: #333;
		text-transform: uppercase;
		letter-spacing: 1px;
	}
</style>
