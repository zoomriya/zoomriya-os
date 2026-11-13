<script lang="ts">
	// Storage Manager › Drives (tools.py's storage_drives): each drive's health, size, place, heat and age.
	import { ask, type Answer } from '../lib/api';
	import { size } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { DRIVE_HEALTH } from '../lib/words';

	let { shown = true }: { shown?: boolean } = $props();
	let drives = $state<Answer[]>([]);
	let problem = $state('');
	ask('disk-health').then((h) => (drives = h.drives ?? [])).catch((e) => (problem = e.message));

	function rows(drive: Answer): [string, string][] {
		const listed: [string, string][] = [[_('Size'), size(drive.size_bytes ?? 0)], [_('Place'), drive.system ? _("The box's own") : drive.name ?? '']];
		if (drive.temperature_c != null) listed.push([_('Temperature'), `${drive.temperature_c} °C`]);
		if (drive.power_on_hours != null) listed.push([_('In use for'), fill(_('{n} days'), { n: Math.floor(drive.power_on_hours / 24) })]);
		return listed;
	}
</script>

<div style:display="flex" style:flex-direction="column" style:gap="12px">
	<span class="page-title">{_('Drives')}</span>
	{#if problem}<p class="error">{problem}</p>{/if}
	<div class="drive-cards">
		{#each drives as drive, i (i)}
			{@const state = drive.health ?? 'unknown'}
			<div class="drive-card">
				<div class="row-between">
					<span class="fact grow">{drive.model || drive.name || ''}</span>
					<span class="badge {state in DRIVE_HEALTH ? state : 'unknown'}">{_(DRIVE_HEALTH[state] ?? 'Unknown')}</span>
				</div>
				<div style:display="grid" style:grid-template-columns="auto 1fr" style:gap="4px 16px">
					{#each rows(drive) as [name, value] (name)}<span class="fact-name">{name}</span><span class="small-fact">{value}</span>{/each}
				</div>
				{#each drive.reasons ?? [] as reason, j (j)}<p class="lead">{reason}</p>{/each}
			</div>
		{/each}
	</div>
</div>
