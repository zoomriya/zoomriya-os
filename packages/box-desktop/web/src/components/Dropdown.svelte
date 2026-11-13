<script lang="ts">
	// A dropdown, its narrowest 170 px; a longer choice makes it wider.
	let {
		names, selected = $bindable(-1), changed, label = '', disabled = false
	}: { names: string[]; selected?: number; changed?: (i: number) => void; label?: string; disabled?: boolean } = $props();
</script>

<select bind:value={selected} aria-label={label} {disabled} onchange={() => changed?.(Number(selected))}>
	{#if selected < 0}<option value={-1} disabled hidden></option>{/if}
	{#each names as name, i (i)}<option value={i}>{name}</option>{/each}
</select>
