<script lang="ts">
	import PageLayout from '$components/PageLayout.svelte';
</script>

<PageLayout title="Installation Complete">
	<div class="finish">
		<div class="icon">✓</div>
		<p class="message">AestheticOS has been installed successfully.</p>
	</div>
</PageLayout>

<style>
	.finish {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
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
