<script lang="ts">
	import { getContext } from 'svelte';
	import { installState, syncEncryptionToBackend } from '$lib/store.svelte';
	import { t } from '$lib/i18n';
	import { StartInstallation, EventsOn, LogInfo } from '$lib/backend';
	import { WIZARD_NAV, type WizardNavigation } from '$lib/navigation';
	import CenteredPage from '$components/CenteredPage.svelte';

	const nav = getContext<WizardNavigation>(WIZARD_NAV);

	const EVENT_PROGRESS = 'installation:progress';
	const EVENT_COMPLETE = 'installation:complete';
	const EVENT_ERROR    = 'installation:error';

	let currentStepId = $state('');
	let currentStepLabel = $state('');
	let progress = $state(0);
	let isRunning = $state(false);
	let isComplete = $state(false);
	let hasTransitioned = $state(false);
	let elapsedSeconds = $state(0);
	let errorMessage = $state('');

	$effect(() => {
		if (!isRunning) return;
		const interval = setInterval(() => { elapsedSeconds++; }, 1000);
		return () => clearInterval(interval);
	});

	$effect(() => {
		// The backend sends a stable stepId (e.g. "format_disk"); the UI localizes
		// it as `step.<id>`. No label parsing or prefix stripping is needed.
		const handleProgress = (data: { stepId: string; label: string; percent: number; phase: string }) => {
			if (data.stepId && data.stepId !== currentStepId) {
				currentStepId = data.stepId;
				currentStepLabel = t(`step.${data.stepId}`);
				progress = 0;
			}
			if (data.phase === 'running') {
				if (data.percent >= progress) progress = data.percent;
				errorMessage = '';
			} else if (data.phase === 'done') {
				progress = 100;
			}
		};

		const handleComplete = () => {
			isRunning = false;
			isComplete = true;
			progress = 100;
			errorMessage = '';
			LogInfo('[InstallationPage] Installation complete');
		};

		const handleError = (_data: unknown, message: string) => {
			isRunning = false;
			errorMessage = message;
			LogInfo(`[InstallationPage] Installation error: ${message}`);
		};

		const offProgress = EventsOn(EVENT_PROGRESS, handleProgress);
		const offComplete = EventsOn(EVENT_COMPLETE, handleComplete);
		const offError = EventsOn(EVENT_ERROR, handleError);

		return () => { offProgress(); offComplete(); offError(); };
	});

	$effect(() => {
		if (!isComplete || hasTransitioned) return;
		hasTransitioned = true;
		setTimeout(() => {
			nav.next();
		}, 1000);
	});

	// Entering this page starts the install. Push the committed LUKS state to
	// the Go backend first — the backend decides whether to pass --luks (and
	// which passphrase to feed the CLI on stdin); the confirmation note is
	// derived purely from the frontend store.
	//
	// syncEncryptionToBackend awaits every binding call on purpose: the Wails
	// runtime dispatches each call on its own goroutine, so a fire-and-forget
	// sync races StartInstallation() and can hand the CLI --luks before the
	// passphrase has landed — which aborts the install at 0%.
	async function init(): Promise<void> {
		isRunning = true;
		try {
			await syncEncryptionToBackend();
			await StartInstallation();
		} catch (err) {
			isRunning = false;
			errorMessage = err instanceof Error ? err.message : 'Failed to start installation';
			LogInfo(`[InstallationPage] ${errorMessage}`);
		}
	}

	init();

	function formatTime(s: number): string {
		const mins = Math.floor(s / 60);
		const secs = s % 60;
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}

	const currentLabel = $derived(
		isComplete ? t('page.installation.completed') :
		errorMessage ? errorMessage :
		currentStepLabel || t('page.installation.initializing')
	);
</script>

<CenteredPage>
	<div class="content-wrapper">
		<h2 class="page-title">{t('page.installation.title')}</h2>
		{#if installState.diskId}
			<p class="hint">
				{t('page.installation.target')}: <strong>{installState.diskId}</strong>
			</p>
		{/if}
		<div class="progress-container">
			<div class="progress-bar-bg">
				<div
					class:running={isRunning}
					class="progress-bar-fill"
					style="--progress: {progress}%"
				></div>
			</div>
			<div class="progress-info">
				<span class="progress-percent">{progress}%</span>
				<span class="progress-time">{formatTime(elapsedSeconds)}</span>
			</div>
		</div>
		<p class="step-label">{currentLabel}</p>
	</div>
</CenteredPage>

<style>
	.hint {
		margin: 0;
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
		text-align: center;
	}

	.progress-container {
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: var(--gap-xs);
	}

	.progress-bar-bg {
		width: 100%;
		height: 14px;
		background: var(--color-white-glow);
		border-radius: var(--radius-pill);
		overflow: hidden;
	}

	.progress-bar-fill {
		height: 100%;
		width: var(--progress);
		background: var(--color-progress);
		border-radius: var(--radius-pill);
		transition: width 0.5s ease;
		position: relative;
		overflow: hidden;
	}

	.progress-bar-fill.running::after {
		content: '';
		position: absolute;
		top: 0;
		left: -40%;
		width: 40%;
		height: 100%;
		background: linear-gradient(90deg, transparent, var(--color-white-glow-strong), transparent);
		animation: slide 1.8s ease-in-out infinite;
	}

	@keyframes slide {
		0% { left: -40%; }
		100% { left: 100%; }
	}

	.progress-info {
		display: flex;
		justify-content: space-between;
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
	}

	.progress-percent {
		font-weight: var(--font-weight-700);
		color: var(--color-progress);
	}

	.step-label {
		margin: 0;
		font-size: var(--font-size-base-lg);
		font-weight: var(--font-weight-600);
		color: var(--color-text);
		min-height: 1.5em;
		transition: opacity 0.2s;
	}

	.content-wrapper {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--spacing-lg);
		width: 100%;
		max-width: 600px;
	}

	.page-title {
		margin: 0 0 var(--spacing-lg) 0;
		font-size: var(--font-size-xl);
		font-weight: var(--font-weight-700);
		line-height: 1.2;
		color: var(--color-text);
		text-align: center;
	}
</style>
