<script lang="ts">
	// App Center › Add from USB (apps.py's apps_usb): the .zapp files on the
	// sticks plugged in; only ZOOMRIYA-signed apps install.
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { size } from '../lib/format';
	import { _, fill } from '../lib/i18n.svelte';
	import Group from '../components/Group.svelte';
	import Row from '../components/Row.svelte';
	import Spinner from '../components/Spinner.svelte';
	import { appsReload } from './apps';

	let { shown = true }: { shown?: boolean } = $props();
	let busy = $state(false);
	let found = $state<Answer[] | null>(null);
	let problem = $state('');

	async function look(): Promise<void> {
		busy = true;
		problem = '';
		try {
			found = (await ask('apps', { action: 'usb' })).found ?? [];
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	function install(item: Answer): void {
		const name = `${item.name ?? ''} ${item.version ?? ''}`;
		desk.confirm({
			title: fill(_('Install {app}?'), { app: name }), text: _("It's checked, then started. This can take a few minutes."),
			button: _('Install'), action: (password) => ask('apps', { action: 'install', device: item.device, name: item.file }, password),
			done: () => {
				desk.openTool('apps', 'installed');
				appsReload.now?.();
			}
		});
	}

	const where = (item: Answer) => [size(item.size_bytes ?? 0), fill(_('On {stick}'), { stick: item.stick || _('the USB stick') })].join(' · ');
	function why(item: Answer): string {
		if (item.kind === 'older') return fill(_('Older than {version}, which this box has had.'), { version: item.newest ?? '' });
		if (item.kind === 'blocked') return _('Has a known problem. Use a newer version.');
		return _(item.problem ?? '');
	}
	function tag(item: Answer): string {
		return ({ new: _('new'), update: fill(_('update from {version}'), { version: item.installed ?? '' }), same: _('installed') } as Answer)[item.kind] ?? '';
	}
</script>

<div class="settings">
	<Group title={_('Add from a USB stick')} icon="disk" lead={_('Put the .zapp file in zoomriya/apps on the stick. Only ZOOMRIYA apps install.')}>
		<Row big icon="disk" title={_('Apps on USB sticks')} sub={_('Nothing on the stick is changed.')}>
			{#snippet ends()}{#if busy}<Spinner />{/if}<button class="primary" disabled={busy} onclick={look}>{_('Look on USB stick')}</button>{/snippet}
		</Row>
	</Group>
	{#if problem}<p class="error">{problem}</p>{/if}
	{#if found}
		<Group title={_('Found')}>
			{#if !found.length}<Row title={_('No apps found')} sub={_('Plug in a stick with a zoomriya/apps folder.')} />{/if}
			{#each found as item, i (i)}
				{#if !item.ok}
					<Row icon="alert" role="warn" title={item.name ? `${item.name} ${item.version ?? ''}` : item.file ?? ''} sub={why(item) + '\n' + where(item)} />
				{:else}
					<Row icon="app" title={`${item.name ?? ''} ${item.version ?? ''}`} sub={where(item)} tag={tag(item)}>
						{#snippet ends()}
							{#if item.kind !== 'same'}<button class="primary" onclick={() => install(item)}>{item.kind === 'update' ? _('Update') : _('Install')}</button>{/if}
						{/snippet}
					</Row>
				{/if}
			{/each}
		</Group>
	{/if}
</div>
