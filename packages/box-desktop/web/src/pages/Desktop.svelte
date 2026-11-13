<script lang="ts">
	// The desktop (app.py's desktop_page): the box's background, the top bar,
	// the shortcuts down the left, the widgets down the right, the windows
	// over them, the launcher over the windows, and sheets over everything.
	// Its keys: Super (or Ctrl+Space in a browser, which keeps Super) opens or
	// closes the main menu, Super+L locks, Escape closes the sheet on top,
	// Ctrl+W closes the window in front, Alt+Tab goes to the next window.
	import { onMount } from 'svelte';
	import { desk } from '../lib/desk.svelte';
	import Confirm from '../components/Confirm.svelte';
	import Backdrop from './Backdrop.svelte';
	import Dock from './Dock.svelte';
	import Shortcuts from './Shortcuts.svelte';
	import TopBar from './TopBar.svelte';
	import Widgets from './Widgets.svelte';
	import Window from './Window.svelte';

	let confirm: Confirm;
	let superAlone = false;

	onMount(() => {
		const fit = () => desk.resized(window.innerWidth, window.innerHeight);
		fit();
		window.addEventListener('resize', fit);
		return () => window.removeEventListener('resize', fit);
	});

	function keyDown(e: KeyboardEvent): void {
		superAlone = e.key === 'Meta' || e.key === 'OS';
		if (desk.page !== 'desktop') return;
		const ctrl = e.ctrlKey && !e.altKey && !e.metaKey;
		if (e.metaKey && !e.ctrlKey && !e.altKey && e.key.toLowerCase() === 'l') {
			void desk.lock();
		} else if (e.key === 'Escape' && (confirm?.close() || desk.closeTopSheet())) {
			// Closed.
		} else if (e.key === 'Escape' && desk.menuOpen) {
			desk.menuOpen = false;
		} else if (ctrl && e.key.toLowerCase() === 'w' && desk.front()) {
			desk.close(desk.front());
		} else if (e.altKey && !e.ctrlKey && e.key === 'Tab' && desk.windows.length) {
			desk.nextWindow();
		} else if (ctrl && e.key === ' ') {
			desk.menuOpen = !desk.menuOpen;
		} else {
			return;
		}
		e.preventDefault();
	}

	function keyUp(e: KeyboardEvent): void {
		if ((e.key === 'Meta' || e.key === 'OS') && superAlone && desk.page === 'desktop') desk.menuOpen = !desk.menuOpen;
		superAlone = false;
	}

	function dropped(e: DragEvent): void {
		const kind = e.dataTransfer?.getData('text/x-box-desktop');
		if (kind && desk.addShortcut(kind)) e.preventDefault();
	}
</script>

<svelte:window onkeydowncapture={keyDown} onkeyup={keyUp} />

<Backdrop />
<div class="drop-area" role="presentation" ondragover={(e) => e.dataTransfer?.types.includes('text/x-box-desktop') && e.preventDefault()} ondrop={dropped}></div>
<Shortcuts />
{#if desk.widgetsShown}<Widgets />{/if}
<div class="layer">
	{#each desk.windows as win (win.kind)}
		<Window {win} z={desk.order.indexOf(win.kind) + 1} />
	{/each}
</div>
<Dock />
<TopBar />
{#each desk.sheets as s (s.id)}
	<s.component {...s.props} />
{/each}
<Confirm bind:this={confirm} />
