<script lang="ts">
	// A slider with its two ends named under it (Slow … Fast).
	import Level from './Level.svelte';

	let {
		low, high, value = $bindable(0), min = 0, max = 1, changed
	}: { low: string; high: string; value?: number; min?: number; max?: number; changed?: (v: number) => void } = $props();
</script>

<div>
	<Level bind:value {min} {max} {changed} label="{low} – {high}" />
	<div class="level-ends"><span class="row-sub">{low}</span><span class="row-sub">{high}</span></div>
</div>
