<script lang="ts">
	import type { PartitionType } from '$lib/types';
	import { TYPE_ESP, TYPE_SYSTEM } from '$lib/types';
	import { t } from '$lib/i18n';
	import { glyph } from '$lib/icons';
	import './RoleSelect.css';

	// A fully custom role selector so the dropdown menu is themed to the installer's
	// light UI. The native <select> popup is drawn by the OS/browser and can't be
	// recoloured, so it would render a dark list over a light theme.
	//
	// Options follow the ARIA listbox pattern: a trigger button with aria-haspopup,
	// a menu with role="listbox", and option buttons with role="option".
	//
	// options keep their translation key so labels stay in sync with the rest of the
	// installer; role is used only for colour-coding the menu rows.
	const OPTIONS = [
		{ value: '' as PartitionType, labelKey: 'manual.type.preserve', role: '' as const },
		{ value: TYPE_ESP, labelKey: 'manual.type.esp', role: 'esp' as const },
		{ value: TYPE_SYSTEM, labelKey: 'manual.type.system', role: 'system' as const },
	] as const;

	let { value, onChange }: { value: PartitionType; onChange: (v: PartitionType) => void } = $props();

	// Unique id per instance so aria-controls points each trigger at its own menu.
	// ($props.id() is Svelte's per-instance id; a module- or instance-local counter
	// would hand every row the same id.)
	const menuId = $props.id();

	let open = $state(false);
	let wrap: HTMLElement | null = $state(null);
	let triggerEl: HTMLButtonElement | null = $state(null);
	let menuEl: HTMLElement | null = $state(null);
	// Index of the keyboard-active option (roving via aria-activedescendant).
	let activeIndex = $state(0);

	function optionId(i: number): string {
		return `${menuId}-opt-${i}`;
	}

	function openMenu(): void {
		// Start on the current selection so arrow keys move from where you are.
		const selected = OPTIONS.findIndex((o) => o.value === value);
		activeIndex = selected >= 0 ? selected : 0;
		open = true;
		requestAnimationFrame(() => menuEl?.focus());
	}

	function closeMenu(refocus = false): void {
		open = false;
		if (refocus) triggerEl?.focus();
	}

	function toggle(): void {
		if (open) {
			closeMenu(true);
		} else {
			openMenu();
		}
	}

	function pick(v: PartitionType): void {
		// setType treats re-selecting the active role as a clear-back-to-Preserve, so
		// the toggle semantics stay identical to the old per-row pills.
		onChange(v);
		closeMenu(true);
	}

	function moveActive(delta: number): void {
		const next = activeIndex + delta;
		if (next < 0 || next >= OPTIONS.length) return;
		activeIndex = next;
	}

	// Close when the click lands outside the control.
	$effect(() => {
		if (!open) return;
		const onDoc = (e: MouseEvent): void => {
			if (wrap && !wrap.contains(e.target as Node)) open = false;
		};
		document.addEventListener('click', onDoc);
		return () => document.removeEventListener('click', onDoc);
	});

	// The trigger opens on ArrowDown/ArrowUp (Enter/Space already open it via the
	// button's click). Only one of the trigger/menu is ever a tab stop.
	function onTriggerKey(e: KeyboardEvent): void {
		if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && !open) {
			e.preventDefault();
			openMenu();
		}
	}

	function onMenuKey(e: KeyboardEvent): void {
		switch (e.key) {
			case 'ArrowDown':
				e.preventDefault();
				moveActive(1);
				break;
			case 'ArrowUp':
				e.preventDefault();
				moveActive(-1);
				break;
			case 'Home':
				e.preventDefault();
				activeIndex = 0;
				break;
			case 'End':
				e.preventDefault();
				activeIndex = OPTIONS.length - 1;
				break;
			case 'Enter':
			case ' ':
				e.preventDefault();
				pick(OPTIONS[activeIndex].value);
				break;
			case 'Escape':
			case 'Tab':
				// Close and hand focus back to the trigger so Tab resumes from a
				// stable element (the menu unmounts, so letting Tab continue from a
				// focused option would drop focus to the document).
				e.preventDefault();
				closeMenu(true);
				break;
		}
	}
</script>

<div class="role-select" bind:this={wrap}>
	<button
		type="button"
		class={['role-trigger', { esp: value === TYPE_ESP, system: value === TYPE_SYSTEM }]}
		bind:this={triggerEl}
		aria-label={t('manual.type')}
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-controls={menuId}
		onclick={toggle}
		onkeydown={onTriggerKey}
	>
		<span class="role-trigger-label">{t(value === '' ? 'manual.type.preserve' : 'manual.type.' + value)}</span>
		<span class="role-chevron" aria-hidden="true">{glyph(open ? 'eab7' : 'eab4')}</span>
	</button>

	{#if open}
		<div
			class="role-menu"
			id={menuId}
			role="listbox"
			tabindex="-1"
			bind:this={menuEl}
			aria-activedescendant={optionId(activeIndex)}
			onkeydown={onMenuKey}
		>
			{#each OPTIONS as opt, i (opt.value)}
				<button
					type="button"
					id={optionId(i)}
					role="option"
					tabindex="-1"
					aria-selected={value === opt.value}
					class={['role-option', { esp: opt.role === 'esp', system: opt.role === 'system', active: i === activeIndex }]}
					onclick={() => pick(opt.value)}
					onmouseenter={() => (activeIndex = i)}
				>
					<span class="role-option-dot" aria-hidden="true"></span>
					{t(opt.labelKey)}
				</button>
			{/each}
		</div>
	{/if}
</div>
