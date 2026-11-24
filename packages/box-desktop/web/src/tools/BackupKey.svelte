<script lang="ts">
	// The backup key, to print or write down (Storage Manager › Backups): it
	// opens every backup on the drive, on this box or a new one.
	import { _ } from '../lib/i18n.svelte';
	import Sheet from '../components/Sheet.svelte';

	let { key, close }: { key: string; close: () => void } = $props();

	function print(): void {
		const page = window.open('', '_blank', 'width=640,height=480');
		if (!page) return;
		const title = _('ZOOMRIYA backup key');
		const words = _('Keep this with your important papers, away from the backup drive. It opens the backups: with it, a new box brings everything back.');
		page.document.title = title;
		const body = page.document.body;
		body.style.fontFamily = 'sans-serif';
		body.style.padding = '48px';
		for (const [tag, text, css] of [
			['h1', title, 'font-size:22px'],
			['p', key, 'font:bold 28px monospace;letter-spacing:0.08em;margin:32px 0'],
			['p', words, 'font-size:14px;max-width:440px'],
			['p', new Date().toLocaleDateString(), 'font-size:12px;color:#555']
		]) {
			const el = page.document.createElement(tag);
			el.textContent = text;
			el.setAttribute('style', css);
			body.appendChild(el);
		}
		page.focus();
		page.print();
	}
</script>

<Sheet title={_('Your backup key')} {close}>
	<p class="lead">{_('Print it or write it down, and keep it away from the drive. Bringing a backup back always needs it. The box keeps it too, so you can show it again here.')}</p>
	<p class="mono" style:font-size="26px" style:text-align="center" style:margin="24px 0" style:user-select="all">{key}</p>
	<div class="row-end">
		<button onclick={print}>{_('Print')}</button>
		<button class="primary" onclick={close}>{_("I've kept it")}</button>
	</div>
</Sheet>
