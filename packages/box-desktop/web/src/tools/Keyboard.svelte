<script lang="ts">
	// Control Panel › Keyboard & Mouse (control.py's cp_keyboard): each
	// admin's own, like the theme; on the box's screen while they're signed in
	// there. The layout is also the box's own (the agent's keyboard-set).
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import Dropdown from '../components/Dropdown.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Segmented from '../components/Segmented.svelte';
	import Slider from '../components/Slider.svelte';
	import Switch from '../components/Switch.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let layouts = $state<Answer[]>([]);
	let chosen = $state(-1);
	const primary = $derived(desk.prefs.handed === 'left' ? 'right' : 'left');

	async function load(): Promise<void> {
		try {
			const answer = await ask('keyboard');
			layouts = answer.layouts ?? [];
			const mine = desk.prefs.keyboard || answer.layout;
			chosen = layouts.findIndex((x) => x.id === mine);
		} catch {
			layouts = [];
		}
	}

	function choose(i: number): void {
		const layout = layouts[i]?.id;
		if (!layout) return;
		desk.prefs.keyboard = layout;
		desk.savePrefs();
		void ask('keyboard-set', { layout }).catch(() => {});
	}

	function mouse(changed: Partial<typeof desk.prefs.mouse>): void {
		desk.prefs.mouse = { ...desk.prefs.mouse, ...changed };
		desk.savePrefs();
	}

	void load();
</script>

<div class="settings">
	<Group title={_('Keyboard')} icon="keyboard">
		<Row big icon="keyboard" title={_('Input Layout')}>
			{#snippet ends()}<Dropdown names={layouts.map((x) => x.name)} bind:selected={chosen} changed={choose} label={_('Input Layout')} />{/snippet}
		</Row>
	</Group>
	<Group title={_('Mouse')} icon="mouse">
		<Row title={_('Primary click side')} sub={_('The other button opens menus.')}>
			{#snippet ends()}
				<Segmented choices={[['left', _('Left'), 'mouse'], ['right', _('Right'), 'mouse']]} chosen={primary}
					changed={(which) => { desk.prefs.handed = which === 'right' ? 'left' : 'right'; desk.savePrefs(); }} />
			{/snippet}
		</Row>
		<Row big icon="pointer" title={_('Cursor Speed')}>
			{#snippet ends()}
				<Slider low={_('Slow')} high={_('Fast')} min={-1} max={1} value={desk.prefs.mouse.speed}
					changed={(v) => mouse({ speed: Math.round(v * 100) / 100 })} />
			{/snippet}
		</Row>
		<Row big icon="accel" title={_('Dynamic Cursor Acceleration')} sub={_('Moves further when the mouse moves fast.')}>
			{#snippet ends()}<Switch on={desk.prefs.mouse.accel} changed={(on) => mouse({ accel: on })} label={_('Dynamic Cursor Acceleration')} />{/snippet}
		</Row>
	</Group>
	<Group title={_('Scroll direction')} icon="scroll">
		<Row big icon="screen-move" title={_('Screen moves (view)')} sub={_('Scrolling moves the visible part of the page.')}>
			{#snippet ends()}<input type="radio" name="scroll" checked={!desk.prefs.mouse.natural} onchange={() => mouse({ natural: false })} aria-label={_('Screen moves (view)')} />{/snippet}
		</Row>
		<Row big icon="content-move" title={_('Content moves (finger)')} sub={_('Content follows your finger, like on a phone.')}>
			{#snippet ends()}<input type="radio" name="scroll" checked={desk.prefs.mouse.natural} onchange={() => mouse({ natural: true })} aria-label={_('Content moves (finger)')} />{/snippet}
		</Row>
	</Group>
</div>
