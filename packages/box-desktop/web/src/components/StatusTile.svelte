<script lang="ts">
	// A summary at the top of a page: an icon in a circle, ok or not.
	import Icon from './Icon.svelte';

	let { icon, title, words, state }: { icon: string; title: string; words: string; state: string } = $props();
</script>

<div class="tile-card">
	<div class="ring {state}"><Icon name={icon} px={26} role={state === 'ok' || state === 'warn' ? state : 'muted'} /></div>
	<span class="row-title">{title}</span>
	<span class="row-sub">{words}</span>
</div>
