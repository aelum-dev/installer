<script lang="ts">
	import { setContext, tick, type Component } from 'svelte';

	import AppHeader from '$components/AppHeader.svelte';
	import AppFooter from '$components/AppFooter.svelte';
	import ConfirmDialog from '$components/ConfirmDialog.svelte';

	import WelcomePage from '$pages/WelcomePage.svelte';
	import UserAccountPage from '$pages/UserAccountPage.svelte';
	import DiskSelectionPage from '$pages/DiskSelectionPage.svelte';
	import ConfirmationPage from '$pages/ConfirmationPage.svelte';
	import InstallationPage from '$pages/InstallationPage.svelte';
	import FinishPage from '$pages/FinishPage.svelte';

	import { PAGE_IDS, TOTAL_PAGES } from '$lib/types';
	import type { PageID } from '$lib/types';
	import { goNext, goBack, finish, canGoNext, WIZARD_NAV, type WizardNavigation } from '$lib/navigation';
	import { navState, setDryRun } from '$lib/store.svelte';
	import { isPageValid, getPageValidationReasons } from '$lib/page-validation';
	import { shouldAdvanceOnEnter } from '$lib/enter-advance';
	import { t, getIsRTL } from '$lib/i18n/index.svelte';
	import { pageFade } from '$lib/transitions';

	import { GetStartPage, ClearStartPage, GetDryRun } from '$lib/backend';

	// One entry per wizard page. Record<PageID, …> makes the compiler reject a
	// missing or unknown page, so this can't drift from PAGE_IDS.
	const PAGES: Record<PageID, Component> = {
		[PAGE_IDS.WELCOME]: WelcomePage,
		[PAGE_IDS.USER_ACCOUNT]: UserAccountPage,
		[PAGE_IDS.DISK_SELECTION]: DiskSelectionPage,
		[PAGE_IDS.CONFIRMATION]: ConfirmationPage,
		[PAGE_IDS.INSTALLATION]: InstallationPage,
		[PAGE_IDS.FINISH]: FinishPage,
	};

	const CurrentPage = $derived(PAGES[navState.currentPage as PageID]);

	// Jump to start page on init if configured
	async function initStartPage(): Promise<void> {
		try {
			const startPage = await GetStartPage();
			if (startPage >= 0 && startPage < TOTAL_PAGES) {
				navState.currentPage = startPage;
				await ClearStartPage();
			}
		} catch {
			// Ignore errors — start page not configured
		}

		try {
			setDryRun(await GetDryRun());
		} catch {
			// Ignore errors — dry run state not available
		}
	}

	initStartPage();

	// Validation error modal state: a list of reasons to show as bullets.
	let validationError = $state<string[] | null>(null);

	async function handleNext(): Promise<void> {
		const error = await goNext(navState.currentPage);
		if (error) {
			console.error(`[App] Navigation failed: ${error}`);
			validationError = [error];
		}
	}

	function handleValidationFailed(reasons: string[]): void {
		// Frontend validation failed — show modal
		validationError = reasons;
	}

	// The single "advance" path for both the footer's Next button and Enter:
	// validate first, then go (or list why not).
	function advance(): void {
		if (isPageValid(navState.currentPage)) {
			void handleNext();
		} else {
			handleValidationFailed(getPageValidationReasons(navState.currentPage));
		}
	}

	// Enter is the keyboard equivalent of Next. The predicate yields to controls
	// that own Enter and to open dialogs; the validation modal is tracked here
	// because its state is local to this component.
	function handleGlobalKeydown(event: KeyboardEvent): void {
		const canAdvance = shouldAdvanceOnEnter(event, {
			canAdvance: canGoNext(navState.currentPage),
			blocked: validationError !== null,
		});
		if (!canAdvance) return;

		event.preventDefault();
		advance();
	}

	function dismissValidationError(): void {
		validationError = null;
	}

	async function handleBack(): Promise<void> {
		if (navState.currentPage === PAGE_IDS.WELCOME) return;
		const error = await goBack(navState.currentPage);
		if (error) {
			console.error(`[App] Back failed: ${error}`);
			validationError = [error];
		}
	}

	async function handleFinish(): Promise<void> {
		const error = await finish();
		if (error) {
			console.error(`[App] Finish failed: ${error}`);
		}
	}

	// Share the navigation callbacks with pages via context, so a dialog or the
	// installation-complete handler can advance the wizard without dispatching
	// DOM events.
	setContext<WizardNavigation>(WIZARD_NAV, {
		next: handleNext,
		back: handleBack,
		finish: handleFinish,
	});

	// Move focus to the new page's heading when the wizard transitions, so
	// keyboard users land at the top of the content instead of on a detached
	// control (or the start of the document). This is an action on the keyed
	// page element, so it runs once per page mount; the initial mount is skipped
	// to leave boot focus alone.
	let initialPageFocused = false;

	function focusPageHeading(node: HTMLElement): void {
		if (!initialPageFocused) {
			initialPageFocused = true;
			return;
		}
		void tick().then(() => {
			const heading = node.querySelector<HTMLElement>('h1, h2');
			const target = heading ?? node;
			if (!target.hasAttribute('tabindex')) target.setAttribute('tabindex', '-1');
			// preventScroll: the page is still faded by its transition when
			// this runs, and a scrolling focus() would drag the
			// overflow:hidden ancestors along with it.
			target.focus({ preventScroll: true });
		});
	}
