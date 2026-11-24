<script lang="ts">
	// Resource Monitor › Temperature (tools.py's temperatures): each sensor and fan, every five seconds.
	import { onDestroy } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { _ } from '../lib/i18n.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let answer = $state<Answer | null>(null);
	const tick = () => ask('resources').then((a) => (answer = a)).catch(() => {});
	void tick();
	const timer = setInterval(tick, 5000);
	onDestroy(() => clearInterval(timer));
	const temps = $derived((answer?.temperatures ?? []) as Answer[]);
	const fans = $derived((answer?.fans ?? []) as Answer[]);
</script>

<div style:display="flex" style:flex-direction="column" style:gap="10px">
	<span class="page-title">{_('Temperature')}</span>
	{#if answer}
		<div class="facts">
			{#if !temps.length && !fans.length}<span class="lead" style:grid-column="1 / -1">{_('This box reports no sensors.')}</span>{/if}
			{#each temps as sensor, i (i)}
				<span class="fact-name">{sensor.label || sensor.sensor || ''}</span>
				<span class="fact" class:warn-text={(sensor.celsius ?? 0) >= 80} class:ok-text={(sensor.celsius ?? 0) < 80}>{Number(sensor.celsius ?? 0).toFixed(0)} °C</span>
			{/each}
			{#each fans as fan, i (i)}
				<span class="fact-name">{fan.label || fan.sensor || _('Fan')}</span>
				<span class="fact">{fan.rpm ?? 0} rpm</span>
			{/each}
		</div>
	{/if}
</div>
