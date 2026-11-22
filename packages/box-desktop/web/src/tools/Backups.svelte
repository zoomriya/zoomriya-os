<script lang="ts">
	// Storage Manager › Backups (the agent's backups, design §6.10): the
	// backup drive, the last backup and how it went, Back up now, the nightly
	// backup, the apps in it, the backup key, and bringing a backup back.
	import { onDestroy } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { size, when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Sheet from '../components/Sheet.svelte';
	import Spinner from '../components/Spinner.svelte';
	import StatusTile from '../components/StatusTile.svelte';
	import Switch from '../components/Switch.svelte';
	import BackupKey from './BackupKey.svelte';
	import RestoreBackup from './RestoreBackup.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	/** How often an open page asks again while a backup or a restore runs. */
	const RUNNING_REFRESH_SECONDS = 5;
	let answer = $state<Answer | null>(null);
	let problem = $state('');
	let timer: ReturnType<typeof setInterval> | null = null;
	let choosing = $state(false);
	let drives = $state<Answer[] | null>(null);
	let looking = $state(false);
	let key = $state('');
	let restoring = $state(false);
	onDestroy(() => timer && clearInterval(timer));

	function got(a: Answer): void {
		answer = a;
		if (a.running && !timer) {
			timer = setInterval(() => ask('backups').then(got).catch(() => {}), RUNNING_REFRESH_SECONDS * 1000);
		} else if (!a.running && timer) {
			clearInterval(timer);
			timer = null;
		}
	}

	const act = (fields: Answer) => ask('backups', fields).then(got).catch((e) => (problem = e.message));
	ask('backups').then(got).catch((e) => (problem = e.message));

	const job = $derived<Answer | null>(answer?.job ?? null);
	const last = $derived<Answer | null>(answer?.last ?? null);
	const restoreDone = $derived(job?.action === 'restore' && job.done);

	async function lookForDrives(): Promise<void> {
		looking = true;
		problem = '';
		try {
			drives = (await ask('backups', { action: 'drives' })).drives ?? [];
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			looking = false;
		}
	}

	function chooseDrives(): void {
		choosing = true;
		drives = null;
		void lookForDrives();
	}

	const driveName = (d: Answer) => d.label || _('USB drive');
	const driveSub = (d: Answer) => [size(d.size_bytes ?? 0), (d.fstype ?? '').toUpperCase()].filter(Boolean).join(' · ');

	function choose(drive: Answer): void {
		desk.confirm({
			title: fill(_('Back up to {drive}?'), { drive: driveName(drive) }),
			text: drive.has_backups
				? _('Its backups stay. The box makes its backup key next: print it or write it down.')
				: _('Nothing on the drive is erased. The box makes its backup key next: print it or write it down.'),
			button: _('Use this drive'),
			action: (password) => ask('backups', { action: 'setup', device: drive.device }, password),
			done: (a) => {
				choosing = false;
				got(a as Answer);
				key = (a as Answer).key ?? '';
			}
		});
	}

	function showKey(): void {
		desk.confirm({
			title: _('Show the backup key?'), text: _('Anyone with the key and the drive can open the backups. Show it only where no one else is looking.'),
			button: _('Show key'), action: (password) => ask('backups', { action: 'key' }, password),
			done: (a) => (key = (a as Answer).key ?? '')
		});
	}

	function stateTile(): { icon: string; title: string; words: string; state: string } {
		if (answer?.running && job) {
			return { icon: 'backup', title: job.action === 'restore' ? _('Bringing a backup back') : _('Backing up'),
				words: job.step && job.step !== 'Starting' ? _(job.step) : fill(_('Started {time}'), { time: when(job.started ?? '') }), state: 'plain' };
		}
		if (!last) return { icon: 'backup', title: _('No backup yet'), words: _('Back up now to make the first.'), state: 'warn' };
		if (last.ok) return { icon: 'check', title: _('Backed up'), words: when(last.ended ?? ''), state: 'ok' };
		return { icon: 'alert', title: _('The last backup failed'), words: when(last.ended ?? ''), state: 'warn' };
	}
	const tile = $derived(stateTile());
</script>

<div class="settings">
	{#if answer}
		{#if restoreDone}
			<Group title={_('Bringing a backup back')} icon="backup">
				{#if job?.ok}
					<Row big icon="check" role="ok" title={fill(_('The backup of {date} is back'), { date: when(job.from ?? '') })}
						sub={job.accounts_in ? _('Everyone signs in as before, with their old password. Sign in again to go on.') : _('Its apps are running again.')} />
				{:else}
					<Row big icon="alert" role="warn" title={_('Not everything came back')} sub={job?.error || _('Each app that didn\'t is below.')} />
				{/if}
				{#each job?.problems ?? [] as p, i (i)}<Row icon="alert" role="warn" title={p} />{/each}
				{#if job?.accounts_in}
					<div class="row-end"><button class="primary" onclick={() => desk.signOut()}>{_('Sign in again')}</button></div>
				{/if}
			</Group>
		{/if}

		{#if !answer.set_up}
			<Group title={_('Backups')} icon="backup" lead={_('Back up to a USB drive of your own, plugged into the box. Photos are copied as they are; everything else is locked with a backup key you print.')}>
				<Row big icon="disk" title={_('Choose a backup drive')} sub={_('A drive used only for backups is best. Nothing on it is erased.')}>
					{#snippet ends()}<button class="primary" onclick={chooseDrives}>{_('Choose a drive')}</button>{/snippet}
				</Row>
			</Group>
		{:else}
			<div class="tiles"><StatusTile {...tile} /></div>
			<Group title={_('Backups')} icon="backup">
				<Row big icon="backup" title={_('Back up now')} sub={_("Every app's data and the accounts, as they are now.")}>
					{#snippet ends()}
						{#if answer?.running}<Spinner />{/if}
						<button class="primary" disabled={!!answer?.running || !answer?.drive_here} onclick={() => act({ action: 'now' })}>{_('Back up now')}</button>
					{/snippet}
				</Row>
				<Row big icon="calendar-clock" title={_('Nightly backup')}
					sub={answer.next ? fill(_('Next: {time}'), { time: when(answer.next) }) : _('Every night at 4:00, while the family sleeps. Leave the drive plugged in.')}>
					{#snippet ends()}<Switch on={!!answer?.nightly} changed={(on) => act({ action: 'nightly', on })} label={_('Nightly backup')} />{/snippet}
				</Row>
				<Row big icon="disk" title={_('Backup drive')}
					sub={`${answer.drive?.label || _('USB drive')} · ${answer.drive_here ? _('Plugged in') : _('Not plugged in')}`}>
					{#snippet ends()}<button disabled={!!answer?.running} onclick={chooseDrives}>{_('Change')}</button>{/snippet}
				</Row>
				<Row big icon="key" title={_('Backup key')} sub={_('Needed to bring a backup back, on this box or a new one. Keep the printed key away from the drive.')}>
					{#snippet ends()}<button onclick={showKey}>{_('Show key')}</button>{/snippet}
				</Row>
			</Group>
			{#if (answer.problems ?? []).length}
				<Group title={_('What went wrong last time')} icon="alert" note={_('The other apps were backed up. Older backups are kept.')}>
					{#each answer.problems as p, i (i)}<Row icon="alert" role="warn" title={p} />{/each}
				</Group>
			{/if}
			<Group title={_("What's backed up")} icon="layers" lead={_('Photos and documents are copied as they are, in folders by person, year and month. Older copies and deleted files are kept 30 days.')}>
				{#each answer.apps ?? [] as app (app.id)}
					<Row icon="app" title={app.name ?? ''} sub={app.required ? _('Always backed up: no app opens without the accounts.') : ''}>
						{#snippet ends()}
							<Switch on={!!app.in} fixed={!!app.required} label={app.name ?? ''} changed={(on) => act({ action: 'app', name: app.id, on })} />
						{/snippet}
					</Row>
				{/each}
			</Group>
		{/if}

		<Group title={_('Bring a backup back')} icon="backup"
			lead={_('On a new box: finish the setup, install the apps from App Center, then bring their data back here.')}>
			<Row big icon="backup" title={_('Bring a backup back')} sub={_('Choose the drive and the backup, then type the backup key.')}>
				{#snippet ends()}<button disabled={!!answer?.running} onclick={() => (restoring = true)}>{_('Bring back')}</button>{/snippet}
			</Row>
		</Group>
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>

{#if choosing}
	<Sheet title={_('Choose a backup drive')} close={() => (choosing = false)}>
		<p class="lead">{_('Plug a USB drive into the box. It may be formatted for Windows, Mac or Linux.')}</p>
		{#if looking}<Spinner />{/if}
		{#if drives && !drives.length}<Row icon="alert" role="warn" title={_('No drive found')} sub={_('Plug one in, then look again.')} />{/if}
		{#each drives ?? [] as d (d.device)}
			<Row icon="disk" title={driveName(d)} sub={driveSub(d)} tag={d.chosen ? _('in use') : d.has_backups ? _('has backups') : ''}>
				{#snippet ends()}{#if !d.chosen}<button class="primary" onclick={() => choose(d)}>{_('Use')}</button>{/if}{/snippet}
			</Row>
		{/each}
		<div class="row-end"><button disabled={looking} onclick={lookForDrives}>{_('Look again')}</button></div>
	</Sheet>
{/if}

{#if key}
	<BackupKey {key} close={() => (key = '')} />
{/if}

{#if restoring}
	<RestoreBackup close={() => (restoring = false)} started={(a) => { restoring = false; got(a); }} />
{/if}
