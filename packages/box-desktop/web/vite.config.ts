import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vitest/config';

// Built into box-desktopd (../src/dist, go:embed). `npm run dev` proxies the
// API to a box-desktopd started by scripts/preview-desktop.sh.
export default defineConfig({
	plugins: [svelte()],
	build: {
		outDir: '../src/dist', emptyOutDir: true, assetsInlineLimit: 0, target: 'es2022',
		// The browsers Box Desktop is for, and WebKitGTK on the box's screen.
		cssTarget: ['chrome111', 'firefox115', 'safari16.4']
	},
	server: { proxy: { '/api': { target: 'http://127.0.0.1:17443', ws: false } } },
	test: { environment: 'node', include: ['src/**/*.test.ts'] }
});
