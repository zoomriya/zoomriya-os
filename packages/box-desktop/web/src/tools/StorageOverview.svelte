<script lang="ts">
	// Storage Manager › Overview (tools.py's storage_overview): the library and
	// its check, and the box's own disk, with what takes its space.
	import { ask, type Answer } from '../lib/api';
	import { size } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { DRIVE_HEALTH } from '../lib/words';
	import Meter from '../components/Meter.svelte';
	import LibraryCheck from './LibraryCheck.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let storage = $state<Answer | null>(null);
	let drives = $state<Answer[]>([]);
	let problem = $state('');

	Promise.all([ask('storage'), ask('disk-health')])
		.then(([s, h]) => { storage = s; drives = h.drives ?? []; })
		.catch((e) => (problem = e.message));

	const library = $derived(storage?.library ?? {});
	const libraryState = $derived.by(() => {
		const states = drives.filter((d) => !d.system).map((d) => d.health);
		return states.length && states.every((s) => s === 'healthy') ? 'healthy' : states.length ? 'warning' : 'unknown';
	});
	const systemState = $derived(drives.find((d) => d.system)?.health ?? 'unknown');
	const cap = (t: string) => t.charAt(0).toUpperCase() + t.slice(1);
</script>

{#snippet space(title: string, sub: string, part: Answer, state: string, kind: string)}
	{@const total = part.size_bytes ?? 0}
	{@const used = part.used_bytes ?? 0}
	<div style:display="flex" style:flex-direction="column" style:gap="8px">
		<div class="row-between">
			<div class="grow" style:display="flex" style:flex-direction="column">
				<span class="page-title">{title}</span>
				<span class="lead">{sub}</span>
			</div>
			<span class="badge {state in DRIVE_HEALTH ? state : 'unknown'}">{_(DRIVE_HEALTH[state] ?? 'Unknown')}</span>
		</div>
		<div class="row-between">
			<span class="fact grow">{fill(_('{size} used'), { size: size(used) })}</span>
			<span class="fact-name">{fill(_('{free} free of {total}'), { free: size(part.free_bytes ?? 0), total: size(total) })}</span>
		</div>
		<Meter fraction={total ? used / total : 0} {kind} />
	</div>
{/snippet}

<div style:display="flex" style:flex-direction="column" style:gap="16px">
	{#if problem}<p class="error">{problem}</p>{/if}
	{#if storage}
		{#if library.path}
			{@render space(_('Library'), library.mirror ? fill(_('{n} drives, each a copy of the other'), { n: (library.drives ?? []).length })
				: (library.drives ?? []).map((d: Answer) => d.name ?? '').join(', '), library, libraryState, 'library')}
			<LibraryCheck check={library.check ?? {}} mirror={!!library.mirror} />
		{/if}
		{@render space(_("The box's own disk"), _('The OS, the apps and their data'), storage.system ?? {}, systemState, 'system')}
		{#if (storage.system_parts ?? []).length}
			<div style:display="flex" style:flex-wrap="wrap" style:gap="6px 18px">
				{#each storage.system_parts as part (part.kind)}
					<span class="fact-name">{cap(String(part.kind ?? '').replaceAll('_', ' '))} · {size(part.bytes ?? 0)}</span>
				{/each}
			</div>
		{/if}
	{/if}
</div>
