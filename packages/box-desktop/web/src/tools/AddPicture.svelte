<script lang="ts">
	// Add a picture (control.py's add_picture): the pictures on the USB sticks
	// plugged in, as the agent found and previewed them; the one chosen
	// becomes the box's background. What's kept is the agent's own JPEG made
	// from it, never the file on the stick.
	import { ask, post, type Answer } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import Sheet from '../components/Sheet.svelte';
	import Spinner from '../components/Spinner.svelte';

	let { close }: { close: () => void } = $props();
	let busy = $state(false);
	let words = $state('');
	let problem = $state(false);
	let found = $state<Answer[]>([]);

	function working(text: string): void {
		busy = true;
		words = text;
		problem = false;
	}

	function idle(text: string, isProblem = false): void {
		busy = false;
		words = text;
		problem = isProblem;
	}

	async function look(): Promise<void> {
		working(_('Looking on the USB sticks…'));
		try {
			const answer = await ask('pictures', { action: 'usb' });
			found = answer.found ?? [];
			idle(found.length ? _('Choose one for your background.') : _('No pictures found. Plug in a USB stick with JPEG or PNG pictures.'));
		} catch (e) {
			idle((e as Error).message, true);
		}
	}

	async function chosen(item: Answer): Promise<void> {
		working(_('Making it your background…'));
		try {
			const answer = await post<{ box: typeof desk.box }>('/api/box/picture', { device: item.device, name: item.file });
			desk.hasPicture = true;
			desk.pictureStamp = Date.now();
			desk.box = answer.box;
			close();
		} catch (e) {
			idle((e as Error).message, true);
		}
	}

	void look();
</script>

<Sheet title={_('Add a picture')} width={720} {close}>
	<p class="lead">{_('JPEG or PNG pictures on a USB stick, in its top folder or a folder in it. Nothing on the stick is changed.')}</p>
	<div class="row-between">
		{#if busy}<Spinner />{/if}
		<span class="row-sub grow" class:error={problem}>{words}</span>
		<button onclick={look} disabled={busy}>{_('Look again')}</button>
	</div>
	<div class="backgrounds" style:max-height="420px" style:overflow-y="auto" style:justify-content="flex-start">
		{#each found as item (item.device + item.file)}
			<button class="background-tile found" disabled={busy} title="{item.file} · {item.width} × {item.height}" onclick={() => chosen(item)}>
				<span class="thumb" style:background-image="url(data:image/jpeg;base64,{item.thumb})"></span>
				<span class="found-name">{String(item.file ?? '').split('/').pop()}</span>
			</button>
		{/each}
	</div>
</Sheet>
