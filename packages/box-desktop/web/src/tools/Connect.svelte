<script lang="ts">
	// Control Panel › ZOOMRIYA Connect: the box's name, how it's doing, the
	// paired phones and laptops, and the choices (zoomriya-connect's docs).
	// Everything goes through box-desktopd to connectd (connect.go).
	import { onDestroy } from 'svelte';
	import { desk } from '../lib/desk.svelte';
	import { day } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Switch from '../components/Switch.svelte';
	import ConnectServer from './ConnectServer.svelte';
	import PairPhone from './PairPhone.svelte';
	import {
		connectWords, devices as listDevices, iso, release, removeDevice, setRelay, setup, status, turnOff, turnOn,
		type ConnectStatus, type Device
	} from './connect';

	let { shown = true }: { shown?: boolean } = $props();
	let st = $state<ConnectStatus | null>(null);
	let list = $state<Device[]>([]);
	let problem = $state('');
	let name = $state('');
	let busy = $state(false);

	async function load(): Promise<void> {
		try {
			st = await status();
			list = st.on ? await listDevices() : [];
			problem = '';
		} catch (e) {
			problem = connectWords(e);
		}
	}

	async function run(work: () => Promise<unknown>): Promise<void> {
		busy = true;
		problem = '';
		try {
			await work();
			await load();
		} catch (e) {
			problem = connectWords(e);
		} finally {
			busy = false;
		}
	}

	function withPassword(title: string, text: string, button: string, action: (password: string) => Promise<unknown>): void {
		desk.confirm({
			title, text, button,
			action: async (password) => {
				try {
					return await action(password);
				} catch (e) {
					throw new Error(connectWords(e));
				}
			},
			done: () => void load()
		});
	}

	// The relay and certificate can take a moment after switching on.
	const tick = setInterval(() => shown && void load(), 15000);
	onDestroy(() => clearInterval(tick));
	void load();

	const suffix = $derived(st?.our_server ? '.zoomriya.app' : '');
	const certificate = $derived(
		st?.certificate_days !== undefined
			? fill(_('Valid for {n} more days'), { n: st.certificate_days })
			: st?.problems?.certificate
				? _('Not yet')
				: _('Getting it…')
	);
	/** The problems connectd names, in Box Desktop's words. */
	const problems = $derived(
		Object.keys(st?.problems ?? {}).map((kind) => {
			switch (kind) {
				case 'server': return _("The box can't reach the ZOOMRIYA Connect server. It keeps trying.");
				case 'home': return _("The box couldn't point its name at your home network yet. It keeps trying.");
				case 'certificate': return _("The box couldn't get its certificate yet. It tries again in an hour.");
				default: return _('ZOOMRIYA Connect has a problem. Open Support Center if it stays.');
			}
		})
	);
	function seen(d: Device): string {
		if (d.connected) return _('Connected now');
		return d.last_seen ? fill(_('Last connected {date}'), { date: day(iso(d.last_seen), true) }) : _('Not connected yet');
	}
	function how(d: Device): string {
		const r = d.reports?.[d.reports.length - 1];
		if (!r) return '';
		return r.ms_to_direct != null ? _('Direct') : _('Through the relay');
	}
</script>

