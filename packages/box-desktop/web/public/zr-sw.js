// Box Desktop away from home in Firefox, through the ZOOMRIYA extension.
//
// Firefox lets extensions run no service worker of their own, so the box
// serves this one. The extension installs it at home, while pairing, through
// a hidden frame of zr-bridge.html. Away from home the box's address doesn't
// answer, and this worker answers instead: it hands each request to the
// bridge frame, which passes it to the extension, which sends it to the box
// over its encrypted WebRTC connection and checks the answer against the box
// key. Without the extension (no bridge frame), requests go to the network as
// usual, so at home nothing changes.
//
// The messages are zoomriya-connect's: docs/laptops.md, "Firefox".

const VERSION = 1;
const OWN = ['/zr-bridge.html', '/zr-bridge.js'];

self.addEventListener('install', (e) => {
	self.skipWaiting();
	e.waitUntil(caches.open('zr-v' + VERSION).then((c) => c.addAll(OWN)));
});

self.addEventListener('activate', (e) => {
	e.waitUntil(
		(async () => {
			for (const k of await caches.keys()) if (k !== 'zr-v' + VERSION) await caches.delete(k);
			await self.clients.claim();
		})()
	);
});

self.addEventListener('fetch', (e) => {
	const url = new URL(e.request.url);
	if (url.origin !== location.origin || url.pathname === '/zr-sw.js') return;
	if (OWN.includes(url.pathname)) {
		e.respondWith(caches.match(url.pathname).then((r) => r ?? fetch(e.request)));
		return;
	}
	e.respondWith(answer(e.request, url));
});

async function bridge() {
	const all = await self.clients.matchAll({ includeUncontrolled: true, type: 'window' });
	const found = all.filter((c) => new URL(c.url).pathname === '/zr-bridge.html');
	return found[found.length - 1] ?? null;
}

function fr() {
	return (self.navigator.language || '').toLowerCase().startsWith('fr');
}

function esc(s) {
	return String(s).replace(/[&<>"']/g, (c) => '&#' + c.charCodeAt(0) + ';');
}

function page(title, text, status) {
	title = esc(title);
	text = esc(text);
	const html = `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>${title}</title>
<style>body{font:16px system-ui,sans-serif;background:#0b1020;color:#e8ecf5;display:grid;place-items:center;min-height:100vh;margin:0}main{max-width:28rem;padding:1.5rem}a{color:#9cc2ff}</style>
<main><h1>${title}</h1><p>${text}</p><p><a href="">${fr() ? 'Réessayer' : 'Try again'}</a></p></main>`;
	return new Response(html, { status, headers: { 'content-type': 'text/html; charset=utf-8' } });
}

async function answer(req, url) {
	// A form posted to the box from another website: never sent on. The box's
	// own pages send their changes with fetch, never as a page load.
	if (req.mode === 'navigate' && req.method !== 'GET' && req.method !== 'HEAD') {
		return new Response('', { status: 403 });
	}
	const b = await bridge();
	if (!b) {
		// No extension: the network as usual. At home that's the box itself.
		try {
			return await fetch(req);
		} catch {
			return req.mode === 'navigate'
				? page(
						fr() ? 'Activez l’extension ZOOMRIYA' : 'Turn on the ZOOMRIYA extension',
						fr()
							? 'Loin de la maison, Firefox ouvre votre box grâce à l’extension ZOOMRIYA. Activez-la, puis réessayez.'
							: 'Away from home, Firefox reaches your box through the ZOOMRIYA extension. Turn it on, then try again.',
						503
					)
				: Response.error();
		}
	}
	const body = req.method === 'GET' || req.method === 'HEAD' ? null : await req.arrayBuffer();
	const ch = new MessageChannel();
	const port = ch.port1;
	const inbox = [];
	let wake = null;
	port.onmessage = (e) => {
		inbox.push(e.data);
		if (wake) wake();
	};
	const next = async () => {
		while (inbox.length === 0) await new Promise((ok) => (wake = ok));
		wake = null;
		return inbox.shift();
	};
	// A request from a page on this address, or a page load (a link from
	// elsewhere included): the extension needs to know which.
	const sameOrigin = req.mode !== 'navigate' || req.method === 'GET';
	b.postMessage(
		{ t: 'request', v: VERSION, method: req.method, path: url.pathname + url.search, headers: [...req.headers], body, navigate: req.mode === 'navigate', sameOrigin },
		[ch.port2, ...(body ? [body] : [])]
	);
	const head = await next();
	if (head.t !== 'response') {
		port.close();
		const why = head.t === 'error' ? head.message : '';
		return req.mode === 'navigate'
			? page(fr() ? 'Impossible de joindre votre box' : 'Can’t reach your box', why, 502)
			: new Response(JSON.stringify({ code: 'unreachable', message: why }), { status: 502, headers: { 'content-type': 'application/json' } });
	}
	const status = head.status;
	const headers = new Headers(head.headers);
	if (status === 204 || status === 205 || status === 304 || req.method === 'HEAD') {
		port.close();
		return new Response(null, { status, headers });
	}
	const stream = new ReadableStream({
		async pull(ctl) {
			const m = await next();
			if (m.t === 'chunk') ctl.enqueue(new Uint8Array(m.bytes));
			else if (m.t === 'end') {
				ctl.close();
				port.close();
			} else if (m.t === 'error') {
				ctl.error(new Error(m.message));
				port.close();
			}
		},
		cancel() {
			port.postMessage({ t: 'cancel' });
			port.close();
		}
	});
	return new Response(stream, { status, headers });
}
