<script lang="ts">
	import { PAGE_IDS, TOTAL_PAGES } from '$lib/types';
	import { canGoNext } from '$lib/navigation';
	import { t, getIsRTL } from '$lib/i18n';
	import Chevron from '$components/Chevron.svelte';

	let {
		currentPage,
		onNext,
		onBack,
		onFinish,
	}: {
		currentPage: number;
		onNext: () => void;
		onBack: () => void;
		onFinish: () => void;
	} = $props();

	const isLastPage = $derived(currentPage === PAGE_IDS.FINISH);
	const canAdvance = $derived(canGoNext(currentPage));
	// The base chevron points down; rotate it to face the travel direction. In
	// RTL the arrows flip: Next points left, Back points right.
	const nextArrow = $derived(getIsRTL() ? 'left' : 'right');
	const backArrow = $derived(getIsRTL() ? 'right' : 'left');
	// Installation and Finish are terminal, so neither control is shown there;
	// Welcome additionally has nothing to go back to.
	const showNext = $derived(canAdvance);
	const showBack = $derived(currentPage > PAGE_IDS.WELCOME && canAdvance);

	const steps = $derived(Array.from({ length: TOTAL_PAGES }, (_, i) => i));

	function getStepState(index: number): 'completed' | 'active' | 'pending' {
		if (index < currentPage) return 'completed';
		if (index === currentPage) return 'active';
		return 'pending';
	}
</script>

<footer class="footer">
	<!-- Next is first in the DOM so Tab reaches the primary action before Back;
	     CSS `order` keeps Back on the leading edge and Next on the trailing
	     edge, so the visual layout is unchanged. -->
	<div class="footer-next">
		{#if showNext}
			<button
				type="button"
				class="nav-btn"
				aria-label={t('nav.next')}
				onclick={onNext}
			>
				<Chevron direction={nextArrow} size={20} />
			</button>
		{:else if isLastPage}
			<button type="button" class="btn-reboot" onclick={onFinish}>
				{t('page.finish.reboot')}
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
	<div class="footer-back">
		{#if showBack}
			<button
				type="button"
				class="nav-btn nav-btn--back"
				aria-label={t('nav.back')}
				onclick={onBack}
			>
				<Chevron direction={backArrow} size={20} />
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

	/* `order` restores the visual sequence (Back, dots, Next) while the DOM
	   keeps Next first for Tab order. */
	.footer-back {
		display: flex;
		align-items: center;
		flex: 1;
		justify-content: flex-start;
		order: 1;
	}

	.footer-center {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: var(--spacing-sm);
		flex: 1;
		order: 2;
	}

	.footer-next {
		display: flex;
		align-items: center;
		flex: 1;
		justify-content: flex-end;
		order: 3;
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
		width: 40px;
		height: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--color-text);
		cursor: pointer;
		transition: background-color 0.15s ease, color 0.15s ease;
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
