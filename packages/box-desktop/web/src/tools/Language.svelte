<script lang="ts">
	// Control Panel › Language & Region (control.py's cp_language): the box's
	// language, its country or region, and its state or province. A new
	// language changes every word on the page at once.
	import { ask, get } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, lang } from '../lib/i18n.svelte';
	import Dropdown from '../components/Dropdown.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	interface Choice { code: string; name: string }
	let langs = $state<Choice[]>([]);
	let places = $state<Choice[]>([]);
	let subs = $state<Choice[]>([]);
	let langPick = $state(-1);
	let regionPick = $state(-1);
	let subPick = $state(0);
	let done = $state('');

	async function showSubdivisions(region: string, chosen = ''): Promise<void> {
		try {
			subs = await get<Choice[]>(`/api/places/subdivisions?region=${encodeURIComponent(region)}&lang=${lang()}`);
		} catch {
			subs = [];
		}
		subPick = Math.max(0, subs.findIndex((s) => s.code === chosen) + 1);
	}

	async function load(): Promise<void> {
		const [listed, status] = await Promise.all([
			get<{ languages: Choice[]; regions: Choice[] }>(`/api/places?lang=${lang()}`), ask('setup-status')
		]);
		langs = listed.languages;
		places = listed.regions;
		const code = String(status.language ?? '').split('_')[0];
		langPick = langs.findIndex((l) => l.code === code);
		regionPick = places.findIndex((p) => p.code === status.region);
		if (regionPick >= 0) await showSubdivisions(status.region, status.subdivision ?? '');
	}

	async function save(): Promise<void> {
		if (langPick < 0 || regionPick < 0) return;
		const language = langs[langPick].code;
		const region = places[regionPick].code;
		const sub = subPick > 0 ? subs[subPick - 1]?.code ?? '' : '';
		try {
			const { locale } = await get<{ locale: string }>(`/api/places/locale?language=${language}&region=${region}`);
			await ask('region-set', { region, subdivision: sub });
			await ask('language-set', { locale });
			const before = lang();
			const session = await get<{ language: string }>('/api/session');
			if (session.language !== before) {
				// A new language: the desktop in it, at once; the countries' names too.
				await desk.languageChanged(session.language);
				await load();
			} else {
				done = _('Saved.');
			}
		} catch (e) {
			done = (e as Error).message;
		}
	}

	void load().catch((e) => (done = (e as Error).message));
</script>

<div class="settings">
	<Group title={_('Language & Region')} icon="globe">
		<Row big icon="globe" title={_('Language')} sub={_("For the box's screens and messages.")}>
			{#snippet ends()}<Dropdown names={langs.map((l) => l.name)} bind:selected={langPick} changed={save} label={_('Language')} />{/snippet}
		</Row>
		<Row big icon="pin" title={_('Country or Region')} sub={_('Sets date, time and number formats.')}>
			{#snippet ends()}
				<Dropdown names={places.map((p) => p.name)} bind:selected={regionPick} label={_('Country or Region')}
					changed={async (i) => { await showSubdivisions(places[i].code); await save(); }} />
			{/snippet}
		</Row>
		{#if subs.length}
			<Row big icon="pin" title={_('State or Province')} sub={_('Optional.')}>
				{#snippet ends()}
					<Dropdown names={[_('Not chosen'), ...subs.map((s) => s.name)]} bind:selected={subPick} changed={save} label={_('State or Province')} />
				{/snippet}
			</Row>
		{/if}
	</Group>
	{#if done}<span class="group-note">{done}</span>{/if}
</div>
