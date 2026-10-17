<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * The two-column wizard page shell: a centered info rail (icon, title,
	 * subtitle, optional extra content) beside a content column. Keeps the
	 * layout and its spacing identical across pages instead of duplicating the
	 * same markup and CSS in every page component.
	 */
	let {
		icon,
		title,
		subtitle,
		variant = 'form',
		wideInset = false,
		info,
		contentEl = $bindable(null),
		children,
	}: {
		icon?: string;
		title: string;
		subtitle?: string;
		/** `form` clips, `summary` allows overflow, `list` lets the child scroll. */
		variant?: 'form' | 'summary' | 'list';
		/** Slightly wider inline padding, used by the language list. */
		wideInset?: boolean;
		/** Extra content under the subtitle (e.g. requirement hints). */
		info?: Snippet;
		contentEl?: HTMLElement | null;
		children: Snippet;
	} = $props();
</script>

<div class={['page-layout', { 'page-layout--wide': wideInset }]}>
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
	<div class={['page-content', `page-content--${variant}`]} bind:this={contentEl}>
		{@render children()}
	</div>
</div>

<style>
	.page-layout {
		display: flex;
		flex-direction: row;
		height: 100%;
		width: 100%;
		padding-inline: var(--spacing-xl);
		box-sizing: border-box;
		gap: var(--gap-xl);
	}

	.page-layout--wide {
		padding-inline: calc(var(--spacing-xl) + 8px);
	}

	.page-info {
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: var(--spacing-2xl);
		flex: 1;
	}

	.page-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.page-content--form {
		justify-content: center;
		align-items: flex-start;
		text-align: left;
		overflow: hidden;
		min-width: 0;
	}

	.page-content--summary {
		justify-content: center;
		align-items: flex-start;
		text-align: left;
		min-width: 0;
	}

	.page-content--list {
		overflow: visible;
	}
</style>
