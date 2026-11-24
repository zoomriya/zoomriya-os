<script lang="ts">
	// Control Panel › Network (control.py's cp_network): each Ethernet port,
	// then how the chosen one gets its address, tried for a few minutes and
	// put back unless kept. Wi-Fi, on a box that has it, is shown turned off.
	import { ask, type Answer } from '../lib/api';
	import { when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { fixedSettings, maskOf } from '../lib/network';
	import Group from '../components/Group.svelte';
	import Icon from '../components/Icon.svelte';
	import Row from '../components/Row.svelte';
	import Segmented from '../components/Segmented.svelte';
	import Switch from '../components/Switch.svelte';
	import Wide from '../components/Wide.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let links = $state<Answer[]>([]);
	let wifi = $state(false);
	let port = $state('');
	let mode = $state('dhcp');
	let address = $state('');
	let mask = $state('');
	let router = $state('');
	let dns = $state('');
	let problem = $state('');
	const link = $derived(links.find((x) => x.name === port) ?? {});
	const undo = $derived(link.undo_at as string | undefined);

	function showPort(name: string): void {
		port = name;
		const chosen = links.find((x) => x.name === name) ?? {};
		mode = chosen.mode === 'fixed' ? 'fixed' : 'dhcp';
		const fixed = chosen.fixed ?? {};
		if (fixed.address) {
			const [a, bits] = String(fixed.address).split('/');
			address = a;
			if (bits) mask = maskOf(Number(bits));
			router = fixed.gateway ?? '';
			dns = (fixed.dns ?? []).join(', ');
		}
	}

	async function load(): Promise<void> {
		try {
			const answer = await ask('network');
			links = (answer.links ?? []).filter((x: Answer) => x.kind === 'ethernet');
			wifi = (answer.links ?? []).some((x: Answer) => x.kind === 'wifi');
			const chosen = links.some((x) => x.name === port) ? port
				: (links.find((x) => x.connected)?.name ?? links[0]?.name ?? '');
			showPort(chosen);
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	async function apply(): Promise<void> {
		problem = '';
		let settings: Record<string, unknown>;
		try {
			settings = mode === 'fixed' ? fixedSettings(address, mask, router, dns) : { mode: 'dhcp' };
		} catch (e) {
			problem = (e as Error).message;
			return;
		}
		try {
			await ask('network-set', { link: port, ...settings });
			setTimeout(load, 3000);
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	async function keep(): Promise<void> {
		try {
			await ask('network-keep');
		} finally {
			await load();
		}
	}

	void load();
</script>

<div class="settings">
	{#each links as l (l.name)}
		<Group title={fill(_('Ethernet Status ({port})'), { port: l.name ?? '' })} icon="ethernet">
			<Row big icon="ethernet"
				title={l.connected ? _('Connection: Connected') : _('Connection: Cable unplugged')}
				sub={l.connected
					? fill(_('IPv4 Address: {address} ({how})'), {
							address: String(l.ipv4?.[0] ?? '').split('/')[0] || _('none yet'),
							how: l.mode === 'fixed' ? _('Static') : _('Assigned by Router') })
					: fill(_('MAC address: {mac}'), { mac: l.mac ?? '' })}>
				{#snippet ends()}
					<div style:display="flex" style:flex-direction="column" style:align-items="flex-end" style:gap="6px">
						{#if l.connected && l.speed_mbps}
							<span class="with-icon"><span class="row-title">{Number(l.speed_mbps).toLocaleString()} Mb/s</span><Icon name="speed" px={20} role="accent" /></span>
						{/if}
						<span class={l.connected ? 'allowed-dot' : 'off-dot'} title={l.connected ? _('Connected') : _('Not connected')}></span>
					</div>
					{#if links.length > 1}
						<button class="bar-button" title={fill(_('Configuration Details ({port})'), { port: l.name ?? '' })} onclick={() => showPort(l.name)}>
							<Icon name="gear" px={18} />
						</button>
					{/if}
				{/snippet}
			</Row>
		</Group>
	{/each}
	{#if !links.length && !problem}
		<Group title={_('Ethernet')} icon="ethernet">
			<Row title={_('No network port found')} sub={_('The box needs a cable to the router.')} />
		</Group>
	{/if}
	{#if wifi}
		<Group title={_('Wi-Fi (Wireless Network)')} icon="wifi-off">
			<Row big icon="wifi-off" title={_('Status: Off')} sub={_('The box connects to the router by a cable.')}>
				{#snippet ends()}<Switch fixed on={false} />{/snippet}
			</Row>
		</Group>
	{/if}

	<Group title={port ? fill(_('Configuration Details ({port})'), { port }) : _('Configuration Details')} icon="gear"
		note={_('New settings are tried for a few minutes and revert unless kept.')}>
		<Row big icon="swap" title={_('IP Address Assignment Method')} sub={_('From the router (DHCP), or a fixed static address.')}>
			{#snippet ends()}<Segmented choices={[['dhcp', _('DHCP')], ['fixed', _('Static')]]} bind:chosen={mode} />{/snippet}
		</Row>
		{#each [['address', _("Box's address"), '192.168.1.40', 'ip-screen'], ['mask', _('Subnet mask'), '255.255.255.0', 'layers'],
			['router', _('Router'), '192.168.1.1', 'router'], ['dns', _('Name servers'), '192.168.1.1', 'dns']] as [key, caption, hint, icon] (key)}
			<Row big {icon} title={caption}>
				{#snippet ends()}
					{#if key === 'address'}<input type="text" style:width="260px" bind:value={address} placeholder={hint} aria-label={caption} disabled={mode !== 'fixed'} />
					{:else if key === 'mask'}<input type="text" style:width="260px" bind:value={mask} placeholder={hint} aria-label={caption} disabled={mode !== 'fixed'} />
					{:else if key === 'router'}<input type="text" style:width="260px" bind:value={router} placeholder={hint} aria-label={caption} disabled={mode !== 'fixed'} />
					{:else}<input type="text" style:width="260px" bind:value={dns} placeholder={hint} aria-label={caption} disabled={mode !== 'fixed'} />{/if}
				{/snippet}
			</Row>
		{/each}
		<Wide>
			{#if problem}<p class="error">{problem}</p>{/if}
			<div class="row-between">
				<span class="row-sub grow">
					{fill(_('Current Address: {address}.'), { address: String(link.ipv4?.[0] ?? 'none').split('/')[0] })}{undo ? ' ' + fill(_('Reverts at {time} unless kept.'), { time: when(undo) }) : ''}
				</span>
				{#if undo}<button onclick={keep}>{_('Keep them')}</button>{/if}
				<button class="primary" onclick={apply} disabled={!port}>{_('Use these settings')}</button>
			</div>
		</Wide>
	</Group>
</div>
