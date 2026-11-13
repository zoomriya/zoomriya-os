<script lang="ts">
	// Save to a USB stick (support.py's save_bundle): the sticks plugged in,
	// and Save on each; the box unmounts it after.
	import { ask, type Answer } from '../lib/api';
	import { size } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import { SUPPORT_URL } from '../lib/support';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Sheet from '../components/Sheet.svelte';
	import Spinner from '../components/Spinner.svelte';

	let { name, close }: { name: string; close: () => void } = $props();
	let sticks = $state<Answer[] | null>(null);
	let busy = $state(false);
	let saved = $state('');
	let problem = $state('');

	async function load(): Promise<void> {
		try {
			sticks = (await ask('diagnostics')).usb ?? [];
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	async function write(stick: Answer): Promise<void> {
		busy = true;
		try {
			await ask('diagnostics', { action: 'export', name, device: stick.device });
			saved = fill(_('Saved to {stick}. You can unplug it.'), { stick: stick.label || _('the USB stick') });
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	void load();
</script>

<Sheet title={_('Save to a USB stick')} {close}>
	{#if busy}<Spinner />
	{:else if saved}
		<p class="lead">{saved}</p>
		<p class="lead">{fill(_('Upload it at {url}.'), { url: SUPPORT_URL })}</p>
	{:else if problem}<p class="error">{problem}</p>
	{:else if sticks && !sticks.length}
		<p class="lead">{_('Plug a USB stick into the box.')}</p>
		<div class="row-end"><button onclick={load}>{_('Try again')}</button></div>
	{:else if sticks}
		<Group>
			{#each sticks as stick (stick.device)}
				<Row icon="disk" title={stick.label || _('USB stick')} sub={`${size(stick.size_bytes ?? 0)} · ${stick.fstype ?? ''}`}>
					{#snippet ends()}<button class="primary" onclick={() => write(stick)}>{_('Save')}</button>{/snippet}
				</Row>
			{/each}
		</Group>
	{/if}
</Sheet>
