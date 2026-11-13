<script lang="ts">
	// App Center › Installed (apps.py's apps_installed): each app, its version
	// and whether it runs, Restore previous version, Remove, and the pin that
	// puts it on this admin's launcher; what App Center is doing, every two
	// seconds while it runs.
	import { onDestroy } from 'svelte';
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { day } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { APP_HEALTH } from '../lib/words';
	import Group from '../components/Group.svelte';
	import Icon from '../components/Icon.svelte';
	import Row from '../components/Row.svelte';
	import Spinner from '../components/Spinner.svelte';
	import RemoveApp from './RemoveApp.svelte';
	import { appsReload } from './apps';

	let { shown = true }: { shown?: boolean } = $props();
	let answer = $state<Answer | null>(null);
	let problem = $state('');
	let timer: ReturnType<typeof setInterval> | null = null;
	onDestroy(() => {
		if (timer) clearInterval(timer);
		appsReload.now = null;
	});

	function got(a: Answer): void {
		answer = a;
		// The launcher follows what's installed (a removed app leaves it).
		desk.gotApps(a.apps ?? []);
		const running = !!(a.job && !a.job.done);
		if (running && !timer) timer = setInterval(() => ask('apps').then(got).catch(() => {}), 2000);
		else if (!running && timer) {
			clearInterval(timer);
			timer = null;
		}
	}

	function load(): void {
		ask('apps').then(got).catch((e) => (problem = e.message));
	}
	appsReload.now = load;

	function restore(app: Answer): void {
		desk.confirm({
			title: fill(_('Go back to {app} {version}?'), { app: app.name ?? '', version: app.previous ?? '' }),
			text: fill(_('{app} restarts on the version before.'), { app: app.name ?? '' }),
			button: _('Restore'), action: (password) => ask('apps', { action: 'restore', name: app.id }, password), done: load
		});
	}

	const health = (state: string) => APP_HEALTH[state] ?? ['Unknown', 'warn'];
	load();
</script>

<div class="settings">
	{#if answer}
		{@const job = answer.job}
		{#if job}
			<Group attention={!!job.done && !job.ok}>
				{#if !job.done}
					<Row icon="app" iconPx={28} title={fill(_('Installing {app}'), { app: job.name ?? '' })} sub={_(job.step || 'Starting')}>
						{#snippet ends()}<Spinner />{/snippet}
					</Row>
				{:else if job.ok}
					<Row icon="check" role="ok" iconPx={28} title={fill(_('{app} {version} is installed'), { app: job.name ?? '', version: job.version ?? '' })} sub={_("It's running.")} />
				{:else}
					<Row icon="alert" role="warn" iconPx={28} title={fill(_("{app} wasn't installed"), { app: job.name ?? '' })} sub={job.error ?? ''} />
				{/if}
			</Group>
		{/if}
		<Group title={_('Installed apps')} icon="app">
			{#each answer.apps ?? [] as app (app.id)}
				{@const [words, role] = health(app.health ?? '')}
				{@const about = [fill(_('Version {version}'), { version: app.version ?? '' }), app.installed ? fill(_('Installed {date}'), { date: day(app.installed) }) : ''].filter(Boolean).join(' · ')}
				<Row big icon={app.blocked ? 'alert' : 'app'} role={app.blocked ? 'warn' : 'accent'} title={app.name ?? ''}
					sub={app.blocked ? _('This version has a known problem. Update it from a USB stick.') + '\n' + about : about}>
					{#snippet beside()}<span class="badge {role === 'ok' ? 'healthy' : 'warning'}">{_(words)}</span>{/snippet}
					{#snippet ends()}
						{#if app.previous && !app.previous_blocked}
							<button onclick={() => restore(app)}>{fill(_('Restore {version}'), { version: app.previous })}</button>
						{/if}
						<button onclick={() => desk.sheet(RemoveApp, { app, done: load })}>{_('Remove')}</button>
						<!-- Last, at the right edge: on this admin's dock or not. -->
						<button class="dock-pin" class:on={desk.inDock(app.id)} aria-pressed={desk.inDock(app.id)}
							title={desk.inDock(app.id) ? _('In dock') : _('Add to dock')} aria-label={desk.inDock(app.id) ? _('In dock') : _('Add to dock')}
							onclick={() => desk.pinApp(app.id, !desk.inDock(app.id))}>
							<Icon name="pushpin" px={17} role={desk.inDock(app.id) ? 'accent' : 'muted'} />
						</button>
					{/snippet}
				</Row>
			{/each}
			{#if !(answer.apps ?? []).length}
				<Row big icon="app" title={_('No apps yet')} sub={_('Add ZOOMRIYA apps from a USB stick.')}>
					{#snippet ends()}<button class="primary" onclick={() => desk.openTool('apps', 'usb')}>{_('Add from USB')}</button>{/snippet}
				</Row>
			{/if}
		</Group>
		{#if answer.keys === 0}<span class="group-note">{_("Apps can't be installed until the box is updated.")}</span>{/if}
	{/if}
	{#if problem}<p class="error">{problem}</p>{/if}
</div>
