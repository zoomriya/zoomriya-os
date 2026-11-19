// The bridge between zr-sw.js and the ZOOMRIYA extension in Firefox. It runs
// only in a hidden frame of the extension's background page: the box serves
// this page with `frame-ancestors moz-extension:`, so no website can frame it.
// It installs zr-sw.js, then passes each request the worker hands it to the
// extension. See zr-sw.js.
const VERSION = 1;
const up = (msg, transfer) => window.parent.postMessage(msg, '*', transfer ?? []);

if (window.parent !== window) {
	navigator.serviceWorker.addEventListener('message', (e) => {
		if (e.data && e.data.t === 'request' && e.ports[0]) up(e.data, [e.ports[0]]);
	});
	navigator.serviceWorker.startMessages();
	// At home this installs or updates the worker. Away it can't fetch it,
	// and the one installed at home carries on.
	navigator.serviceWorker.register('/zr-sw.js', { scope: '/' }).catch(() => {});
	navigator.serviceWorker.ready.then(() => up({ t: 'bridge-ready', v: VERSION }));
}
