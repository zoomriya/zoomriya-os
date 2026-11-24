<script lang="ts">
	// Diagnostics (support.py's diagnostics, in Support Center and Log Center):
	// make a bundle for support, review it, save it to a USB stick.
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { size, when } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { findings, SUPPORT_URL } from '../lib/support';
	import Group from '../components/Group.svelte';
	import Icon from '../components/Icon.svelte';
	import Row from '../components/Row.svelte';
	import Spinner from '../components/Spinner.svelte';
	import Switch from '../components/Switch.svelte';
	import Wide from '../components/Wide.svelte';
	import ReviewBundle from './ReviewBundle.svelte';
	import SaveBundle from './SaveBundle.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	let note = $state('');
	let journal = $state(true);
	let network = $state(true);
	let busy = $state(false);
	let answer = $state<Answer | null>(null);
	let problem = $state('');

	async function run(fields: Answer, making = false): Promise<void> {
		if (making) busy = true;
		try {
			answer = await ask('diagnostics', fields);
			problem = '';
			if (making) note = '';
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	function make(): void {
		const include = [journal && 'journal', network && 'network'].filter(Boolean);
		void run({ action: 'make', include, note: note.trim() || undefined }, true);
	}

	void run({});
</script>

<div class="settings">
	<Group title={_('Make a bundle')} icon="bundle" lead={_('Settings, versions and logs, for support. No photos, files or messages.')}>
		<Wide>
			<span class="row-title">{_("What's wrong?")}</span>
			<input type="text" maxlength="2000" bind:value={note} placeholder={_('For example: photos won\'t upload since Tuesday')} aria-label={_("What's wrong?")} />
		</Wide>
		<Row icon="logs" title={_('Include the system log')} sub={_('The last week. Support needs it for most problems.')}>
			{#snippet ends()}<Switch bind:on={journal} label={_('Include the system log')} />{/snippet}
		</Row>
		<Row icon="router" title={_('Include network details')} sub={_('Addresses, router and name servers.')}>
			{#snippet ends()}<Switch bind:on={network} label={_('Include network details')} />{/snippet}
		</Row>
		<Row big icon="bundle" title={_('New bundle')} sub={_('Takes a minute.')}>
			{#snippet ends()}{#if busy}<Spinner />{/if}<button class="primary" disabled={busy} onclick={make}>{_('Make bundle')}</button>{/snippet}
		</Row>
	</Group>
	<Group title={_('Bundles on the box')} icon="layers" note={_('The newest five are kept.')}>
		{#each answer?.bundles ?? [] as bundle (bundle.name)}
			<Row icon="bundle" title={when(bundle.made ?? '')} sub={`${findings(bundle)} · ${size(bundle.size_bytes ?? 0)}`}
				tag={bundle.name === answer?.made ? _('new') : ''}>
				{#snippet ends()}
					<button onclick={() => desk.sheet(ReviewBundle, { name: bundle.name })}>{_('Review')}</button>
					<button class="primary" onclick={() => desk.sheet(SaveBundle, { name: bundle.name })}>{_('Save to USB')}</button>
					<button title={_('Delete')} aria-label={_('Delete')} onclick={() => run({ action: 'delete', name: bundle.name })}><Icon name="trash" px={16} role="muted" /></button>
				{/snippet}
			</Row>
		{/each}
		{#if answer && !(answer.bundles ?? []).length}<Row title={_('None yet')} />{/if}
	</Group>
	<Group title={_('Send it to support')} icon="lifebuoy">
		{#each [_('Save the bundle to a USB stick.'), fill(_('Upload it at {url}.'), { url: SUPPORT_URL }), _('Give support the code you get.')] as step, n (n)}
			<Row title={step}>{#snippet lead()}<span class="step-number">{n + 1}</span>{/snippet}</Row>
		{/each}
	</Group>
	{#if problem}<p class="error">{problem}</p>{/if}
</div>
