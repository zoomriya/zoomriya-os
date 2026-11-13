<script lang="ts">
	// Storage Manager › Space (tools.py's storage_space): what takes the space on the box's own disk.
	import { ask, type Answer } from '../lib/api';
	import { size } from '../lib/format';
	import { _ } from '../lib/i18n.svelte';
	import Meter from '../components/Meter.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let storage = $state<Answer | null>(null);
	let problem = $state('');
	ask('storage').then((s) => (storage = s)).catch((e) => (problem = e.message));
	const names = $derived<Record<string, string>>({
		databases: _("The apps' databases"), 'app-images': _('The apps themselves'), 'models-and-maps': _("Myira's models and the maps"),
		'app-data': _("Photos, files and the apps' data"), 'restore-points': _('Restore points'), os: 'ZOOMRIYA OS'
	});
	const total = $derived(storage?.system?.used_bytes ?? 0);
	const parts = $derived([...(storage?.system_parts ?? [])].sort((a: Answer, b: Answer) => (b.bytes ?? 0) - (a.bytes ?? 0)));
</script>

<div style:display="flex" style:flex-direction="column" style:gap="12px">
	<span class="page-title">{_('Space')}</span>
	<p class="lead">{_("What takes the space on the box's own disk.")}</p>
	{#if problem}<p class="error">{problem}</p>{/if}
	{#if storage}
		<div class="facts three">
			{#if !parts.length}<span class="lead" style:grid-column="1 / -1">{_('Not counted yet; updated every few hours.')}</span>{/if}
			{#each parts as part (part.kind)}
				<span class="fact-name">{names[part.kind] ?? part.kind}</span>
				<Meter fraction={total ? (part.bytes ?? 0) / total : 0} />
				<span class="small-fact">{size(part.bytes ?? 0)}</span>
			{/each}
		</div>
	{/if}
</div>
