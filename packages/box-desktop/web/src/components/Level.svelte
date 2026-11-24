<script lang="ts">
	// A slider as every one on Box Desktop: a thin track, a small knob, the same width everywhere.
	let {
		value = $bindable(0), min = 0, max = 1, steps = 100, label = '', changed, disabled = false
	}: { value?: number; min?: number; max?: number; steps?: number; label?: string; changed?: (v: number) => void; disabled?: boolean } = $props();
	const fill = $derived(`${((value - min) / (max - min)) * 100}%`);
</script>

<input class="level" type="range" {min} {max} step={(max - min) / steps} bind:value aria-label={label} {disabled}
	style:--fill={fill} oninput={() => changed?.(Number(value))} />
