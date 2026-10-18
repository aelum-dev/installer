<script lang="ts">
	import { fly } from 'svelte/transition';
	import { backOut } from 'svelte/easing';
	import { setContext, type Component } from 'svelte';

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
	import { goNext, goBack, finish, WIZARD_NAV, type WizardNavigation } from '$lib/navigation';
	import { navState, setDryRun } from '$lib/store.svelte';
	import { t, getIsRTL } from '$lib/i18n/index.svelte';

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

</script>

<div class="app-layout" dir={getIsRTL() ? 'rtl' : 'ltr'}>
	<AppHeader />
	<main class="content-area">
		{#key navState.currentPage}
			<div
				class="page"
				transition:fly={{ x: navState.direction === 'forward' ? -80 : 80, duration: 350, easing: backOut }}
			>
				<CurrentPage />
			</div>
		{/key}
	</main>

	<AppFooter
		currentPage={navState.currentPage}
		onNext={handleNext}
		onBack={handleBack}
		onFinish={handleFinish}
		onValidationFailed={handleValidationFailed}
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
		width: 100%;
		height: 100%;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}
</style>
