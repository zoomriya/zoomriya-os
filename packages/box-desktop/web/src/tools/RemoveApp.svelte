<script lang="ts">
	// Remove an app (apps.py's remove_app): keep its data (the default), or
	// delete it too; then the password.
	import { ask, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import Sheet from '../components/Sheet.svelte';

	let { app, done, close }: { app: Answer; done: () => void; close: () => void } = $props();
	let withData = $state(false);

	function ahead(): void {
		close();
		const include = withData ? ['data'] : undefined;
		desk.confirm({
			title: fill(_('Remove {app}?'), { app: app.name ?? '' }), text: withData ? _('Its data is deleted too.') : _('Its data is kept.'),
			button: _('Remove'), action: (password) => ask('apps', { action: 'remove', name: app.id, include }, password), done
		});
	}
</script>

<Sheet title={fill(_('Remove {app}?'), { app: app.name ?? '' })} {close}>
	<p class="lead">{fill(_('{app} stops and is removed.'), { app: app.name ?? '' })}</p>
	<label class="check"><input type="radio" name="data" checked={!withData} onchange={() => (withData = false)} />{_('Keep its data')}</label>
	<label class="check"><input type="radio" name="data" checked={withData} onchange={() => (withData = true)} />{_("Delete its data too (can't be undone)")}</label>
	<div class="row-end"><button class="danger" onclick={ahead}>{_('Remove')}</button></div>
</Sheet>
