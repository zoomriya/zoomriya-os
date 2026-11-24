<script lang="ts">
	// The box's background, one for everyone: the sign-in screen, the desktop
	// and the lock screen all show it, and change together.
	import { desk } from '../lib/desk.svelte';
	import { backgroundStyle } from '../lib/look';
</script>

<div class="backdrop" style={backgroundStyle(desk.box.background, desk.hasPicture, desk.pictureStamp)}></div>
