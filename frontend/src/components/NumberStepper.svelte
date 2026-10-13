<script lang="ts">
	import { glyph } from '$lib/icons';
	import './NumberStepper.css';

	let {
		value = $bindable(1),
		min = 1,
		max,
		step = 1,
		disabled = false,
		placeholder = ''
	}: {
		value?: number;
		min?: number;
		max?: number;
		step?: number;
		disabled?: boolean;
		placeholder?: string;
	} = $props();

	// Step the value by `delta`, clamping to [min, max] when max is finite. A
	// cleared field binds as NaN (empty number input), so start from the minimum
	// in that case rather than letting NaN poison the result.
	function stepBy(delta: number): void {
		if (disabled) return;
		const base = Number.isFinite(value) ? value : (min ?? 1);
		let next = base + delta;
		if (max != null) next = Math.min(next, max);
		value = Math.max(next, min ?? 1);
	}
</script>

<div class={['number-stepper', { disabled }]}>
	<input
		type="number"
		class="number-stepper-input"
		bind:value
		min={min}
		max={max}
		step={step}
		{disabled}
		placeholder={placeholder}
	/>
	<button
		type="button"
		class="stepper-btn stepper-up"
		disabled={disabled}
		onclick={() => stepBy(step)}
		aria-label="Increase"
	>
		{glyph('eab7')}
	</button>
	<button
		type="button"
		class="stepper-btn stepper-down"
		disabled={disabled}
		onclick={() => stepBy(-step)}
		aria-label="Decrease"
	>
		{glyph('eab4')}
	</button>
</div>
