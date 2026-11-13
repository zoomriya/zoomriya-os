<script lang="ts">
	// The top bar (app.py's top_bar): the main menu, one tab per open window,
	// a restart that's needed, notifications, the volume, the network cable,
	// the widgets and the clock.
	import { onDestroy } from 'svelte';
	import { post, get } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { topBarClock, when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import AppTile from '../components/AppTile.svelte';
	import Icon from '../components/Icon.svelte';
	import Level from '../components/Level.svelte';
	import Logo from '../components/Logo.svelte';
	import Tile from '../components/Tile.svelte';
	import MainMenu from './MainMenu.svelte';

	let now = $state(new Date());
	const timer = setInterval(() => (now = new Date()), 1000);
	onDestroy(() => clearInterval(timer));
	let bellOpen = $state(false);
	let volumeOpen = $state(false);
	let volume = $state(0);

	const links = $derived(((desk.network.links ?? []) as Record<string, unknown>[]).filter((x) => x.kind === 'ethernet'));
	const connected = $derived(links.some((x) => x.connected));

	/** The pill: the library being checked, a restart that's needed, advanced access on. */
	const pill = $derived.by(() => {
		const check = desk.storage.library?.check ?? {};
		if (check.running && !check.paused)
			return { words: fill(_('Checking the library · {percent}%'), { percent: Number(check.percent ?? 0).toFixed(0) }),
				busy: true, kind: 'storage', section: 'overview' };
		if (desk.status.restart_needed) return { words: _('Restart needed'), busy: false, kind: 'control', section: 'restarts' };
		if (desk.status.advanced_access) return { words: _('Advanced access is on'), busy: false, kind: 'control', section: 'access' };
		return null;
	});

	async function openVolume(): Promise<void> {
		volumeOpen = !volumeOpen;
		if (volumeOpen) {
			try {
				const sound = await get<Record<string, { volume: number }>>('/api/sound');
				volume = sound.output?.volume ?? 0;
			} catch {
				// The slider shows where it was.
			}
		}
	}

	function setVolume(v: number): void {
		void post('/api/sound', { action: 'volume', device: 'output', level: v }).catch(() => {});
	}

	function closeAll(e: MouseEvent): void {
		const target = e.target as HTMLElement;
		if (!target.closest('.bell-place')) bellOpen = false;
		if (!target.closest('.volume-place')) volumeOpen = false;
		if (!target.closest('.menu-place')) desk.menuOpen = false;
	}
</script>

<svelte:window onpointerdown={closeAll} />

<header class="topbar">
	<div class="menu-place" style:position="relative">
		<button class="bar-button menu-button" class:on={desk.menuOpen} title={_('Main menu')} aria-label={_('Main menu')}
			aria-expanded={desk.menuOpen} onclick={() => (desk.menuOpen = !desk.menuOpen)}>
			<Logo mark height={14} />
		</button>
		{#if desk.menuOpen}<MainMenu />{/if}
	</div>
	<span class="divider"></span>
	<nav class="tabs" aria-label={_('Open windows')}>
		{#each desk.windows as win (win.kind)}
			<button class="tab" class:active={desk.active === win.kind && !win.minimised} onclick={() => desk.tabClicked(win.kind)}>
				{#if win.kind.startsWith('app:')}<AppTile id={win.kind.slice(4)} name={desk.title(win.kind)} size={18} />{:else}<Tile kind={win.kind} size={18} />{/if}
				<span>{desk.title(win.kind)}</span>
			</button>
		{/each}
	</nav>
	{#if pill}
		<button class="pill" class:busy={pill.busy} onclick={() => desk.openTool(pill.kind, pill.section)}>{pill.words}</button>
	{/if}

	<div class="bell-place" style:position="relative">
		<button class="bar-button" class:on={bellOpen} title={_('Notifications')} aria-label={_('Notifications')} onclick={() => (bellOpen = !bellOpen)}>
			<Icon name="bell" px={20} />
			{#if desk.unread().length}<span class="badge-count">{desk.unread().length}</span>{/if}
		</button>
		{#if bellOpen}
			<div class="popover" style:right="0" style:top="40px" style:width="340px">
				<div class="popover-body">
					<div class="row-between">
						<span class="section grow">{_('Notifications')}</span>
						{#if desk.unread().length}<button class="flat-link" onclick={() => desk.markRead()}>{_('Mark all read')}</button>{/if}
					</div>
					{#if !desk.shownNotes().length}<p class="lead">{_('Nothing new.')}</p>{/if}
					{#each desk.shownNotes().slice(0, 8) as n, i (i)}
						<div class="notice-row" class:read={(n.time ?? '') <= (desk.prefs.notifications?.read_until ?? '')}>
							<span class="fact-name {n.level ?? ''}">{when(n.time)}</span>
							<span class="fact">{n.message}</span>
						</div>
					{/each}
					<div><button class="flat-link" onclick={() => { bellOpen = false; desk.openTool('logs', 'notifications'); }}>{_('See all in Log Center')}</button></div>
				</div>
			</div>
		{/if}
	</div>

	{#if desk.hasSound}
		<div class="volume-place" style:position="relative">
			<button class="bar-button" class:on={volumeOpen} title={_('Sound')} aria-label={_('Sound')} onclick={openVolume}>
				<Icon name="volume" px={20} />
			</button>
			{#if volumeOpen}
				<div class="popover" style:right="0" style:top="40px">
					<div class="popover-body" style:gap="8px">
						<span class="field-label">{_('Speakers')}</span>
						<Level bind:value={volume} changed={setVolume} label={_('Speakers')} />
						<button onclick={() => { volumeOpen = false; desk.openTool('control', 'sound'); }}>{_('Sound settings')}</button>
					</div>
				</div>
			{/if}
		</div>
	{/if}

	<!-- The network cable: plugged in and connected, or not (Control Panel › Network). -->
	<button class="bar-button" title={connected ? _('Connected') : _('Not connected')} aria-label={connected ? _('Connected') : _('Not connected')}
		onclick={() => desk.openTool('control', 'network')}>
		{#if connected}<Icon name="ethernet" px={20} />{:else}<Icon name="ethernet-off" px={20} role="warn" />{/if}
	</button>
	<button class="bar-button" class:on={desk.widgetsShown} title={_('Widgets')} aria-label={_('Widgets')} aria-pressed={desk.widgetsShown}
		onclick={() => (desk.widgetsShown = !desk.widgetsShown)}>
		<Icon name="widgets" px={20} />
	</button>
	<span class="clock">{topBarClock(now)}</span>
</header>
