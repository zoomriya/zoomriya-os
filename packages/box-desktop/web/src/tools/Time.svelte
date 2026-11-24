<script lang="ts">
	// Control Panel › Date and time (control.py's cp_time): the box's clock,
	// its time zone, and whether it keeps itself right from the internet.
	import { onDestroy } from 'svelte';
	import { ask, get } from '../lib/api';
	import { day, setZone } from '../lib/format';
	import { _ } from '../lib/i18n.svelte';
	import Dropdown from '../components/Dropdown.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let zones = $state<string[]>([]);
	let picked = $state(-1);
	let synced = $state<boolean | null>(null);
	let done = $state('');
	let now = $state(new Date().toISOString());
	const timer = setInterval(() => (now = new Date().toISOString()), 30000);
	onDestroy(() => clearInterval(timer));
	const utc = (zone: string) => (['Etc/UTC', 'Etc/Universal', 'UCT'].includes(zone) ? 'UTC' : zone);

	async function load(): Promise<void> {
		const [listed, status] = await Promise.all([get<{ time_zones: string[] }>('/api/places'), ask('setup-status')]);
		zones = listed.time_zones;
		picked = zones.indexOf(utc(status.zone ?? 'UTC'));
		synced = !!status.clock_synced;
	}

	async function choose(i: number): Promise<void> {
		try {
			await ask('time-zone-set', { zone: zones[i] });
			setZone(zones[i]);
			now = new Date().toISOString();
			done = _('Saved.');
		} catch (e) {
			done = (e as Error).message;
		}
	}

	void load().catch((e) => (done = (e as Error).message));
</script>

<div class="settings">
	<Group title={_('Date & Time')} icon="clock">
		<Row big icon="clock" title={_('Current Time')} sub={_("On the box's clock.")}>
			{#snippet ends()}<span class="row-value" style:white-space="nowrap">{day(now, true)}</span>{/snippet}
		</Row>
		<Row big icon="globe" title={_('Time Zone')} sub={_("The box's dates and times follow it.")}>
			{#snippet ends()}<Dropdown names={zones.map((z) => z.replaceAll('_', ' '))} bind:selected={picked} changed={choose} label={_('Time Zone')} />{/snippet}
		</Row>
		<Row big icon="refresh" title={_('Time Sync')}
			sub={synced === null ? '' : synced ? _('Set by itself from the internet.') : _('Syncs once the box is online.')}>
			{#snippet ends()}<span class={synced ? 'allowed-dot' : 'off-dot'} title={synced ? _('In sync') : _('Not yet in sync')}></span>{/snippet}
		</Row>
	</Group>
	{#if done}<span class="group-note">{done}</span>{/if}
</div>
