<script lang="ts">
	// A QR code of text, dark on white with its quiet margin, for a phone's camera.
	import qrcode from 'qrcode-generator';

	let { text, size = 220 }: { text: string; size?: number } = $props();
	const svg = $derived.by(() => {
		const code = qrcode(0, 'M');
		code.addData(text);
		code.make();
		const count = code.getModuleCount();
		let cells = '';
		for (let y = 0; y < count; y++)
			for (let x = 0; x < count; x++)
				if (code.isDark(y, x)) cells += `M${x + 4} ${y + 4}h1.02v1.02h-1.02z`;
		// Four modules of white round it.
		return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${count + 8} ${count + 8}" ` +
			`shape-rendering="crispEdges"><rect width="100%" height="100%" fill="#fff"/><path d="${cells}" fill="#0a0f1a"/></svg>`;
	});
</script>

<span class="qr" style:width="{size}px" style:height="{size}px">{@html svg}</span>
