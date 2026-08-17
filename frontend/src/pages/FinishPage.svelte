<script lang="ts">
	import { t } from '$lib/i18n';
</script>

<div class="centered-page">
	<div class="centered-inner">
		<div class="finish">
			<div class="icon">✓</div>
			<p class="message">{t('page.finish.message')}</p>
		</div>
	</div>
</div>

<style>
	.centered-page {
		height: 100%;
		width: 100%;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		overflow: hidden;
	}

	.centered-inner {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--spacing-lg);
		width: 100%;
		padding-inline: var(--spacing-xl);
		box-sizing: border-box;
	}

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
