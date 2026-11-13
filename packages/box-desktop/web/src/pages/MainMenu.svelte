<script lang="ts">
	// The main menu (app.py's main_menu): each tool as a large icon, with a
	// search box; any tool can be dragged to the desktop as a shortcut. Its
	// foot shows who's signed in and the box, with Open Family Hub on a phone,
	// Lock screen, Restart the box and Sign out.
	import { desk, sections, TOOLS } from '../lib/desk.svelte';
	import { initials } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { search, type Found } from '../lib/search';
	import { HELP } from '../lib/words';
	import Icon from '../components/Icon.svelte';
	import Tile from '../components/Tile.svelte';
	import { openElsewhere } from './elsewhere';

	let typed = $state('');
	let chosen = $state(0);
	let field: HTMLInputElement;
	$effect(() => {
		field?.focus();
	});

	const tools = $derived(Object.entries(TOOLS).filter(([, [english]]) => _(english).toLowerCase().includes(typed.toLowerCase())));
	const results = $derived.by((): Found[] => {
		const text = typed.trim();
		if (!text) return [];
		const names = Object.fromEntries(Object.entries(TOOLS).map(([k, [english]]) => [k, english]));
		const titles = new Map<string, string>();
		for (const kind of Object.keys(TOOLS)) for (const [id, title] of sections(kind, desk.hasSound)) titles.set(`${kind} ${id}`, title);
		const found = search(text, names, titles, HELP);
		found.push({ title: fill(_('Search the system log for “{text}”'), { text }), where: `${_('Log Center')} › ${_('System log')}`,
			kind: '', section: '', action: 'log', score: 0 });
		return found;
	});
	$effect(() => {
		void typed;
		chosen = 0;
	});

	function goTo(found: Found): void {
		const text = typed.trim();
		desk.menuOpen = false;
		if (found.kind) desk.openTool(found.kind, found.section);
		else if (found.action === 'log') desk.searchLog(text);
		else if (found.action === 'restart') desk.restart();
		else if (found.action === 'shut-down') desk.shutDown();
		else if (found.action === 'sign-out') void desk.signOut();
		else if (found.action === 'lock') void desk.lock();
		else if (found.action === 'hub-phone') hubElsewhere();
		else if (found.action === 'midnight' || found.action === 'daylight') desk.setTheme(found.action);
		else if (found.action === 'widgets') desk.widgetsShown = !desk.widgetsShown;
	}

	/** Enter in the search: the first tool shown, or else the chosen result. */
	function enter(): void {
		const text = typed.trim().toLowerCase();
		for (const [kind, [english]] of Object.entries(TOOLS)) {
			if (text && _(english).toLowerCase().startsWith(text)) {
				desk.openTool(kind);
				return;
			}
		}
		if (results[chosen]) goTo(results[chosen]);
	}

	function keys(e: KeyboardEvent): void {
		if (e.key === 'Enter') enter();
		else if (e.key === 'ArrowDown') chosen = Math.min(chosen + 1, results.length - 1);
		else if (e.key === 'ArrowUp') chosen = Math.max(chosen - 1, 0);
		else return;
		e.preventDefault();
	}

	function hubElsewhere(): void {
		openElsewhere(_('Family Hub'), `https://${desk.boxName || location.hostname}/`);
	}

	const footer = $derived(desk.boxName ? `${desk.boxName} · ${desk.osName}` : desk.osName);
</script>

<div class="popover" style:left="0" style:top="40px">
	<div class="menu-body">
		<input bind:this={field} type="search" bind:value={typed} placeholder={_('Search tools')} aria-label={_('Search tools')} onkeydown={keys} />
		<div class="menu-grid">
			{#each tools as [kind, [english]] (kind)}
				<button class="menu-tool" draggable="true" onclick={() => desk.openTool(kind)}
					ondragstart={(e) => { e.dataTransfer?.setData('text/x-box-desktop', kind); desk.menuOpen = false; }}>
					<Tile {kind} size={48} />
					<span>{_(english)}</span>
				</button>
			{/each}
		</div>
		{#if results.length}
			<div class="menu-results" role="listbox">
				{#each results as item, i (i)}
					<button class:chosen={i === chosen} role="option" aria-selected={i === chosen} onclick={() => goTo(item)}
						onpointerenter={() => (chosen = i)}>
						<Icon name={item.action === 'log' ? 'search' : item.action ? 'power' : item.section === 'help' ? 'help' : 'gear'} px={18} role="muted" />
						<span style:display="flex" style:flex-direction="column" style:gap="1px">
							<span class="result-title">{item.title}</span>
							{#if item.where}<span class="result-where">{item.where}</span>{/if}
						</span>
					</button>
				{/each}
			</div>
		{/if}
		<div class="menu-footer">
			<span class="avatar menu-avatar">{initials(desk.member?.name ?? '')}</span>
			<span style:flex="1" style:display="flex" style:flex-direction="column" style:min-width="0">
				<span class="menu-who">{desk.member?.name}</span>
				<span class="menu-box">{footer}</span>
			</span>
			{#each [['phone', _('Open Family Hub on a phone'), hubElsewhere], ['lock-screen', _('Lock screen'), () => desk.lock()],
				['restart', _('Restart the box'), () => desk.restart()], ['sign-out', _('Sign out'), () => desk.signOut()]] as [icon, tip, act] (icon)}
				<button class="bar-button" title={tip as string} aria-label={tip as string}
					onclick={() => { desk.menuOpen = false; (act as () => void)(); }}>
					<Icon name={icon as string} px={18} />
				</button>
			{/each}
		</div>
	</div>
</div>
