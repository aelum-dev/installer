<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * The wizard's left info rail: page icon, title, subtitle, and optional extra
	 * content. Shared by PageLayout and the disk-selection sidebar so the rail
	 * markup/styles live in one place.
	 */
	let {
		icon,
		title,
		subtitle,
		info,
	}: {
		icon?: string;
		title: string;
		subtitle?: string;
		/** Extra content under the subtitle (e.g. requirement hints). */
		info?: Snippet;
	} = $props();
</script>

<div class="page-info">
	{#if icon}
		<span class="nf-icon page-icon" aria-hidden="true">{icon}</span>
	{/if}
	<h2 class="page-info-title">{title}</h2>
	{#if subtitle}
		<p class="page-info-subtitle">{subtitle}</p>
	{/if}
	{@render info?.()}
</div>

<style>
	.page-info {
		/* The icon/title/subtitle stack is one block of writing, so it is spaced at
		   the rhythm of a paragraph. Anything a page adds below it (a location card,
		   a hint list) opens its own margin from here rather than widening this gap.
		   The rail is centred in the row: it is the page's reading matter, and it
		   sits at the height of the window rather than at the top of the column. */
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: var(--spacing-lg);
		flex: 1;
	}
</style>
