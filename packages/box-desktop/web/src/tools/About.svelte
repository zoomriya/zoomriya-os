<script lang="ts">
	// Control Panel › System Info (control.py's cp_about): the box, Family Hub's
	// address to open on a phone, and what the box is made of and runs.
	import { ask, get, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { duration, size } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { updatesUntil } from '../lib/support';
	import Group from '../components/Group.svelte';
	import Logo from '../components/Logo.svelte';
	import Row from '../components/Row.svelte';
	import { openElsewhere } from '../pages/elsewhere';

	let { shown = true }: { shown?: boolean } = $props();
	let listed = $state<[string, string, string][]>([]);
	let details = $state<[string, string, string][]>([]);
	let problem = $state('');
	const hub = $derived(desk.boxName ? `https://${desk.boxName}/` : '');

	async function load(): Promise<void> {
		const [status, resources, storage, info] = await Promise.all([ask('status'), ask('resources'), ask('storage'), get<Answer>('/api/info')]);
		const temps = (resources.temperatures ?? []).map((t: Answer) => t.celsius).filter((c: unknown) => c != null) as number[];
		const used = resources.memory_used_bytes ?? 0, total = resources.memory_total_bytes ?? 0;
		const disks = [fill(_('{size} for the box'), { size: size(storage.system?.size_bytes ?? 0) })];
		if (storage.library?.size_bytes) disks.push(fill(_('{size} library'), { size: size(storage.library.size_bytes) }));
		listed = [
			[_('Name'), desk.boxName || _('Unknown'), 'tag'],
			[_('Model'), status.model ?? '', 'box'],
			[_('Serial Number'), status.serial ?? '', 'barcode'],
			[_('Operating System'), `${status.os ?? ''} (${status.image_version ?? ''})`, 'layers'],
			[_('Security updates'), updatesUntil(status), 'updates'],
			[_('Processor'), info.processor ?? '', 'cpu'],
			[_('Memory'), total ? fill(_('{used} of {total} in use'), { used: size(used), total: size(total) }) : _('Unknown'), 'memory'],
			[_('Disk Capacity'), disks.join(' + '), 'disk'],
			[_('Temperature'), temps.length ? `${Math.max(...temps).toFixed(0)} °C` : _('No sensor'), 'thermometer'],
			[_('Running For'), duration(status.uptime_seconds ?? 0), 'maintenance']
		];
		details = [
			[_('System Release'), status.debian ?? '', 'layers'], [_('Base Package'), status.base ?? '', 'package'],
			[_('Factory Image'), status.image_version ?? '', 'factory'], [_('Kernel'), status.kernel ?? '', 'cpu']
		];
	}

	void load().catch((e) => (problem = e.message));
</script>

<div class="settings">
	<div style:display="flex" style:flex-direction="column" style:align-items="center" style:gap="8px">
		<Logo mark height={52} />
		<span class="about-name">{desk.boxName || 'ZOOMRIYA Box'}</span>
	</div>
	<Group>
		<Row big icon="phone" title={_('Family Hub')} sub={hub}>
			{#snippet ends()}<button class="primary" onclick={() => openElsewhere(_('Family Hub'), hub || `https://${location.hostname}/`)}>{_('Open on a phone')}</button>{/snippet}
		</Row>
	</Group>
	{#if listed.length}
		<Group>
			{#each listed.filter(([, said]) => said) as [caption, said, icon] (caption)}
				<Row big {icon} title={caption}>{#snippet ends()}<span class="row-value">{said}</span>{/snippet}</Row>
			{/each}
		</Group>
		<Group title={_('Details')} icon="info">
			{#each details as [caption, said, icon] (caption)}
				<Row big {icon} title={caption}>{#snippet ends()}<span class="row-value">{said}</span>{/snippet}</Row>
			{/each}
		</Group>
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>
