<script lang="ts">
	// The admin's shortcuts down the desktop's left (app.py's fill_shortcuts):
	// a tool's, or an app's while it's on the box; dropped there from the menu
	// or the launcher.
	import { desk, TOOLS } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import AppTile from '../components/AppTile.svelte';
	import Tile from '../components/Tile.svelte';

	let menuFor = $state('');
	let menuTop = $state(0);
	const shown = $derived(desk.prefs.shortcuts.filter((k) => k in TOOLS || (k.startsWith('app:') && k.slice(4) in desk.apps)));

	function openMenu(kind: string, e: Event): void {
		e.preventDefault();
		menuTop = (e.currentTarget as HTMLElement).offsetTop;
		menuFor = kind;
	}
</script>

<svelte:window onpointerdown={(e) => { if (!(e.target as HTMLElement).closest('.app-menu')) menuFor = ''; }} />

<div class="shortcuts">
	{#each shown as kind (kind)}
		{@const app = kind.startsWith('app:') ? desk.apps[kind.slice(4)] : null}
		<button class="shortcut" title={desk.title(kind)} onclick={() => desk.dockClicked(kind)} oncontextmenu={(e) => openMenu(kind, e)}>
			{#if app}<AppTile id={app.id} name={app.name ?? ''} size={46} />{:else}<Tile {kind} size={46} />{/if}
			<span class="shortcut-name">{desk.title(kind)}</span>
		</button>
	{/each}
	{#if menuFor}
		{@const kind = menuFor}
		<div class="popover app-menu" style:left="104px" style:top="{menuTop}px">
			<button class="menu-item" onclick={() => { menuFor = ''; desk.dockClicked(kind); }}>{_('Open')}</button>
			<button class="menu-item" onclick={() => { menuFor = ''; desk.removeShortcut(kind); }}>{_('Remove from desktop')}</button>
		</div>
	{/if}
</div>
