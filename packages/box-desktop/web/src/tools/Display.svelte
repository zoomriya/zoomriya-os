<script lang="ts">
	// Control Panel › Display (control.py's cp_display): when the box's screen
	// locks and turns off, then the box's own screen, tried at once and kept,
	// or put back after 15 seconds.
	import { onDestroy } from 'svelte';
	import { ask, get, post } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import { ORIENTATIONS, SCALES } from '../lib/words';
	import Dropdown from '../components/Dropdown.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	interface Mode { width: number; height: number; refresh: number }
	interface Output { name: string; title: string; modes: Mode[]; mode: Mode; scale: number; transform: string }

	const LOCK_AFTER = [0, 1, 2, 5, 10, 15, 30];
	const SCREEN_OFF_AFTER = [0, 5, 10, 15, 30, 60];
	/** How long a new screen setting waits to be kept before going back. */
	const KEEP_SECONDS = 15;

	let outputs = $state<Output[]>([]);
	let loaded = $state(false);
	let question = $state('');
	let saved = $state('');
	let before: { out: Output; mode: Mode; scale: number; transform: string } | null = null;
	let timer: ReturnType<typeof setInterval> | null = null;
	let left = 0;

	const minutes = (n: number) => (n === 0 ? _('Never') : n === 1 ? _('1 minute') : fill(_('{n} minutes'), { n }));

	function ratio(w: number, h: number): string {
		for (const [name, [a, b]] of Object.entries({ '16:9': [16, 9], '16:10': [16, 10], '4:3': [4, 3], '21:9': [64, 27], '5:4': [5, 4], '3:2': [3, 2] }))
			if (Math.abs(w / h - a / b) < 0.01) return name;
		return '';
	}

	/** The resolutions it can show, biggest first, each once. */
	function sizes(o: Output): [number, number][] {
		const seen = new Map<string, [number, number]>();
		for (const m of o.modes) seen.set(`${m.width}x${m.height}`, [m.width, m.height]);
		return [...seen.values()].sort((a, b) => b[0] * b[1] - a[0] * a[1]);
	}

	function rates(o: Output, w: number, h: number): number[] {
		return [...new Set(o.modes.filter((m) => m.width === w && m.height === h).map((m) => m.refresh))].sort((a, b) => b - a);
	}

	async function load(): Promise<void> {
		try {
			outputs = (await get<{ outputs: Output[] }>('/api/display')).outputs;
		} catch {
			outputs = [];
		}
		loaded = true;
	}

	async function apply(o: Output, mode: Mode, scale: number, transform: string): Promise<void> {
		if (!before) before = { out: o, mode: o.mode, scale: o.scale, transform: o.transform };
		await post('/api/display', { name: o.name, mode, scale, transform }).catch(() => {});
		o.mode = mode;
		o.scale = scale;
		o.transform = transform;
		askToKeep(o);
	}

	function choose(o: Output, what: 'size' | 'rate' | 'turn' | 'scale', i: number): void {
		const listed = sizes(o);
		let [w, h] = [o.mode.width, o.mode.height];
		let refresh = o.mode.refresh;
		let { scale, transform } = o;
		if (what === 'size') {
			[w, h] = listed[i];
			const r = rates(o, w, h);
			refresh = r.includes(refresh) ? refresh : r[0];
		} else if (what === 'rate') refresh = rates(o, w, h)[i];
		else if (what === 'turn') transform = ORIENTATIONS[i][0];
		else scale = SCALES[i];
		void apply(o, { width: w, height: h, refresh }, scale, transform);
	}

	function askToKeep(o: Output): void {
		if (timer) clearInterval(timer);
		left = KEEP_SECONDS;
		question = fill(_('Keep these settings? Reverting in {n} s.'), { n: KEEP_SECONDS });
		timer = setInterval(() => {
			left -= 1;
			if (left <= 0) void goBack();
			else question = fill(_('Keep these settings? Reverting in {n} s.'), { n: left });
		}, 1000);
	}

	async function goBack(): Promise<void> {
		if (timer) clearInterval(timer);
		timer = null;
		question = '';
		if (!before) return;
		const b = before;
		before = null;
		await post('/api/display', { name: b.out.name, mode: b.mode, scale: b.scale, transform: b.transform }).catch(() => {});
		b.out.mode = b.mode;
		b.out.scale = b.scale;
		b.out.transform = b.transform;
	}

	async function kept(): Promise<void> {
		if (timer) clearInterval(timer);
		timer = null;
		question = '';
		const o = before?.out;
		before = null;
		if (!o) return;
		try {
			await ask('display-set', { output: o.name, size: `${o.mode.width}x${o.mode.height}`, refresh: o.mode.refresh, scale: o.scale, transform: o.transform });
			saved = _('Saved. The screen starts this way from now on.');
		} catch (e) {
			saved = (e as Error).message;
		}
	}

	onDestroy(() => {
		if (timer) void goBack();
	});
	void load();
