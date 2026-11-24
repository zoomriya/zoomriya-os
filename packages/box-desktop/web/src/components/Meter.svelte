<script lang="ts">
	// A bar: how full, nine tenths and more in the danger colour.
	let { fraction = 0, kind = '', width = '' }: { fraction?: number; kind?: string; width?: string } = $props();
	const shown = $derived(Math.max(0, Math.min(1, fraction || 0)));
</script>

<div class="meter {kind}" class:full={shown > 0.9 && kind !== 'mic'} style:width={width || null} role="meter"
	aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(shown * 100)}>
	<div style:width="{shown * 100}%"></div>
</div>
