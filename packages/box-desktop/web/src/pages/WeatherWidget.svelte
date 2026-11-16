<script lang="ts">
	// The town's weather (weather.md §7): now, today and the next two days,
	// any active warning first, the credit and when it was fetched. Read from
	// what zoomriya-weather keeps; Box Desktop never asks a weather service.
	import { desk } from '../lib/desk.svelte';
	import { clockTime } from '../lib/format';
	import { _, fill, lang } from '../lib/i18n.svelte';
	import Badge from '../components/Badge.svelte';

	interface Words { en: string; fr: string }
	interface Day { date: string; name: Words; high: number | null; low: number | null; condition: Words; icon: string }
	interface Alert { id: string; kind: string; event: Words; ends?: string }

	const w = $derived(desk.weather);
	const state = $derived(String(w.state ?? 'none'));
	const say = (words: Words | undefined) => (words ? (lang() === 'fr' ? words.fr : words.en) || words.en : '');
	const degrees = (v: number | null | undefined) => (v === null || v === undefined ? '–' : `${Math.round(v)}°`);
	const days = $derived(((w.days ?? []) as Day[]).slice(0, 3));
	const alerts = $derived(((w.alerts ?? []) as Alert[]).filter((a) => a.kind === 'warning' || a.kind === 'watch' || a.kind === 'advisory'));
	const fetched = $derived(w.fetched ? clockTime(new Date(w.fetched)) : '');
	const icon = $derived(String(w.now?.icon ?? days[0]?.icon ?? 'cloudy'));
	const town = $derived(String(w.town?.name ?? ''));
</script>

<section class="widget widget-sky" aria-label={_('Weather')}>
	<div class="widget-head">
		<Badge name={state === 'none' ? 'weather-partly' : `weather-${icon}`} px={20} shade="blue" />
		<span class="widget-title">{_('Weather')}</span>
		<span class="widget-aside">{town}</span>
	</div>
	{#if state === 'none'}
		<button class="widget-small link" onclick={() => desk.openTool('language')}>
			{_('Choose your town in Control Panel › Language & Region to see the weather.')}
		</button>
	{:else if state === 'waiting'}
		<span class="widget-small">{w.problem ? _("The weather service isn't answering yet.") : _('Getting the weather…')}</span>
	{:else}
		{#if alerts.length}
			{#each alerts as a (a.id)}
				<span class="widget-value warn alert">⚠ {say(a.event)}</span>
			{/each}
			<span class="widget-small">{fill(_('For official warnings, see {where}.'), { where: say(w.official_name) })}</span>
		{/if}
		<div style:display="flex" style:gap="12px" style:align-items="center">
			<span class="widget-figure">{degrees(w.now?.temp ?? days[0]?.high)}</span>
			<div style:display="flex" style:flex-direction="column">
				<span class="widget-big">{say(w.now?.condition ?? days[0]?.condition)}</span>
				{#if days[0]}
					<span class="widget-small">{fill(_('High {high} · Low {low}'), { high: degrees(days[0].high), low: degrees(days[0].low) })}</span>
				{/if}
			</div>
		</div>
		<div class="widget-rows">
			{#each days.slice(1) as d (d.date)}
				<span class="widget-small">{say(d.name)} · {say(d.condition)}</span>
				<span class="widget-value">{degrees(d.high)} / {degrees(d.low)}</span>
			{/each}
		</div>
		{#if !w.alerts_available}
			<span class="widget-small">{_("Weather alerts aren't available for your country yet.")}</span>
		{/if}
		<span class="widget-small" class:warn={state !== 'ok'}>
			{state === 'off' ? _('Weather is off.') + ' ' : ''}{fetched ? fill(_('Last updated {time}'), { time: fetched }) : ''}
		</span>
		<span class="widget-small credit">{say(w.credit)} {_('Place names from GeoNames (CC BY 4.0), shortened.')}</span>
	{/if}
</section>

<style>
	.link { background: none; border: 0; padding: 0; text-align: left; color: inherit; cursor: pointer; text-decoration: underline; }
	.alert { text-align: left; }
	.credit { font-size: 10px; opacity: 0.8; }
	.warn { color: var(--warn, #c2410c); }
</style>
