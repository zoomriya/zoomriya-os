<script lang="ts">
	// Resource Monitor › By app (tools.py's resources_by_app): busiest first, every five seconds.
	import { onDestroy } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { size } from '../lib/format';
	import { _ } from '../lib/i18n.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let services = $state<Answer[]>([]);
	const tick = () => ask('resources').then((a) => (services = a.services ?? [])).catch(() => {});
	void tick();
	const timer = setInterval(tick, 5000);
	onDestroy(() => clearInterval(timer));
</script>

<div style:display="flex" style:flex-direction="column" style:gap="10px">
	<span class="page-title">{_('By app')}</span>
	<p class="lead">{_('Busiest first. CPU is a share of the whole box.')}</p>
	<div class="facts three">
		<span></span><span class="field-label">{_('CPU')}</span><span class="field-label">{_('Memory')}</span>
		{#each services as use, i (i)}
			<span class="fact" title={use.kind === 'app' ? _('An app') : _('Part of the box')}>{use.name || use.unit || ''}</span>
			<span class="small-fact">{Number(use.cpu_percent ?? 0).toFixed(1)}%</span>
			<span class="small-fact">{size(use.memory_bytes ?? 0)}</span>
		{/each}
	</div>
</div>
