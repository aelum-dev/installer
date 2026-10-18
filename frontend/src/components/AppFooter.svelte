<script lang="ts">
	import { PAGE_IDS, TOTAL_PAGES } from '$lib/types';
	import { isPageValid, getPageValidationReasons } from '$lib/page-validation';
	import { t, getIsRTL } from '$lib/i18n';

	let {
		currentPage,
		onNext,
		onBack,
		onFinish,
		onValidationFailed,
	}: {
		currentPage: number;
		onNext: () => void;
		onBack: () => void;
		onFinish: () => void;
		onValidationFailed?: (reasons: string[]) => void;
	} = $props();

	const isLastPage = $derived(currentPage === PAGE_IDS.FINISH);
	const isInstallationPage = $derived(currentPage === PAGE_IDS.INSTALLATION);
	const mirrorArrow = $derived(getIsRTL());
	const showBack = $derived(currentPage > PAGE_IDS.WELCOME && !isLastPage && !isInstallationPage);
	const showNext = $derived(!isLastPage && !isInstallationPage);

	const steps = $derived(Array.from({ length: TOTAL_PAGES }, (_, i) => i));

	function getStepState(index: number): 'completed' | 'active' | 'pending' {
		if (index < currentPage) return 'completed';
		if (index === currentPage) return 'active';
		return 'pending';
	}

	// Next button is always enabled, but validates before proceeding. On
	// failure the modal lists every reason (rendered as a plain-text list).
	const nextValidationReasons = $derived(getPageValidationReasons(currentPage));
</script>

<footer class="footer">
	<div class="footer-left">
		{#if showBack}
			<button
				type="button"
				class="nav-btn nav-btn--back"
				aria-label={t('nav.back')}
				onclick={onBack}
			>
				<svg class={['nav-arrow', { 'nav-arrow--mirror': mirrorArrow }]} viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
					<path d="M15 18l-6-6 6-6"/>
				</svg>
			</button>
		{/if}
	</div>
	<div class="footer-center">
		{#each steps as step, index}
			<div
				class={['step-dot', `step-dot--${getStepState(index)}`]}
				aria-label={`Step ${index + 1}`}
				aria-current={index === currentPage ? 'step' : undefined}
			></div>
		{/each}
	</div>
	<div class="footer-right">
		{#if showNext}
			<button
				type="button"
				class="nav-btn"
				aria-label={t('nav.next')}
				onclick={() => {
					if (isPageValid(currentPage)) {
						onNext();
					} else {
						onValidationFailed?.(nextValidationReasons);
					}
				}}
			>
				<svg class={['nav-arrow', { 'nav-arrow--mirror': mirrorArrow }]} viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
					<path d="M9 18l6-6-6-6"/>
				</svg>
			</button>
		{:else if isLastPage}
			<button type="button" class="btn-reboot" onclick={onFinish}>
				{t('page.finish.reboot')}
			</button>
		{/if}
	</div>
</footer>

<style>
	.footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: var(--spacing-lg);
		min-height: 40px;
	}

	.footer-left {
		display: flex;
		align-items: center;
		flex: 1;
		justify-content: flex-start;
	}

	.footer-center {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--spacing-sm);
		flex: 1;
	}

	.footer-right {
		display: flex;
		align-items: center;
		flex: 1;
		justify-content: flex-end;
	}

	.step-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		transition: all 0.2s ease;
	}

	.step-dot--pending {
		background-color: var(--color-border-light);
	}

	.step-dot--active {
		background-color: var(--color-primary);
		transform: scale(1.2);
	}

	.step-dot--completed {
		background-color: var(--color-primary);
	}

	.nav-btn {
		background: transparent;
		border: none;
		border-radius: 50%;
		aspect-ratio: 1;
		padding: 8px;
		display: flex;
		justify-content: center;
		align-items: center;
		color: var(--color-text);
		cursor: pointer;
		transition: background-color 0.15s ease, color 0.15s ease;
	}

	.nav-arrow {
		display: block;
		transition: transform 0.15s ease;
	}

	.nav-arrow--mirror {
		transform: scaleX(-1);
	}

	.nav-btn:focus-visible {
		outline: 2px solid var(--color-focus);
		outline-offset: 2px;
	}

	.nav-btn:hover:not(:disabled) {
		background-color: var(--color-bg-hover);
	}


	.btn-reboot {
		font-family: inherit;
		font-weight: 600;
		font-size: var(--font-size-md);
		padding: 10px 24px;
		border-radius: var(--radius-xl);
		border: 1.5px solid var(--color-primary);
		background: transparent;
		color: var(--color-primary);
		cursor: pointer;
		transition: all 0.15s ease;
		display: inline-flex;
		align-items: center;
	}

	.btn-reboot:hover {
		background-color: var(--color-primary);
		color: var(--color-on-primary);
		border-color: var(--color-primary);
	}
</style>
