<script lang="ts">
	// Control Panel › Remote access: on or off, how the box is reached away
	// from home, and what would make it faster (zoomriya-connect
	// docs/direct-plus.md §4, §8 A). Through box-desktopd to connectd
	// (connect.go); no ZOOMRIYA server is involved.
	import { onDestroy } from 'svelte';
	import { day } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import StatusTile from '../components/StatusTile.svelte';
	import Switch from '../components/Switch.svelte';
	import { desk } from '../lib/desk.svelte';
	import { deviceWords, iso, status as getStatus } from './devices';
	import OwnName from './OwnName.svelte';
	import {
		forgetNames, guests as getGuests, homeCase, names as getNames, setAway, steps,
		type GuestsStatus, type NamesStatus, type RemoteStatus, type Step
	} from './remote';

	let { shown = true }: { shown?: boolean } = $props();
	let s = $state<RemoteStatus | null>(null);
	let own = $state<NamesStatus | null>(null);
	let guest = $state<GuestsStatus | null>(null);
	let problem = $state('');
	let copied = $state(false);

	async function load(): Promise<void> {
		try {
			s = (await getStatus()) as RemoteStatus;
			own = await getNames();
			guest = await getGuests();
			problem = '';
		} catch (e) {
			problem = deviceWords(e);
		}
	}

	async function change(on: boolean): Promise<void> {
		try {
			await setAway(on);
		} catch (e) {
			problem = deviceWords(e);
		}
		await load();
	}

	// The router's answers and the addresses change slowly.
	const tick = setInterval(() => shown && void load(), 30000);
	onDestroy(() => clearInterval(tick));
	void load();

	const kase = $derived(s ? homeCase(s) : null);
	const todo = $derived(s ? steps(s) : []);
	const message = $derived(
		_('Hello. My home internet connection shares its address with other customers. Please give my home its own IPv6 addresses, or a public IPv4 address, so devices can reach my home server directly. Thank you.')
	);

	function stepTitle(step: Step): string {
		switch (step) {
			case 'ipv6': return _('Turn on IPv6 in your router');
			case 'ipv6-firewall': return _("Allow the box in your router's IPv6 firewall");
			case 'upnp': return _('Turn on UPnP in your router');
			case 'bridge': return _("Put your provider's modem in bridge mode");
			case 'provider': return _('Ask your internet provider');
		}
	}

	function stepWords(step: Step, port: number): string {
		switch (step) {
			case 'ipv6': return _("It is in the router's settings, often under Internet or WAN. The box finds its new address by itself.");
			case 'ipv6-firewall': return fill(_('Allow incoming connections to this box on UDP port {port}. Some routers call this a pinhole or an exposed host.'), { port });
			case 'upnp': return _('The box then asks the router for its own port, by itself.');
			case 'bridge': return _('Two routers are in a row. With the modem in bridge mode, your own router can open a port for the box.');
			case 'provider': return _('Your home shares its internet address with other homes. Send your provider this message:');
		}
	}

	function removeName(): void {
		desk.confirm({
			title: _('Stop using your own name?'),
			text: _('The box stops changing its records. The apps keep working.'),
			button: _('Stop'),
			action: async (password) => {
				try {
					return await forgetNames(password);
				} catch (e) {
					throw new Error(deviceWords(e));
				}
			},
			done: () => void load()
		});
	}

	async function copy(text = message): Promise<void> {
		try {
			await navigator.clipboard.writeText(text);
			copied = true;
			setTimeout(() => (copied = false), 3000);
		} catch {
			/* the words stay on screen to copy by hand */
		}
	}
</script>

