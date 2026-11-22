<script lang="ts">
	import type { Snippet } from 'svelte';
	import PageInfo from '$components/PageInfo.svelte';

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
	<PageInfo {icon} {title} {subtitle} {info} />
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

	.page-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		gap: var(--spacing-md);
	}

	.page-content--form {
		justify-content: center;
		align-items: flex-start;
		text-align: left;
		overflow: hidden;
		min-width: 0;
	}

	.page-content--summary {
		/* The summary centers itself through its own auto margins, and the column
		   starts at the top so an overflowing summary stays fully reachable by
		   scrolling — a justify-content: center would push an overflowing child
		   above the scroll origin, where nothing can reach it. Only the
		   confirmation page uses this variant. */
		justify-content: flex-start;
		align-items: flex-start;
		text-align: left;
		min-width: 0;
		overflow-y: auto;
	}

	.page-content--list {
		overflow: visible;
		/* Without min-width the column's automatic minimum (its widest row's
		   min-content: ellipsized text excluded, but nowrap chips, sizes and
		   marks counted) could push the column past the window edge, clipping
		   the row tint on the right — worse when selection added the mark. */
		min-width: 0;
	}
</style>
