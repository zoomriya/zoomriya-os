<script lang="ts">
	// One window on the desktop (app.py's make_window): its bar (tile, name,
	// the ? for help, minimise, maximise, close), moved by its bar; its
	// sections down the side, each drawn the first time it's shown and kept;
	// or an app's pages, from the box's own address.
	import { desk, DOCK_ROOM, SECTION_ICONS, sections, TOP_BAR, type Win } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import AppTile from '../components/AppTile.svelte';
	import Badge from '../components/Badge.svelte';
	import Icon from '../components/Icon.svelte';
	import Tile from '../components/Tile.svelte';
	import { SECTIONS } from '../tools/registry';

	let { win, z }: { win: Win; z: number } = $props();
	/** Apps whose pages no other page may show (Vault keeps an unlocked vault
	 * in its own window group): opened in a tab of their own instead. */
	const NOT_FRAMED = new Set(['vault']);
	const isApp = $derived(win.kind.startsWith('app:'));
	const listed = $derived(isApp ? [] : sections(win.kind, desk.hasSound));
	const active = $derived(desk.active === win.kind && !win.minimised);
	let built = $state<string[]>([]);
	$effect(() => {
		if (win.section && !built.includes(win.section)) built = [...built, win.section];
	});
	const app = $derived(isApp ? desk.apps[win.kind.slice(4)] : null);

	let start: { x: number; y: number; px: number; py: number } | null = null;

	function down(e: PointerEvent): void {
		if ((e.target as HTMLElement).closest('button')) return;
		desk.raise(win.kind);
		start = { x: win.x, y: win.y, px: e.clientX, py: e.clientY };
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}

	function move(e: PointerEvent): void {
		if (start) desk.drag(win.kind, start.x, start.y, e.clientX - start.px, e.clientY - start.py);
	}

	function up(): void {
		if (start) desk.rememberLayout();
		start = null;
	}
</script>

<div class="window" class:active class:maximised={win.maximised} class:opening={win.opening} class:closing={win.closing}
	class:hidden={win.minimised && !win.closing} class:app-window={isApp}
	style:left="{win.maximised ? 0 : win.x}px" style:top="{win.maximised ? 0 : win.y}px"
	style:width={win.maximised ? '100%' : `${win.width}px`} style:height={win.maximised ? `calc(100vh - ${TOP_BAR + DOCK_ROOM}px)` : `${win.height}px`}
	style:z-index={z} role="dialog" aria-label={desk.title(win.kind)}
	onpointerdowncapture={() => { if (desk.active !== win.kind) desk.raise(win.kind); }}>
	<div class="window-bar" role="toolbar" tabindex="-1" onpointerdown={down} onpointermove={move} onpointerup={up}
		onpointercancel={up} ondblclick={() => desk.maximise(win.kind)}>
		{#if isApp}<AppTile id={win.kind.slice(4)} name={desk.title(win.kind)} size={22} />{:else}<Tile kind={win.kind} size={22} />{/if}
		<span class="window-title">{desk.title(win.kind)}</span>
		{#if win.kind !== 'support' && !isApp}
			<button class="window-button" title={_('Help')} aria-label={_('Help')} onclick={() => desk.openTool('support', 'help')}>
				<Icon name="help" px={14} role="muted" />
			</button>
		{/if}
		<button class="window-button minimise" aria-label={_('Minimise')} onclick={() => desk.minimise(win.kind)}><Icon name="minimise" px={12} role="muted" /></button>
		<button class="window-button maximise" aria-label={_('Maximise')} onclick={() => desk.maximise(win.kind)}><Icon name="maximise" px={12} role="muted" /></button>
		<button class="window-button close" aria-label={_('Close')} onclick={() => desk.close(win.kind)}><Icon name="close" px={12} role="muted" /></button>
	</div>
	<div class="window-body">
		{#if isApp}
			<div class="window-pages">
				{#if app && app.health !== undefined && app.health !== 'running'}
					<div class="unavailable">
						<p class="lead">{fill(_("{app} isn't running. Start or restore it in App Center."), { app: app.name ?? '' })}</p>
						<button onclick={() => desk.openTool('apps', 'installed')}>{_('Open App Center')}</button>
					</div>
				{:else if NOT_FRAMED.has(win.kind.slice(4))}
					<div class="unavailable">
						<p class="lead">{fill(_('{app} opens on its own, for its safety.'), { app: app?.name ?? '' })}</p>
						<button class="primary" onclick={() => window.open(desk.appAddress(win.kind.slice(4)), '_blank', 'noopener')}>
							{fill(_('Open {app}'), { app: app?.name ?? '' })}
						</button>
					</div>
				{:else}
					<iframe class="app-frame" title={desk.title(win.kind)} src={desk.appAddress(win.kind.slice(4))}
						referrerpolicy="same-origin" allow="" sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-downloads allow-modals"></iframe>
				{/if}
			</div>
		{:else}
			{#if listed.length > 1}
				<nav class="window-sidebar" aria-label={desk.title(win.kind)}>
					{#each listed as [id, title] (id)}
						<button class:chosen={win.section === id} aria-current={win.section === id ? 'page' : undefined}
							onclick={() => desk.showSection(win.kind, id)}>
							{#if SECTION_ICONS[win.kind]?.[id]}<Badge name={SECTION_ICONS[win.kind][id]} px={22} />{/if}
							<span class="name"><span>{_(title)}</span><span class="bold" aria-hidden="true">{_(title)}</span></span>
						</button>
					{/each}
				</nav>
			{/if}
			<div class="window-pages">
				{#each listed as [id] (id)}
					{#if built.includes(id)}
						{@const Section = SECTIONS[`${win.kind} ${id}`]}
						<div class="window-page" hidden={win.section !== id} class:settings-page={win.kind === 'control' || win.kind === 'storage' || win.kind === 'apps' || win.kind === 'support'}>
							<div class="page-body">{#if Section}<Section shown={win.section === id && !win.minimised} />{/if}</div>
						</div>
					{/if}
				{/each}
			</div>
		{/if}
	</div>
</div>
