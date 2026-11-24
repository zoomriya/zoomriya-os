<script lang="ts">
	// The network's graph (graphs.py): download as a line over a soft fill,
	// upload as a second line, the last minutes from left to right.
	let { history, height = 40 }: { history: [number, number][]; height?: number } = $props();
	/** The graph never scales below this (bytes a second), so a quiet network stays low. */
	const FLOOR = 50_000;
	const W = 300;
	const paths = $derived.by(() => {
		if (history.length < 2) return null;
		const top = Math.max(FLOOR, ...history.map(([d, u]) => Math.max(d, u))) * 1.15;
		const step = W / (history.length - 1);
		const y = (v: number) => height - 1.5 - (v / top) * (height - 4);
		const line = (i: 0 | 1) => history.map((p, n) => `${n ? 'L' : 'M'}${(n * step).toFixed(1)} ${y(p[i]).toFixed(1)}`).join('');
		return { down: line(0), up: line(1), area: `M0 ${height}` + history.map((p, n) => `L${(n * step).toFixed(1)} ${y(p[0]).toFixed(1)}`).join('') + `L${W} ${height}Z` };
	});
</script>

<svg viewBox="0 0 {W} {height}" preserveAspectRatio="none" style:width="100%" style:height="{height}px" aria-hidden="true">
	{#if paths}
		<path d={paths.area} fill="var(--rate-down)" fill-opacity="0.16" />
		<path d={paths.down} fill="none" stroke="var(--rate-down)" stroke-width="1.8" stroke-linejoin="round" vector-effect="non-scaling-stroke" />
		<path d={paths.up} fill="none" stroke="var(--rate-up)" stroke-width="1.8" stroke-linejoin="round" vector-effect="non-scaling-stroke" />
	{/if}
</svg>
