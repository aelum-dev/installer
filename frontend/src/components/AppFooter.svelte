<script lang="ts">
	let { currentPage }: { currentPage: number } = $props();

	const footerTexts = [
		'Press Next to continue',
		'Fill in your details and press Next',
		'Select a disk and press Next',
		'Installation is running...',
		'Installation complete. Press Finish to exit.',
	];
</script>

<footer class="footer">
	<span>Arch Installer v0.1.0</span>
	<span>{footerTexts[currentPage]}</span>
</footer>

<style>
	.footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 24px;
		background-color: #e8e8e8;
		border-top: 1px solid #ccc;
		min-height: 44px;
		font-size: 0.85rem;
		color: #555;
	}
</style>
