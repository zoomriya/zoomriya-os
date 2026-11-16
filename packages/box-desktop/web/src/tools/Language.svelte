<script lang="ts">
	// Control Panel › Language & Region (control.py's cp_language): the box's
	// language, its country or region, its state or province, and the town
	// for the box's weather (weather.md). A new language changes every word on
	// the page at once; a new country forgets the old one's town.
	import { ask, get } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill, lang } from '../lib/i18n.svelte';
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
	interface Town { id: number; name: string; admin1: string }
	let town = $state<Town | null>(null);
	let townSearch = $state('');
	let towns = $state<Town[]>([]);
	const townName = (t: Town) => [t.name, t.admin1].filter(Boolean).join(', ');

	let townTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		const typed = townSearch.trim();
		const region = places[regionPick]?.code;
		if (townTimer) clearTimeout(townTimer);
		if (!typed || !region) {
			towns = [];
			return;
		}
		townTimer = setTimeout(async () => {
			towns = await get<Town[]>(`/api/places/towns?region=${region}&q=${encodeURIComponent(typed)}`).catch(() => []);
		}, 150);
	});

	async function chooseTown(t: Town): Promise<void> {
		try {
			await ask('town-set', { town: t.id });
			town = t;
			townSearch = '';
			done = _('Saved.');
		} catch (e) {
			done = (e as Error).message;
		}
	}

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
		town = status.town ? { id: status.town.id, name: status.town.name, admin1: status.town.admin1 ?? '' } : null;
	}

	async function save(): Promise<void> {
		if (langPick < 0 || regionPick < 0) return;
		const language = langs[langPick].code;
		const region = places[regionPick].code;
		const sub = subPick > 0 ? subs[subPick - 1]?.code ?? '' : '';
		try {
			const { locale } = await get<{ locale: string }>(`/api/places/locale?language=${language}&region=${region}`);
			await ask('region-set', { region, subdivision: sub });
			// The agent forgets a town in another country.
			const status = await ask('setup-status');
			town = status.town ? { id: status.town.id, name: status.town.name, admin1: status.town.admin1 ?? '' } : null;
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
		<Row big icon="pin" title={_('Town')} sub={town ? fill(_('{town}. For the weather on the box.'), { town: townName(town) }) : _('Not chosen: weather is off until a town is chosen.')}>
			{#snippet ends()}
				<input type="search" bind:value={townSearch} placeholder={_('Search, for example Toronto')} aria-label={_('Town')} />
			{/snippet}
		</Row>
		{#if towns.length}
			<div class="towns" role="listbox" aria-label={_('Town')}>
				{#each towns as t (t.id)}
					<button role="option" aria-selected={town?.id === t.id} onclick={() => chooseTown(t)}>{townName(t)}</button>
				{/each}
				<span class="credit">{_('Place names from GeoNames (CC BY 4.0), shortened.')}</span>
			</div>
		{/if}
	</Group>
	{#if done}<span class="group-note">{done}</span>{/if}
</div>

<style>
	.towns { display: flex; flex-direction: column; max-height: 240px; overflow-y: auto; padding: 4px 12px 12px; }
	.credit { font-size: 11px; opacity: 0.7; padding: 6px 10px 0; }
	.towns button { text-align: left; background: none; border: 0; padding: 8px 10px; border-radius: 8px; color: inherit; cursor: pointer; }
	.towns button:hover, .towns button:focus-visible { background: var(--hover, rgba(127, 127, 127, 0.15)); }
</style>
