<script lang="ts">
	import { t } from '$lib/i18n';
	import { installState } from '$lib/store.svelte';
	import CenteredPage from '$components/CenteredPage.svelte';

	// The screen a stopped install ends on, reached when the person closes the
	// window during an install: the run is cancelled, the disk is released, and the
	// window stays open on this page instead of disappearing mid-step. It states
	// what happened to the disk and nothing else — the Exit control lives in the
	// footer, where every other page's Next sits.
</script>

<CenteredPage>
	<div class="cancelled">
		<h2 class="page-title">{t('page.cancelled.title')}</h2>
		<p class="message">{t('page.cancelled.body', { disk: installState.diskId })}</p>
	</div>
</CenteredPage>

<style>
	.cancelled {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--gap-md);
		width: 100%;
		max-width: 600px;
	}

	/* The Installation page's title, so the two terminal screens of that step
	   read as one screen that stopped. */
	.page-title {
		margin: 0 0 var(--spacing-lg) 0;
		font-size: var(--font-size-xl);
		font-weight: var(--font-weight-700);
		line-height: 1.2;
		color: var(--color-text);
		text-align: center;
	}

	.message {
		margin: 0;
		font-size: var(--font-size-lg);
		color: var(--color-text-muted);
		text-align: center;
	}
</style>
