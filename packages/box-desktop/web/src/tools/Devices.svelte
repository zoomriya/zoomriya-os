<script lang="ts">
	// Control Panel › Devices: the phones and laptops paired with the box,
	// pairing new ones at home, and removing them. Everything goes through
	// box-desktopd to connectd (connect.go); no ZOOMRIYA server is involved
	// (zoomriya-connect, docs/direct-plus.md §6).
	import { onDestroy } from 'svelte';
	import { desk } from '../lib/desk.svelte';
	import { day } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import PairLaptop from './PairLaptop.svelte';
	import PairPhone from './PairPhone.svelte';
	import { deviceWords, devices as listDevices, iso, removeDevice, type Device } from './devices';

	let { shown = true }: { shown?: boolean } = $props();
	let list = $state<Device[] | null>(null);
	let problem = $state('');

	async function load(): Promise<void> {
		try {
			list = await listDevices();
			problem = '';
		} catch (e) {
			problem = deviceWords(e);
		}
	}

	function remove(d: Device): void {
		desk.confirm({
			title: fill(_('Remove {name}?'), { name: d.name }),
			text: _('It stops reaching the box at once. Pair it again to bring it back.'),
			button: _('Remove'),
			action: async (password) => {
				try {
					return await removeDevice(d.id, password);
				} catch (e) {
					throw new Error(deviceWords(e));
				}
			},
			done: () => void load()
		});
	}

	// Who is connected changes as people come and go.
	const tick = setInterval(() => shown && void load(), 15000);
	onDestroy(() => clearInterval(tick));
	void load();

	function seen(d: Device): string {
		if (d.connected) return _('Connected now');
		return d.last_seen ? fill(_('Last connected {date}'), { date: day(iso(d.last_seen), true) }) : _('Not connected yet');
	}
</script>

<div class="settings">
	{#if list}
		<Group title={_('Phones and laptops')} icon="connect"
			note={_('Each one is paired at home, once. Removing one stops it at once.')}>
			{#each list as d (d.id)}
				<Row icon={d.laptop ? 'monitor' : 'phone'} title={d.name} sub={seen(d)}>
					{#snippet ends()}<button onclick={() => remove(d)}>{_('Remove')}</button>{/snippet}
				</Row>
			{:else}
				<Row icon="phone" title={_('None yet')} sub={_('Pair a phone or add a laptop to use the ZOOMRIYA app with this box.')} />
			{/each}
		</Group>
		<Group title={_('Add a device')} icon="scan">
			<Row icon="scan" title={_('Pair a phone')} sub={_('Shows a code to scan with the ZOOMRIYA app.')}>
				{#snippet ends()}<button class="primary" onclick={() => desk.sheet(PairPhone, { done: () => void load() })}>{_('Pair a phone')}</button>{/snippet}
			</Row>
			<Row icon="monitor" title={_('Add a laptop')} sub={_('Shows a code to type into the ZOOMRIYA app on the laptop.')}>
				{#snippet ends()}<button onclick={() => desk.sheet(PairLaptop, { done: () => void load() })}>{_('Add a laptop')}</button>{/snippet}
			</Row>
		</Group>
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>
