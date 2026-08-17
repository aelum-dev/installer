<script lang="ts">
	/**
	 * Scrollable content container with consistent styling.
	 * Used for language lists, disk lists, and other scrollable content.
	 *
	 * Slots:
	 *   default - Content to scroll
	 */

	let scrollContainer: HTMLDivElement | null = null;
	let thumbHeight = $state(0);
	let thumbTop = $state(0);
	let showScrollbar = $state(true);
	let isDragging = $state(false);
	let dragStartY = $state(0);
	let dragStartScroll = $state(0);
	let dragStartThumbTop = $state(0);
	let fadeTimer: ReturnType<typeof setTimeout> | null = null;
	const FADE_DELAY = 2000;
	const HOVER_THRESHOLD = 20; // pixels from right edge to trigger scrollbar

	function calculateScrollbar(): void {
		if (!scrollContainer) return;
		
		const { scrollTop, scrollHeight, clientHeight } = scrollContainer;
		
		if (scrollHeight <= clientHeight) {
			showScrollbar = false;
			return;
		}
		
		showScrollbar = true;
		
		// Calculate thumb size (min 20px, max track height)
		const trackHeight = clientHeight;
		const thumbSize = Math.max(20, (clientHeight / scrollHeight) * trackHeight);
		thumbHeight = thumbSize;
		
		// Calculate thumb position
		const maxScroll = scrollHeight - clientHeight;
		const scrollRatio = scrollTop / maxScroll;
		const maxThumbTop = trackHeight - thumbSize;
		thumbTop = scrollRatio * maxThumbTop;
	}

	function handleScroll(): void {
		if (!isDragging) {
			calculateScrollbar();
		}
		resetFadeTimer();
	}

	// Initialize on mount
	$effect(() => {
		calculateScrollbar();
		resetFadeTimer();
	});

	function resetFadeTimer(): void {
		if (fadeTimer) {
			clearTimeout(fadeTimer);
		}
		showScrollbar = true;
		fadeTimer = setTimeout(() => {
			showScrollbar = false;
		}, FADE_DELAY);
	}

	function handleMouseEnter(): void {
		showScrollbar = true;
		resetFadeTimer();
	}

	function handleMouseLeave(): void {
		if (!isDragging) {
			resetFadeTimer();
		}
	}

	function handleMouseMove(e: MouseEvent): void {
		// Show scrollbar when mouse is near the right edge
		const { clientWidth } = scrollContainer!.parentElement!;
		const distanceFromRight = clientWidth - e.clientX;
		
		if (distanceFromRight <= HOVER_THRESHOLD) {
			showScrollbar = true;
			resetFadeTimer();
		}
	}

	function handleThumbMouseDown(e: MouseEvent): void {
		if (!scrollContainer) return;
		
		isDragging = true;
		dragStartY = e.clientY;
		dragStartScroll = scrollContainer.scrollTop;
		dragStartThumbTop = thumbTop;
		
		document.addEventListener('mousemove', handleThumbMouseMove);
		document.addEventListener('mouseup', handleThumbMouseUp);
		e.preventDefault();
	}

	function handleThumbMouseMove(e: MouseEvent): void {
		if (!isDragging || !scrollContainer) return;
		
		const deltaY = e.clientY - dragStartY;
		const { scrollHeight, clientHeight } = scrollContainer;
		const maxScroll = scrollHeight - clientHeight;
		const maxThumbTop = clientHeight - thumbHeight;
		
		// Calculate raw thumb position
		let newThumbTop = dragStartThumbTop + deltaY;
		
		// Constrain thumb within track boundaries
		newThumbTop = Math.max(0, Math.min(newThumbTop, maxThumbTop));
		
		// Update thumb position
		thumbTop = newThumbTop;
		
		// Update scroll position based on thumb position
		const scrollRatio = newThumbTop / maxThumbTop;
		scrollContainer.scrollTop = scrollRatio * maxScroll;
	}

	function handleThumbMouseUp(): void {
		isDragging = false;
		document.removeEventListener('mousemove', handleThumbMouseMove);
		document.removeEventListener('mouseup', handleThumbMouseUp);
		calculateScrollbar();
	}
</script>

<div class="scroll-outer" onmousemove={handleMouseMove}>
	<div
		class="scroll-inner scroll-box"
		bind:this={scrollContainer}
		onscroll={handleScroll}
	>
		<slot />
	</div>
	<div class="scrollbar-track" class:visible={showScrollbar} onmouseenter={handleMouseEnter} onmouseleave={handleMouseLeave}>
			<div 
				class="scrollbar-thumb"
				class:dragging={isDragging}
				style="height: {thumbHeight}px; top: {thumbTop}px;"
				onmousedown={handleThumbMouseDown}
			></div>
		</div>
</div>

<style>
	.scroll-outer {
		flex: 1;
		min-height: 0;
		height: 100%;
		display: flex;
		position: relative;
	}

	.scroll-box {
		flex: 1;
		min-height: 0;
		height: 100%;
		overflow-y: auto;
		overflow-x: hidden;
		display: flex;
		flex-direction: column;
		gap: var(--spacing-md);
		padding-inline-end: 8px;
		/* Hide native scrollbar on Linux */
		scrollbar-width: none;
		-ms-overflow-style: none;
	}

	.scroll-box::-webkit-scrollbar {
		display: none;
	}

	.scrollbar-track {
		width: 12px;
		background: transparent;
		display: flex;
		align-items: flex-start;
		padding: 2px;
		position: relative;
		opacity: 0;
		transition: opacity 0.6s ease;
		pointer-events: none;
	}

	.scrollbar-track.visible {
		opacity: 1;
		pointer-events: auto;
	}

	.scrollbar-thumb {
		width: 8px;
		background: #a67480;
		border-radius: 4px;
		position: absolute;
		top: 0;
		right: 0;
		cursor: pointer;
		transition: width 0.2s ease, background-color 0.2s ease;
	}

	.scrollbar-thumb:hover,
	.scrollbar-thumb.dragging {
		width: 12px;
		right: 0;
		background: #8e5a64;
	}
</style>
