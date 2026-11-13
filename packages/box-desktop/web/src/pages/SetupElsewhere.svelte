<script lang="ts">
	// A browser on the home network, before the box is set up: the first setup
	// is at the box's own screen only (design §5).
	import { _ } from '../lib/i18n.svelte';
	import Logo from '../components/Logo.svelte';
	import Backdrop from './Backdrop.svelte';
</script>

<Backdrop />
<div class="signin-place">
	<div class="glass-card signin-card" style:text-align="center">
		<div style:align-self="center"><Logo height={22} /></div>
		<h1 class="title">{_('Set up the box at its own screen')}</h1>
		<p class="lead">{_('Plug a screen and a keyboard into the box and follow the steps there. Box Desktop opens here once the box is set up.')}</p>
		<div class="row-end" style:justify-content="center"><button onclick={() => location.reload()}>{_('Try again')}</button></div>
	</div>
</div>
