<script lang="ts">
	// Bring a backup back (Storage Manager › Backups, design §6.10): the
	// drive, the backup, the backup key; then what comes back, each app
	// installed or not; and, for an admin who forgot their old password, a new
	// one. Everyone else's accounts come back as they were.
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { size, when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { passwordProblem } from '../lib/people';
	import Password from '../components/Password.svelte';
	import Row from '../components/Row.svelte';
	import Sheet from '../components/Sheet.svelte';
	import Spinner from '../components/Spinner.svelte';

	let { close, started }: { close: () => void; started: (a: Answer) => void } = $props();
	let busy = $state(false);
	let problem = $state('');
	let drives = $state<Answer[] | null>(null);
	let drive = $state<Answer | null>(null);
	let backups = $state<Answer[] | null>(null);
	let chosen = $state<Answer | null>(null);
	let typed = $state('');
	let check = $state<Answer | null>(null);
	let include = $state<string[]>([]);
	let forgot = $state(false);
	let username = $state('');
	let first = $state('');
	let again = $state('');

	async function run<T>(work: () => Promise<T>): Promise<T | undefined> {
		busy = true;
		problem = '';
		try {
			return await work();
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	async function lookForDrives(): Promise<void> {
		const a = await run(() => ask('backups', { action: 'drives' }));
		if (a) drives = (a.drives ?? []).filter((d: Answer) => d.has_backups);
	}
	void lookForDrives();

	async function pickDrive(d: Answer): Promise<void> {
		const a = await run(() => ask('backups', { action: 'list', device: d.device }));
		if (a) {
			drive = d;
			backups = a.backups ?? [];
		}
	}

	async function openBackup(): Promise<void> {
		const a = await run(() => ask('backups', { action: 'check', device: drive?.device, name: chosen?.name, key: typed }));
		if (!a) return;
		check = a;
		include = (a.parts ?? []).filter((p: Answer) => p.state === 'ready').map((p: Answer) => p.id);
		username = a.accounts?.[0]?.username ?? '';
	}

	function toggle(id: string, on: boolean): void {
		include = on ? [...include, id] : include.filter((i) => i !== id);
	}

	const parts = $derived<Answer[]>(check?.parts ?? []);
	const notInstalled = $derived(parts.filter((p) => p.state === 'not-installed'));
	const accountsIn = $derived(include.includes('id'));

	function partSub(p: Answer): string {
		if (p.state === 'not-installed') return _('Not installed on this box. Install it from App Center first.');
		if (p.state === 'update-first') return fill(_('The backup is from version {version}. Update it first.'), { version: p.version ?? '' });
		if (p.id === 'id') return _("Everyone's accounts, passwords and two-step sign-in, as they were. The admin made at the first setup goes.");
		const words = [fill(_('Replaces what {app} has now'), { app: p.name ?? '' })];
		if (p.copies) words.push(fill(_('{n} files from the drive'), { n: p.copies }));
		return words.join(' · ');
	}

	function bringBack(): void {
		if (!include.length) {
			problem = _('Choose what to bring back.');
			return;
		}
		if (forgot) {
			problem = passwordProblem(first, again);
			if (problem) return;
		}
		const fields: Answer = { action: 'restore', device: drive?.device, name: chosen?.name, key: typed, include };
		if (forgot) Object.assign(fields, { username, password: first });
		desk.confirm({
			title: _('Bring this backup back?'),
			text: fill(_('What is on the box now for the apps you chose is replaced by the backup of {date}. The apps stop while it comes back.'),
				{ date: when(check?.backup?.date ?? '') }),
			button: _('Bring back'),
			action: (password) => ask('backups', fields, password),
			done: (a) => started(a as Answer)
		});
	}
</script>

<Sheet title={_('Bring a backup back')} width={620} {close}>
	{#if !drive}
		<p class="lead">{_('Plug the backup drive into the box.')}</p>
		{#if drives && !drives.length}<Row icon="alert" role="warn" title={_('No backup drive found')} sub={_('Plug in the drive with the ZOOMRIYA Backup folder, then look again.')} />{/if}
		{#each drives ?? [] as d (d.device)}
			<Row icon="disk" title={d.label || _('USB drive')} sub={size(d.size_bytes ?? 0)}>
				{#snippet ends()}<button class="primary" disabled={busy} onclick={() => pickDrive(d)}>{_('Open')}</button>{/snippet}
			</Row>
		{/each}
		<div class="row-end">{#if busy}<Spinner />{/if}<button disabled={busy} onclick={lookForDrives}>{_('Look again')}</button></div>
	{:else if !check}
		<p class="lead">{_('Choose a backup, then type the backup key printed when the drive was chosen.')}</p>
		{#if backups && !backups.length}<Row icon="alert" role="warn" title={_('No backups on this drive yet')} />{/if}
		{#each backups ?? [] as b (b.name)}
			<label class="check">
				<input type="radio" name="backup" checked={chosen?.name === b.name} onchange={() => (chosen = b)} />
				{when(b.date)} · {fill(_('{n} apps'), { n: (b.parts ?? []).filter((p: Answer) => p.id !== 'id').length })}
			</label>
		{/each}
		<input class="mono" placeholder="XXXX-XXXX-XXXX-XXXX-XXXX-XXXX" aria-label={_('Backup key')} bind:value={typed}
			autocomplete="off" spellcheck="false" style:margin-top="12px" />
		<div class="row-end">
			{#if busy}<Spinner />{/if}
			<button onclick={() => { drive = null; backups = null; }}>{_('Back')}</button>
			<button class="primary" disabled={busy || !chosen || !typed.trim()} onclick={openBackup}>{_('Open backup')}</button>
		</div>
	{:else}
		<p class="lead">{fill(_('The backup of {date}. Choose what comes back.'), { date: when(check.backup?.date ?? '') })}</p>
		{#each parts as p (p.id)}
			<Row icon={p.state === 'ready' ? 'app' : 'alert'} role={p.state === 'ready' ? 'accent' : 'warn'} title={p.name ?? ''} sub={partSub(p)}>
				{#snippet lead()}
					<input type="checkbox" aria-label={p.name ?? ''} disabled={p.state !== 'ready'} checked={include.includes(p.id)}
						onchange={(e) => toggle(p.id, (e.currentTarget as HTMLInputElement).checked)} />
				{/snippet}
			</Row>
		{/each}
		{#if notInstalled.length}
			<div class="row-end"><button onclick={() => { close(); desk.openTool('apps', 'usb'); }}>{_('Open App Center')}</button></div>
		{/if}
		{#if accountsIn && (check.accounts ?? []).length}
			<label class="check" style:margin-top="12px">
				<input type="checkbox" bind:checked={forgot} />
				{_('I forgot my old password')}
			</label>
			{#if forgot}
				<p class="row-sub">{_('Your account comes back with a new password, and you set up two-step sign-in again. The backup key proves the backup is yours.')}</p>
				<select bind:value={username} aria-label={_('Your account')}>
					{#each check.accounts as account (account.username)}<option value={account.username}>{account.name} ({account.username})</option>{/each}
				</select>
				<Password bind:value={first} placeholder={_('New password')} autocomplete="new-password" />
				<Password bind:value={again} placeholder={_('Password again')} autocomplete="new-password" />
			{/if}
		{/if}
		<div class="row-end">
			<button onclick={() => (check = null)}>{_('Back')}</button>
			<button class="primary" disabled={busy || !include.length} onclick={bringBack}>{_('Bring back')}</button>
		</div>
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</Sheet>
