<script lang="ts">
	// Control Panel › Security (control.py's cp_security): protected or not at
	// a glance, what protects the box (shown on, never turned off here), what
	// may reach it, what it reaches, internet access for the apps, and what
	// actually left the box, kept current while the page is open.
	import { onDestroy } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { day, when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Icon from '../components/Icon.svelte';
	import Row from '../components/Row.svelte';
	import StatusTile from '../components/StatusTile.svelte';
	import Switch from '../components/Switch.svelte';
	import Internet from './Internet.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	/** How often an open Security page asks again what left the box. */
	const OUTBOUND_REFRESH_SECONDS = 15;
	let answer = $state<Answer | null>(null);
	let sent = $state<Answer | null>(null);
	let problem = $state('');
	let opened = $state<Record<string, boolean>>({});
	let installing = $state(false);
	let fixesWords = $state('');
	const timers: ReturnType<typeof setInterval>[] = [];
	onDestroy(() => timers.forEach(clearInterval));

	const updates = $derived(answer?.updates ?? {});
	const firewall = $derived(answer?.firewall ?? {});
	const integrity = $derived(answer?.integrity ?? {});
	const apparmor = $derived(answer?.apparmor ?? {});
	const secureBoot = $derived(String(answer?.secure_boot ?? ''));
	const certificate = $derived(answer?.certificate ?? {});
	const wrong = $derived(
		([[_("the box's certificate has a problem"), !(certificate.problems ?? []).length], [_('security fixes are off'), updates.automatic],
			[_('the firewall is off'), firewall.on], [_("the OS's files changed"), integrity.clean !== false], [_('AppArmor is off'), apparmor.on]] as [string, boolean][])
			.filter(([, fine]) => !fine).map(([name]) => name)
	);
	const scanned = $derived(
		integrity.checked == null ? _('Not yet: files are checked nightly.')
			: integrity.clean ? fill(_('Completed {time}: OS files match the latest update.'), { time: when(integrity.checked) })
			: fill(_('Completed {time}: files changed outside an update: {files}'), { time: when(integrity.checked), files: (integrity.changes ?? []).join(', ') })
	);

	function showFixes(state: Answer): void {
		installing = !!state.running;
		const last = state.last_installed;
		let said = state.automatic ? _('Auto updates every night.') : _('Auto updates are off.');
		said += '\n' + (last ? fill(_('Last update was {date}.'), { date: day(last, true) }) : _('No update yet.'));
		fixesWords = installing ? _('Installing security updates now.') : said;
	}

	function pollFixes(): void {
		const t = setInterval(async () => {
			try {
				const a = await ask('security');
				showFixes(a.updates ?? {});
				if (!a.updates?.running) clearInterval(t);
			} catch {
				clearInterval(t);
			}
		}, 5000);
		timers.push(t);
	}

	async function checkNow(): Promise<void> {
		try {
			await ask('security-check-now');
			showFixes({ ...updates, running: true });
			pollFixes();
		} catch (e) {
			fixesWords = (e as Error).message;
		}
	}

	async function load(): Promise<void> {
		try {
			answer = await ask('security');
			showFixes(answer.updates ?? {});
			if (answer.updates?.running) pollFixes();
			sent = await ask('outbound');
			timers.push(setInterval(() => ask('outbound').then((a) => (sent = a)).catch(() => {}), OUTBOUND_REFRESH_SECONDS * 1000));
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	void load();
</script>

{#snippet connections(listed: Answer[], none: string)}
	{#each listed as c, i (i)}
		{@const times = c.count ?? 0}
		<Row icon="globe" iconPx={26} title={c.who || c.account || ''}
			sub={`${c.destination ?? ''} · ${String(c.protocol ?? '').toUpperCase()} ${c.port ?? ''}\n${times === 1 ? _('Once') : fill(_('{count} times'), { count: times })} · ${c.last ? when(c.last) : ''}`} />
	{/each}
	{#if !listed.length}<Row title={_('Nothing')} sub={none} />{/if}
{/snippet}

<div class="settings">
	{#if problem}<p class="error">{problem}</p>{/if}
	{#if answer}
		<div class="tiles">
			{#if wrong.length}
				<StatusTile icon="alert" title={_('Needs a look')} words={wrong[0].charAt(0).toUpperCase() + wrong[0].slice(1) + '.'} state="warn" />
			{:else}
				<StatusTile icon="shield" title={_('Protected')} words={_('Security updates install automatically, and the firewall is always enabled.')} state="ok" />
			{/if}
			{#if secureBoot === 'on'}
				<StatusTile icon="lock" title={_('Secure Boot is on')} words={_('Only authenticated software can boot the system.')} state="ok" />
			{:else}
				<StatusTile icon="lock" title={secureBoot === 'off' ? _('Secure Boot is off') : _('Secure Boot is unavailable')}
					words={_('Enable it in the firmware settings, if available.')} state="off" />
			{/if}
		</div>

		<Group title={_('Protection')} icon="shield">
			<Row big icon="updates" title={_('Security Updates')} sub={fixesWords}>
				{#snippet ends()}<button onclick={checkNow} disabled={installing}>{installing ? _('Checking…') : _('Check now')}</button>{/snippet}
			</Row>
			<Row big icon="firewall" title={_('Firewall')}>{#snippet ends()}<Switch fixed on={!!firewall.on} />{/snippet}</Row>
			<Row big icon="apparmor" title={_('AppArmor')} sub={apparmor.on ? fill(_('{n} programs confined'), { n: apparmor.enforced ?? 0 }) : ''}>
				{#snippet ends()}<Switch fixed on={!!apparmor.on} />{/snippet}
				{#snippet below()}
					{#if (apparmor.profiles ?? []).length}
						<details class="expander" style:margin-left="42px">
							<summary>{fill(_('The programs ({n})'), { n: apparmor.profiles.length })}</summary>
							<div style:display="flex" style:flex-direction="column" style:gap="4px" style:margin-top="6px">
								{#each apparmor.profiles as profile (profile.name)}
									<div class="row-between">
										<Icon name={profile.enforced ? 'check' : 'close'} px={16} role={profile.enforced ? 'ok' : 'warn'} />
										<span class="row-sub grow">{profile.name}</span>
										<span class="row-sub">{profile.enforced ? _('Confined') : _('Watched only')}</span>
									</div>
								{/each}
							</div>
						</details>
					{/if}
				{/snippet}
			</Row>
			<Row big icon="key" title={_('Secure Boot')}>{#snippet ends()}<Switch fixed on={secureBoot === 'on'} />{/snippet}</Row>
			<Row big icon="lock" title={_('Secure Connections')}
				sub={(certificate.problems ?? []).length ? certificate.problems[0]
					: !certificate.fingerprint ? _('Not made yet: the box makes its certificate in a few minutes.')
					: fill(_('Each phone or computer trusts the box once, from {address}. The fingerprint it shows must be: {fingerprint}'),
						{ address: certificate.trust_at ?? '', fingerprint: certificate.fingerprint })} />
			<Row big icon="refresh" title={_('OS Update')}>
				{#snippet ends()}<span class="row-value">{updates.last_installed ? day(updates.last_installed) : _('None yet')}</span>{/snippet}
			</Row>
			<Row big icon="scan" title={_('Last System Scan')} sub={scanned} />
		</Group>

		<Group title={_('Allowed Inbound Connections')} lead={_('What devices on your network can reach on this box.')} icon="firewall"
			note={_('Everything else is blocked.')}>
			{#each firewall.open ?? [] as port, i (i)}
				<Row icon={port.icon || 'app'} iconPx={26} title={port.what ?? ''}
					sub={fill(_('Port {port} ({how})'), { port: port.port ?? '', how: [String(port.protocol ?? '').toUpperCase(), port.detail ?? ''].filter(Boolean).join(', ') })}>
					{#snippet ends()}<span class="allowed-dot" title={_('Allowed')}></span>{/snippet}
				</Row>
			{/each}
			{#if !(firewall.open ?? []).length}<Row title={_('Nothing')} sub={_('No other device can reach the box.')} />{/if}
		</Group>

		<Group title={_('Outbound Internet Connections')} lead={_('For updates and the time. No identifying data is sent.')} icon="globe">
			{#each answer.connections ?? [] as c, i (i)}
				<Row big icon={c.icon || 'package'} title={c.title ? `${c.title} (${c.host})` : c.host} sub={c.what ?? ''}>
					{#snippet ends()}
						<span class="allowed-dot" title={_('Allowed')}></span>
						<button class="bar-button" title={_('Details')} aria-expanded={!!opened[i]} onclick={() => (opened[i] = !opened[i])}>
							<Icon name={opened[i] ? 'chevron-up' : 'chevron-down'} px={18} role="muted" />
						</button>
					{/snippet}
					{#snippet below()}
						{#if opened[i]}
							<div class="details">
								<span class="row-sub">{fill(_('Address: {host}'), { host: c.host ?? '' })}</span>
								{#if c.when}<span class="row-sub">{fill(_('When: {when}'), { when: _(c.when) })}</span>{/if}
								<span class="row-sub">{_('No identifying data is sent.')}</span>
							</div>
						{/if}
					{/snippet}
				</Row>
			{/each}
		</Group>

		<Internet />

		{#if sent}
			<Group title={fill(_('Sent in the Last {days} Days'), { days: sent.days ?? 7 })}
				lead={_('Every connection from the box to the internet, by the app that made it.')} icon="globe">
				{@render connections(sent.connections ?? [], _('Nothing left the box for the internet.'))}
			</Group>
		{/if}
	{/if}
</div>
