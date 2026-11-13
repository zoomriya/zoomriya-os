<script lang="ts">
	import { _, fill } from '../lib/i18n.svelte';
	import QR from '../components/QR.svelte';
	import Sheet from '../components/Sheet.svelte';

	let { name, address, close }: { name: string; address: string; close: () => void } = $props();
</script>

<Sheet title={fill(_('Open {app} on a phone or computer'), { app: name })} width={460} {close}>
	<p class="lead">{_('Scan the code with a phone\'s camera, or type the address in a browser on the home network.')}</p>
	<QR text={address} />
	<span class="app-address selectable">{address}</span>
</Sheet>