</script>

<svelte:window onkeydown={handleGlobalKeydown} />

<div class="app-layout" dir={getIsRTL() ? 'rtl' : 'ltr'}>
	<AppHeader />
	<main class="content-area">
		{#key navState.currentPage}
			<div
				class="page"
				use:focusPageHeading
				tabindex="-1"
				in:pageFade={{ duration: 200, delay: 140, zIndex: 2 }}
				out:pageFade={{ duration: 140, zIndex: 1, exiting: true }}
			>
				<CurrentPage />
			</div>
		{/key}
	</main>

	<AppFooter
		currentPage={navState.currentPage}
		onNext={advance}
		onBack={handleBack}
		onFinish={handleFinish}
	/>

	<!-- Validation error modal -->
	<!--
		onCancel must dismiss at the parent level (clear validationError), not just
		flip the child's `show` flag. The modal is MOUNTED by `{#if validationError}`
		and only VISIBLY shown via the child's `show`. Clicking the backdrop toggles
		only `show`, leaving validationError set, so the instance stays mounted but
		hidden and never re-appears on the next Next. Dismissing via onCancel tears
		the whole block down so the following Next recreates it fresh.
	-->
	{#if validationError}
		<ConfirmDialog
			show={validationError !== null}
			title={t('validation.modal.title')}
			reasons={validationError}
			confirmLabel={t('dialog.confirm.ok')}
			onConfirm={dismissValidationError}
			onCancel={dismissValidationError}
			showIcon={true}
			showCancel={false}
		/>
	{/if}
</div>

<style>
	.app-layout {
		display: flex;
		flex-direction: column;
		height: 100vh;
		width: 100vw;
		overflow: hidden;
		box-sizing: border-box;
		background-color: var(--color-bg-light);
		padding: var(--spacing-lg);
	}

	.content-area {
		flex: 1;
		position: relative;
		overflow: hidden;
		min-height: 0;
	}

	.page {
		/* Absolute so the outgoing and incoming pages overlap during the
		   transition instead of stacking to 200% height (which caused a layout
		   jump when the outgoing one unmounted). The incoming page is held at
		   opacity 0 until the outgoing one has faded, so they never blend. */
		position: absolute;
		inset: 0;
		z-index: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	/* Navigation moves focus here programmatically, so no focus ring is needed on
	   the page container or its heading (interactive controls keep their ring). */
	.page:focus,
	.page :global(h1:focus),
	.page :global(h2:focus) {
		outline: none;
	}
</style>
