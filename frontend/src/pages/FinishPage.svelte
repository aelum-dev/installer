<script lang="ts">
	import { t } from '$lib/i18n';
	import '$lib/layout.css';
</script>

<div class="page-content stretch">
		<h2 class="page-title">{t('page.finish.title')}</h2>
		<div class="finish">
			<div class="icon">✓</div>
			<p class="message">{t('page.finish.message')}</p>
		</div>
	</div>

<style>
	.finish {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--gap-md);
	}

	.icon {
		font-size: 4rem;
		color: var(--color-primary);
		line-height: 1;
	}

	.message {
		color: var(--color-text-muted);
		margin: 0;
		font-size: var(--font-size-lg);
		text-align: center;
	}
</style>
