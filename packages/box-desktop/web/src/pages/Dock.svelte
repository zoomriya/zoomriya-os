<script lang="ts">
	// The launcher along the bottom (app.py's dock): each tool's tile, its name
	// when the pointer is on it, a dot under each one open; after a thin
	// divider, the apps this admin pinned, with their menu (right-click or
	// press and hold).
	import { desk, TOOLS, type App } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import AppTile from '../components/AppTile.svelte';
	import Tile from '../components/Tile.svelte';
	import { openElsewhere } from './elsewhere';

	let hovered = $state('');
	let menuFor = $state<App | null>(null);
	let menuLeft = $state(0);
	let hold: ReturnType<typeof setTimeout> | null = null;

	const pinned = $derived(desk.prefs.dock.map((id) => desk.apps[id]).filter((a): a is App => !!a));

	function openMenu(app: App, e: Event): void {
		e.preventDefault();
		const button = e.currentTarget as HTMLElement;
		menuLeft = button.offsetLeft + button.offsetWidth / 2;
		menuFor = app;
	}

	function pressed(app: App, e: PointerEvent): void {
		if (e.pointerType === 'mouse') return;
		hold = setTimeout(() => openMenu(app, e), 550);
	}

	function released(): void {
		if (hold) clearTimeout(hold);
		hold = null;
	}

	function act(what: () => void): void {
		menuFor = null;
		what();
	}
</script>

<svelte:window onpointerdown={(e) => { if (!(e.target as HTMLElement).closest('.app-menu')) menuFor = null; }} />

<nav class="dock" aria-label={_('Launcher')}>
	{#each Object.entries(TOOLS) as [kind, [english]] (kind)}
		<button class="dock-item" aria-label={_(english)} draggable="true" onclick={() => desk.dockClicked(kind)}
			onpointerenter={() => (hovered = kind)} onpointerleave={() => (hovered = '')}
			ondragstart={(e) => e.dataTransfer?.setData('text/x-box-desktop', kind)}>
			<Tile {kind} size={48} />
			<span class="dock-dot" class:open={!!desk.win(kind)}></span>
			{#if hovered === kind}<span class="dock-name">{_(english)}</span>{/if}
		</button>
	{/each}
	{#if pinned.length}<span class="dock-divider"></span>{/if}
	{#each pinned as app (app.id)}
		{@const kind = 'app:' + app.id}
		<button class="dock-item" class:resting={app.health !== undefined && app.health !== 'running'} aria-label={app.name}
			draggable="true" onclick={() => desk.dockClicked(kind)} oncontextmenu={(e) => openMenu(app, e)}
			onpointerdown={(e) => pressed(app, e)} onpointerup={released} onpointercancel={released}
			onpointerenter={() => (hovered = kind)} onpointerleave={() => { hovered = ''; released(); }}
			ondragstart={(e) => e.dataTransfer?.setData('text/x-box-desktop', kind)}>
			<AppTile id={app.id} name={app.name ?? ''} size={48} />
			<span class="dock-dot" class:open={!!desk.win(kind)}></span>
			{#if hovered === kind && !menuFor}<span class="dock-name">{app.name}</span>{/if}
		</button>
	{/each}
	{#if menuFor}
		{@const app = menuFor}
		<div class="popover app-menu" style:left="{menuLeft}px" style:bottom="calc(100% + 10px)" style:transform="translateX(-50%)">
			<div class="app-menu-title">{app.name}</div>
			<button class="menu-item" onclick={() => act(() => desk.dockClicked('app:' + app.id))}>{_('Open')}</button>
			<button class="menu-item" onclick={() => act(() => openElsewhere(app.name ?? '', desk.appAddress(app.id)))}>{_('Open on a phone or computer')}</button>
			<button class="menu-item" onclick={() => act(() => desk.openTool('apps', 'installed'))}>{_('Manage in App Center')}</button>
			<button class="menu-item" onclick={() => act(() => desk.pinApp(app.id, false))}>{_('Remove from dock')}</button>
		</div>
	{/if}
</nav>