</script>

<div class="settings">
	<Group title={_("When No One's Using It")} icon="lock-screen" note={_('For everyone at this screen. The box keeps running.')}>
		{#each [['lock_after', _('Lock the screen after'), _("An admin's password brings the desktop back."), LOCK_AFTER, 'lock-screen'],
			['screen_off_after', _('Turn the screen off after'), _('A key, the mouse or a touch turns it on.'), SCREEN_OFF_AFTER, 'monitor']] as [key, title, words, choices, icon] (key)}
			<Row big icon={icon as string} title={title as string} sub={words as string}>
				{#snippet ends()}
					<Dropdown names={(choices as number[]).map(minutes)} label={title as string}
						selected={(choices as number[]).indexOf(desk.box[key as 'lock_after'])}
						changed={(i) => desk.patchBox({ [key as string]: (choices as number[])[i] }).catch(() => {})} />
				{/snippet}
			</Row>
		{/each}
	</Group>

	{#if loaded && !outputs.length}
		<Group title={_('Display')} icon="monitor">
			<Row title={_('No screen to set here')} sub={_('For a screen plugged into the box.')} />
		</Group>
	{/if}
	{#if question}
		<div class="keep-bar">
			<p>{question}</p>
			<button onclick={goBack}>{_('Go back')}</button>
			<button class="primary" onclick={kept}>{_('Keep')}</button>
		</div>
	{/if}
	{#each outputs as o (o.name)}
		{@const listed = sizes(o)}
		{@const shownRates = rates(o, o.mode.width, o.mode.height)}
		<Group title={o.title === o.name ? o.title : `${o.title} (${o.name})`} icon="monitor">
			<Row big icon="rotate" title={_('Orientation')} sub={_('Landscape or portrait.')}>
				{#snippet ends()}
					<Dropdown names={ORIENTATIONS.map(([, n]) => _(n))} label={_('Orientation')}
						selected={Math.max(0, ORIENTATIONS.findIndex(([k]) => k === o.transform))} changed={(i) => choose(o, 'turn', i)} />
				{/snippet}
			</Row>
			<Row big icon="resolution" title={_('Resolution')} sub={_('Higher is sharper, with smaller items.')}>
				{#snippet ends()}
					<Dropdown names={listed.map(([w, h]) => `${w} × ${h}` + (ratio(w, h) ? ` (${ratio(w, h)})` : ''))} label={_('Resolution')}
						selected={listed.findIndex(([w, h]) => w === o.mode.width && h === o.mode.height)} changed={(i) => choose(o, 'size', i)} />
				{/snippet}
			</Row>
			<Row big icon="rate" title={_('Screen Update Rate')} sub={_('How smoothly things move on the screen.')}>
				{#snippet ends()}
					<Dropdown names={shownRates.map((r) => `${(r / 1000).toFixed(2)} Hz`)} label={_('Screen Update Rate')}
						selected={shownRates.indexOf(o.mode.refresh)} changed={(i) => choose(o, 'rate', i)} />
				{/snippet}
			</Row>
			<Row big icon="text-size" title={_('Text & UI Scaling')} sub={_('Size of text and everything on screen.')}>
				{#snippet ends()}
					{@const nearest = SCALES.reduce((best, s, i) => (Math.abs(s - o.scale) < Math.abs(SCALES[best] - o.scale) ? i : best), 0)}
					<Dropdown names={SCALES.map((s) => `${(s * 100).toFixed(0)} %`)} label={_('Text & UI Scaling')} selected={nearest}
						changed={(i) => choose(o, 'scale', i)} />
				{/snippet}
			</Row>
		</Group>
	{/each}
	{#if saved}<span class="group-note">{saved}</span>{/if}
</div>
