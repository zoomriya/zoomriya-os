<script lang="ts">
	// The CPU's last minute and a half, in its card's indigo.
	let { points, height = 44 }: { points: number[]; height?: number } = $props();
	const W = 200;
	const d = $derived.by(() => {
		const shown = points.slice(-30);
		if (shown.length < 2) return '';
		const top = Math.max(...shown, 10);
		const step = W / (shown.length - 1);
		return shown.map((v, i) => `${i ? 'L' : 'M'}${(i * step).toFixed(1)} ${(height - 2 - (v / top) * (height - 6)).toFixed(1)}`).join('');
	});
</script>

<svg viewBox="0 0 {W} {height}" preserveAspectRatio="none" style:width="100%" style:height="{height}px" aria-hidden="true">
	<path {d} fill="none" stroke="var(--sparkline)" stroke-width="1.6" vector-effect="non-scaling-stroke" />
</svg>
