<script lang="ts">
	// Control Panel › Appearance (control.py's cp_appearance): Daylight or
	// Midnight, each shown as a small desktop; the accent colour; and the
	// box's background, ours, a plain colour, or the family's own picture
	// from a USB stick. Each shows at once and is kept.
	import { desk } from '../lib/desk.svelte';
	import { _ } from '../lib/i18n.svelte';
	import { backgroundStyle } from '../lib/look';
	import { ACCENTS, COLOURS, OWN, PICTURES } from '../lib/words';
	import Group from '../components/Group.svelte';
	import Icon from '../components/Icon.svelte';
	import Wide from '../components/Wide.svelte';
	import AddPicture from './AddPicture.svelte';

	let { shown = true }: { shown?: boolean } = $props();
	const scene = $derived(backgroundStyle(desk.box.background, desk.hasPicture, desk.pictureStamp));
	const backgrounds = $derived([
		...(desk.hasPicture ? [[OWN, _('Your picture')]] : []),
		...Object.entries(PICTURES).map(([k, [n]]) => [k, _(n)]),
		...Object.entries(COLOURS).map(([k, [n]]) => [k, _(n)])
	] as [string, string][]);
</script>

<div class="settings">
	<Group title={_('Style')} icon="contrast">
		<Wide>
			<div class="look-cards" role="radiogroup" aria-label={_('Style')}>
				{#each [['daylight', _('Daylight')], ['midnight', _('Midnight')]] as [theme, name] (theme)}
					<button class="look-card" class:on={desk.theme === theme} role="radio" aria-checked={desk.theme === theme} aria-label={name}
						onclick={() => desk.setTheme(theme as 'daylight' | 'midnight')}>
						<span class="look-scene" style={scene}>
							<span class="mini-window {theme} behind" style:left="70px" style:top="20px"><span class="mini-bar" style:display="block"></span></span>
							<span class="mini-window {theme}" style:left="40px" style:top="46px"><span class="mini-bar" style:display="block"></span></span>
						</span>
						<span>{name}</span>
					</button>
				{/each}
			</div>
		</Wide>
	</Group>

	<Group title={_('Accent Colour')} icon="palette" lead={_("For buttons, switches and what's selected.")}>
		<Wide>
			<div class="swatches" role="radiogroup" aria-label={_('Accent Colour')}>
				{#each Object.entries(ACCENTS) as [key, [english, day, , night]] (key)}
					<button class="swatch" class:on={desk.accent === key} role="radio" aria-checked={desk.accent === key}
						title={_(english)} aria-label={_(english)} style:background={desk.theme === 'midnight' ? night : day}
						onclick={() => desk.changeLook(key)}>
						<Icon name="check" px={16} role="white" />
					</button>
				{/each}
			</div>
		</Wide>
	</Group>

	<Group title={_('Background')} icon="picture" note={_('The box\'s, for everyone: the sign-in screen, the desktop and the lock screen.')}>
		{#snippet aside()}
			<button class="flat-link with-icon" onclick={() => desk.sheet(AddPicture)}><Icon name="plus" px={16} role="accent" />{_('Add Picture')}</button>
		{/snippet}
		<Wide>
			<div class="backgrounds" role="radiogroup" aria-label={_('Background')}>
				{#each backgrounds as [key, name] (key)}
					<button class="background-tile" class:on={desk.box.background === key} role="radio" aria-checked={desk.box.background === key}
						title={name} aria-label={name} onclick={() => desk.changeBackground(key)}>
						<span class="thumb" style={backgroundStyle(key, desk.hasPicture, desk.pictureStamp)}></span>
					</button>
				{/each}
			</div>
		</Wide>
	</Group>
</div>
