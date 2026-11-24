<script lang="ts">
	// The ZOOMRIYA logo as the apps draw it: the letters (or the infinity's
	// loops) in the text's colour, the swoosh in its own blue-to-cyan.
	let { mark = false, height }: { mark?: boolean; height: number } = $props();
	// The images' own sizes: the wordmark 1262 × 162, the mark 736 × 383.
	const width = $derived(Math.round(height * (mark ? 736 / 383 : 1262 / 162)));
	const shape = $derived(mark ? '/logo/mark-loops.png' : '/logo/wordmark-letters.png');
	const swoosh = $derived(mark ? '/logo/mark-swoosh.png' : '/logo/wordmark-swoosh.png');
</script>

<span class="logo" style:width="{width}px" style:height="{height}px" role="img" aria-label="ZOOMRIYA">
	<span class="letters" style:mask-image="url({shape})" style:-webkit-mask-image="url({shape})"></span>
	<img src={swoosh} alt="" />
</span>
