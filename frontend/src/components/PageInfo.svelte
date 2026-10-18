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
		hidden = false,
		info,
	}: {
		icon?: string;
		title: string;
		subtitle?: string;
		/** Collapse the rail (used when disk selection switches to manual mode). */
		hidden?: boolean;
		/** Extra content under the subtitle (e.g. requirement hints). */
		info?: Snippet;
	} = $props();
</script>

<div class={['page-info', { 'page-info--hidden': hidden }]}>
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
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: var(--spacing-2xl);
		flex: 1;
		transition: opacity 0.2s ease, max-width 0.2s ease;
	}

	.page-info--hidden {
		opacity: 0;
		max-width: 0;
		overflow: hidden;
		padding: 0;
		margin: 0;
	}
</style>
