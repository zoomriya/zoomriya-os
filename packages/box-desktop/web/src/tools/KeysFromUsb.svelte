<script lang="ts">
	// The keys on the USB sticks plugged in; none is added until the admin says so.
	import { ask, type Answer } from '../lib/api';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Sheet from '../components/Sheet.svelte';
	import Spinner from '../components/Spinner.svelte';
	import { addKey } from './keys';

	let { done, close }: { done: () => void; close: () => void } = $props();
	let found = $state<Answer[] | null>(null);
	let problem = $state('');
	ask('advanced-access', { action: 'usb-keys' }).then((a) => (found = a.found ?? [])).catch((e) => (problem = e.message));
</script>

<Sheet title={_('Keys on USB sticks')} {close}>
	{#if problem}<p class="error">{problem}</p>
	{:else if !found}<Spinner />
	{:else if !found.length}<p class="lead">{_('No keys found in zoomriya/ssh on the stick.')}</p>
	{:else}
		<Group>
			{#each found as key (key.fingerprint)}
				<Row icon="key" title={key.name ?? ''} sub={`${key.type ?? ''} · ${key.fingerprint ?? ''}\n${fill(_('On {stick}'), { stick: key.stick || _('the USB stick') })}`}>
					{#snippet ends()}<button class="primary" onclick={() => { close(); addKey(key.key, key.name ?? '', done); }}>{_('Add')}</button>{/snippet}
				</Row>
			{/each}
		</Group>
	{/if}
</Sheet>