<div class="settings">
	{#if st}
		{#if !st.name}
			<Group title={_('ZOOMRIYA Connect')} icon="connect"
				lead={_('Use the ZOOMRIYA app on your phones from anywhere: on the bus, at work, on holiday. Nothing to set up on your router.')}>
				<Row icon="globe" title={_("Your box's name")} sub={_("Pick a name that doesn't say who you are: a street or a word, not your surname. Anyone can see box names.")}>
					{#snippet below()}
						<form class="name" onsubmit={(e) => { e.preventDefault(); void run(() => setup(name.trim().toLowerCase())); }}>
							<input type="text" bind:value={name} placeholder="maplehouse" aria-label={_("Your box's name")} spellcheck="false" autocomplete="off" />
							{#if suffix}<span class="row-sub">{suffix}</span>{/if}
							<button class="primary" type="submit" disabled={busy || name.trim().length < 3}>{_('Use this name')}</button>
						</form>
					{/snippet}
				</Row>
			</Group>
		{:else}
			<Group title={_('ZOOMRIYA Connect')} icon="connect" attention={!st.on || problems.length > 0}>
				<Row big icon="connect" role={st.on ? 'accent' : 'warn'} title={st.on ? _('On') : _('Off')}
					sub={st.on ? _('Paired phones reach the box from anywhere.') : _('Paired phones reach the box at home only.')}>
					{#snippet ends()}
						<Switch on={st?.on ?? false} disabled={busy} label={_('ZOOMRIYA Connect')}
							changed={(on) => {
								if (on) void run(turnOn);
								else withPassword(_('Turn ZOOMRIYA Connect off?'), _('Phones reach the box only at home until you turn it on again.'), _('Turn off'), turnOff);
							}} />
					{/snippet}
				</Row>
				<Row icon="globe" title={_("Box's name")}>
					{#snippet ends()}<span class="row-value selectable">{st?.fqdn}</span>{/snippet}
				</Row>
				{#if st.on}
					<Row icon="router" title={_('Relay')}
						sub={st.relay_off ? _('Off: phones away from home need IPv6 or a forwarded port.')
							: st.relay_connected ? _('Connected. Phones go direct whenever they can.')
							: st.relay_refused ? _('The relay turned the box away. Open Support Center.')
							: _('Connecting…')} />
					<Row icon="lock" title={_('Certificate')} sub={certificate} />
					{#if !st.clock_synced}
						<Row icon="clock" role="warn" title={_("The box can't get the time")}
							sub={_("Phones will stop working away from home in a few weeks. Check your internet connection.")} />
					{/if}
					{#each problems as p}<Row icon="alert" role="warn" title={p} />{/each}
				{/if}
			</Group>

			{#if st.on}
				<Group title={_('Paired phones and laptops')} icon="phone"
					note={_('Each one is paired at home, once. Removing one stops it at once, everywhere.')}>
					{#each list as d (d.id)}
						<Row icon="phone" title={d.name} tag={d.admin ? _('Admin') : ''}
							sub={[seen(d), how(d)].filter(Boolean).join(' · ')}>
							{#snippet ends()}
								<button onclick={() => withPassword(fill(_('Remove {name}?'), { name: d.name }),
									_("It can't reach the box any more, at home or away. Pair it again to bring it back."),
									_('Remove'), (pw) => removeDevice(d.id, pw))}>{_('Remove')}</button>
							{/snippet}
						</Row>
					{:else}
						<Row icon="phone" title={_('No phones yet')} sub={_('Pair a phone to use the box away from home.')} />
					{/each}
					<Row icon="scan" title={_('Pair a phone')} sub={_('Shows a code to scan with the ZOOMRIYA app.')}>
						{#snippet ends()}<button class="primary" onclick={() => desk.sheet(PairPhone, { done: () => void load() })}>{_('Pair a phone')}</button>{/snippet}
					</Row>
				</Group>
			{/if}
		{/if}

		<Group title={_('Server')} icon="dns">
			<Row icon="dns" title={st.our_server ? _("ZOOMRIYA's") : _('Your own')} sub={st.server}>
				{#snippet ends()}
					{#if st?.our_server}
						<button onclick={() => desk.sheet(ConnectServer, { done: () => void load() })}>{_('Use my own server')}</button>
					{:else}
						<button onclick={() => desk.sheet(ConnectServer, { done: () => void load(), ours: true })}>{_("Use ZOOMRIYA's")}</button>
					{/if}
				{/snippet}
			</Row>
			{#if st.name}
				<Row icon="router" title={_('Use the relay')}
					sub={_('When phones can\'t reach the box directly, the relay passes their encrypted connection along. It never sees what\'s in it.')}>
					{#snippet ends()}
						<Switch on={!st?.relay_off} disabled={busy} label={_('Use the relay')} changed={(on) => void run(() => setRelay(!on))} />
					{/snippet}
				</Row>
				<Row icon="trash" title={_("Give up the box's name")} sub={_('Someone else can take it after 90 days. Phones pair again with a new name.')}>
					{#snippet ends()}
						<button class="danger" onclick={() => withPassword(_("Give up the box's name?"),
							_('Phones stop reaching the box from away until you pick a new name and pair them again.'),
							_('Give it up'), release)}>{_('Give it up')}</button>
					{/snippet}
				</Row>
			{/if}
		</Group>
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

<style>
	.name { display: flex; gap: 8px; align-items: center; width: 100%; margin-top: 8px; }
	.name input { flex: 1; min-width: 0; }
</style>
