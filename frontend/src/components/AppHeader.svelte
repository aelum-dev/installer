<script lang="ts">
	import { PAGE_NAMES } from '$lib/types';

	let { currentPage }: { currentPage: number } = $props();

	$effect(() => {
		document.title = `${PAGE_NAMES[currentPage]} — Arch Installer`;
	});
</script>

<header class="header">
	<h1>{PAGE_NAMES[currentPage]}</h1>
	<span class="brand">AestheticOS</span>
</header>

<style>
	.header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 24px;
		background-color: #e8e8e8;
		border-bottom: 1px solid #ccc;
		min-height: 44px;
	}

	.header h1 {
		font-size: 1.3rem;
		font-weight: 600;
		color: #333;
		margin: 0;
	}

	.brand {
		font-size: 0.95rem;
		font-weight: 700;
		color: #333;
		text-transform: uppercase;
		letter-spacing: 1px;
	}
</style>
