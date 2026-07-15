<script lang="ts">
	interface Option {
		value: string;
		label: string;
	}

	let {
		options,
		selectedValue,
		placeholder = 'Select...',
		width = 160,
		onChange,
	}: {
		options: Option[];
		selectedValue: string;
		placeholder?: string;
		width?: number;
		onChange: (value: string) => void;
	} = $props();

	let isOpen = $state(false);
	let listRef = $state<HTMLUListElement | null>(null);
	let buttonRef = $state<HTMLButtonElement | null>(null);

	let selectedLabel = $derived.by(() => {
		const found = options.find((o) => o.value === selectedValue);
		return found ? found.label : placeholder;
	});

	function toggle(): void {
		isOpen = !isOpen;
	}

	function select(value: string): void {
		onChange(value);
		isOpen = false;
	}

	function handleKeyDown(event: KeyboardEvent): void {
		if (!isOpen) {
			if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown') {
				event.preventDefault();
				isOpen = true;
			}
			return;
		}

		switch (event.key) {
			case 'Escape':
				isOpen = false;
				buttonRef?.focus();
				break;
			case 'ArrowDown':
				event.preventDefault();
				focusNext(1);
				break;
			case 'ArrowUp':
				event.preventDefault();
				focusNext(-1);
				break;
			case 'Enter':
			case ' ':
				event.preventDefault();
				if (document.activeElement?.hasAttribute('data-option-index')) {
					const idx = parseInt(document.activeElement.getAttribute('data-option-index')!, 10);
					select(options[idx].value);
				}
				break;
		}
	}

	function focusNext(direction: number): void {
		if (!listRef) return;
		const items = listRef.querySelectorAll<HTMLElement>('[data-option-index]');
		if (items.length === 0) return;

		let currentIdx = -1;
		items.forEach((item, idx) => {
			if (item === document.activeElement) currentIdx = idx;
		});

		const nextIdx = (currentIdx + direction + items.length) % items.length;
		items[nextIdx].focus();
	}

	function handleClickOutside(event: MouseEvent): void {
		if (
			isOpen &&
			buttonRef && !buttonRef.contains(event.target as Node) &&
			listRef && !listRef.contains(event.target as Node)
		) {
			isOpen = false;
		}
	}

	$effect(() => {
		if (isOpen) {
			document.addEventListener('mousedown', handleClickOutside);
			return () => document.removeEventListener('mousedown', handleClickOutside);
		}
	});
</script>

<div class={['dropdown', { 'dropdown--open': isOpen }]} tabindex="-1">
	<button
		type="button"
		class="dropdown-button"
		style:width={width + 'px'}
		aria-expanded={isOpen}
		aria-haspopup="listbox"
		aria-label={placeholder}
		onclick={toggle}
		onkeydown={handleKeyDown}
		bind:this={buttonRef}
	>
		<span class="dropdown-label">{selectedLabel}</span>
		<svg class="dropdown-chevron" viewBox="0 0 12 8" width="12" height="8" fill="none" aria-hidden="true">
			<path d="M1 1l5 5 5-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
		</svg>
	</button>

	{#if isOpen}
		<ul
			class="dropdown-list"
			role="listbox"
			aria-label={placeholder}
			bind:this={listRef}
		>
			{#each options as option, i (option.value)}
				<li
					role="option"
					aria-selected={option.value === selectedValue}
					data-option-index={i}
					class={['dropdown-item', { 'dropdown-item--selected': option.value === selectedValue }]}
					onclick={() => select(option.value)}
					onkeydown={(e) => {
						if (e.key === 'Enter' || e.key === ' ') {
							e.preventDefault();
							select(option.value);
						}
					}}
					tabindex={-1}
				>
					{option.label}
				</li>
			{/each}
		</ul>
	{/if}
</div>

<style>
	.dropdown {
		position: relative;
		display: inline-block;
	}

	.dropdown-button {
		display: flex;
		align-items: center;
		gap: var(--spacing-sm);
		padding: 11px 32px 11px 16px;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		background-color: var(--color-bg-white);
		font-size: var(--font-size-base);
		color: var(--color-text);
		cursor: pointer;
		min-width: 160px;
		text-align: left;
		transition: background-color 0.15s ease, border-color 0.15s ease;
	}

	.dropdown-button:hover {
		background-color: var(--color-bg-hover);
	}

	.dropdown-button:focus-visible {
		box-shadow: 0 0 0 2px rgba(30, 102, 245, 0.15);
		outline: none;
	}

	.dropdown-label {
		flex: 1;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.dropdown-chevron {
		position: absolute;
		right: 10px;
		top: 50%;
		transform: translateY(-50%);
		color: var(--color-text-muted);
		pointer-events: none;
		transition: transform 0.15s ease;
	}

	.dropdown--open .dropdown-chevron {
		transform: translateY(-50%) rotate(180deg);
	}

	.dropdown-list {
		position: absolute;
		top: calc(100% + var(--spacing-xs));
		left: 0;
		max-height: 200px;
		overflow-y: auto;
		min-width: 100%;
		background-color: var(--color-bg-white);
		border: 1px solid var(--color-border-light);
		border-radius: var(--radius-sm);
		box-shadow: var(--shadow-md);
		list-style: none;
		margin: 0;
		padding: var(--spacing-xs) 0;
		z-index: 1000;
	}

	.dropdown-item {
		padding: var(--spacing-sm) var(--spacing-sm);
		font-size: 0.9rem;
		color: var(--color-text);
		cursor: pointer;
		white-space: nowrap;
		transition: background-color 0.1s ease;
		text-align: left;
	}

	.dropdown-item:hover,
	.dropdown-item:focus {
		background-color: var(--color-bg-hover);
		outline: none;
	}

	.dropdown-item--selected {
		font-weight: var(--font-weight-600);
	}
</style>