<div class="settings">
	{#if s && kase}
		{#if kase === 'off'}
			<StatusTile icon="box" title={_('At home only')} words={_('Paired phones and laptops reach the box on the home network only.')} state="info" />
		{:else if kase === 'open'}
			<StatusTile icon="shield" title={_('Reached directly from anywhere')} words={_('Your router opened a port for the box. Phones and laptops connect at full speed.')} state="ok" />
		{:else if kase === 'ipv6'}
			<StatusTile icon="shield" title={_('Reached directly over IPv6')} words={_('Phones and laptops on networks with IPv6 connect at full speed. On other networks, they take a few seconds longer to connect.')} state="ok" />
		{:else}
			<StatusTile icon="alert" title={_('Remote access works')} words={_('Phones and laptops can take a few seconds to connect, and big files wait for a faster connection. The steps below help.')} state="warn" />
		{/if}

		<Group title={_('Remote access')} icon="connect">
			<Row big icon="globe" title={_('Use the box away from home')}
				sub={_('Paired phones and laptops reach the box from anywhere. When off, at home only.')}>
				{#snippet ends()}<Switch on={s!.away} changed={(on) => void change(on)} label={_('Use the box away from home')} />{/snippet}
			</Row>
		</Group>

		{#if s.away}
			<Group title={_('How the box is reached')} icon="router">
				<Row icon="router" title={_('Port on the router')}
					sub={s.reach.port_mapped
						? (s.reach.double_nat ? _('Opened, but on the inner of two routers') : _('Opened by the router'))
						: _('Not opened')} />
				<Row icon="router" title={_('IPv6')}
					sub={s.reach.ipv6.length ? _('The box has its own IPv6 address') : _('Not available')} />
				<Row icon="connect" title={_('When nothing direct works')}
					sub={s.tor ? _('Ready: phones and laptops still reach the box') : _('Starting')} />
				{#if s.reach.published_at}
					<Row icon="clock" title={_('Address last shared with your devices')} sub={day(iso(s.reach.published_at), true)} />
				{/if}
			</Group>

			<Group title={_('Your own name')} icon="globe"
				note={_('Optional: a domain of your own gives a real padlock in any browser, and a name for guests.')}>
				{#if own?.domain}
					<Row icon="globe" title={own.home ?? ''}
						sub={own.records.length ? own.records.join(', ') : _('No address that reaches the box yet')}>
						{#snippet ends()}<button onclick={removeName}>{_('Stop using it')}</button>{/snippet}
					</Row>
					<Row icon="lock" title={_('Certificate')}
						sub={own.certificate_days !== null
							? fill(_('Valid for {n} more days; renewed by itself'), { n: own.certificate_days })
							: _('Being requested')} />
					{#if own.caa}
						<Row icon="shield" title={_('Recommended: limit certificates to this box')}
							sub={fill(_('Add this CAA record to {domain}:'), { domain: own.domain })} />
						<div class="message">
							<p><code>{own.caa}</code></p>
							<button onclick={() => copy(own!.caa!)}>{_('Copy')}</button>
						</div>
					{/if}
					{#if guest?.door}
						<Row icon="picture" title={_('Guest links from outside')} sub={guest.door_reachable && guest.door_only_viewer !== false
							? fill(_('Open, checked from outside {date}: guests see the album viewer only'), { date: guest.checked_at ? day(iso(guest.checked_at), true) : '' })
							: guest.door_only_viewer === false
								? _('More than the album viewer answers from outside. Open Support Center.')
								: guest.door_mapped
									? _('The router opened it; waiting for the certificate and the check from outside')
									: _('The router did not open it. Guest links work at home; for guests elsewhere, send the photos.')} />
					{/if}
					{#if own.strangers.length}
						<Row icon="alert" title={_('Certificates this box did not ask for')}
							sub={_('Someone got a certificate for your names. If you did not, check who can change your DNS records.')} />
						{#each own.strangers as stranger (stranger)}<p class="row-sub stranger">{stranger}</p>{/each}
					{/if}
					{#if own.problem}<p class="error">{own.problem}</p>{/if}
				{:else}
					<Row icon="globe" title={_('Use your own name')}
						sub={_('Needs a domain and a key from your DNS provider for dynamic updates.')}>
						{#snippet ends()}<button onclick={() => desk.sheet(OwnName, { done: () => void load() })}>{_('Set up')}</button>{/snippet}
					</Row>
				{/if}
			</Group>

			{#if todo.length}
				<Group title={_('Make remote access faster')} icon="speed"
					note={_('Any one of these helps. Each is done once, in your router or with your provider.')}>
					{#each todo as step (step)}
						<Row icon="gear" title={stepTitle(step)} sub={stepWords(step, s.port)} />
						{#if step === 'provider'}
							<div class="message">
								<p>{message}</p>
								<button onclick={() => copy()}>{copied ? _('Copied') : _('Copy')}</button>
							</div>
						{/if}
					{/each}
				</Group>
			{/if}
		{/if}
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

<style>
	.message {
		display: flex;
		gap: 12px;
		align-items: flex-start;
		padding: 12px 16px;
		user-select: text;
	}
	.message p { flex: 1; margin: 0; overflow-wrap: anywhere; }
	.stranger { padding: 0 16px; }
</style>
