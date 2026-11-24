<script lang="ts">
	// Internet access (control.py's show_internet): Apps offline, Everything
	// offline, each connection an app declares (what leaves, why, for which
	// feature) allowed or blocked, and what the firewall stopped lately.
	import { ask, type Answer } from '../lib/api';
	import { when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Switch from '../components/Switch.svelte';

	let answer = $state<Answer | null>(null);
	let problem = $state('');
	const allOff = $derived(!!answer?.all_offline);
	const appsOff = $derived(!!answer?.apps_offline);

	async function load(): Promise<void> {
		try {
			answer = await ask('internet');
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	async function change(fields: Record<string, unknown>): Promise<void> {
		try {
			answer = await ask('internet-set', fields);
		} catch {
			await load();
		}
	}

	void load();
</script>

{#if problem}<p class="error">{problem}</p>{/if}
{#if answer}
	<Group title={_('Internet Access')} lead={_('What may leave the box. Apps are blocked unless they say what they send and why.')} icon="globe">
		<Row big icon="app" title={_('Apps offline')}
			sub={_('Every app is cut off from the internet. The box still gets its security fixes and the time.')}>
			{#snippet ends()}<Switch on={appsOff} disabled={allOff} changed={(on) => change({ action: 'apps-offline', on })} label={_('Apps offline')} />{/snippet}
		</Row>
		<Row big icon="globe" title={_('Everything offline')}
			sub={_('Nothing leaves your home network, not even security fixes. Turn it off again soon so the box can catch up.')}>
			{#snippet ends()}<Switch on={allOff} changed={(on) => change({ action: 'all-offline', on })} label={_('Everything offline')} />{/snippet}
		</Row>
		{#each answer.declared ?? [] as d (d.id)}
			<Row icon="globe" iconPx={26} title={d.name ?? ''}
				sub={[d.feature ? fill(_('For: {feature}'), { feature: d.feature }) : '', fill(_('What leaves: {what}'), { what: d.what ?? '' }),
					d.why ? fill(_('Why: {why}'), { why: d.why }) : '', d.when ? fill(_('When: {when}'), { when: d.when }) : ''].filter(Boolean).join('\n')}>
				{#snippet ends()}
					<span class="row-value">{d.allowed ? _('Allowed') : fill(_('Blocked: {why}'), { why: _(d.blocked ?? '') })}</span>
					{#if d.optional}
						<Switch on={!d.off} disabled={allOff || appsOff} changed={(on) => change({ action: on ? 'allow' : 'stop', name: d.id })} label={d.name} />
					{/if}
				{/snippet}
			</Row>
		{/each}
		{#if !(answer.declared ?? []).length}
			<Row title={_('No app reaches the internet')} sub={_('Every app is blocked: none has said it needs to send anything.')} />
		{/if}
	</Group>
	<Group title={fill(_('Blocked in the Last {days} Days'), { days: answer.days ?? 7 })} lead={_('What the box stopped from leaving, by the app that tried.')} icon="globe">
		{#each answer.blocked ?? [] as c, i (i)}
			{@const times = c.count ?? 0}
			<Row icon="globe" iconPx={26} title={c.who || c.account || ''}
				sub={`${c.destination ?? ''} · ${String(c.protocol ?? '').toUpperCase()} ${c.port ?? ''}\n${times === 1 ? _('Once') : fill(_('{count} times'), { count: times })} · ${c.last ? when(c.last) : ''}`} />
		{/each}
		{#if !(answer.blocked ?? []).length}<Row title={_('Nothing')} sub={_('Nothing was stopped.')} />{/if}
	</Group>
{/if}
