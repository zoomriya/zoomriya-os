import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

// The USB installer's page (zoomriya-installer), from the same look and parts
// as Box Desktop's: built into the installer's package by its build script,
// served by its own server (zoomriya_installer/server.py).
export default defineConfig({
	plugins: [svelte()],
	build: {
		outDir: 'dist-installer', emptyOutDir: true, assetsInlineLimit: 0, target: 'es2022',
		cssTarget: ['chrome111', 'firefox115', 'safari16.4'],
		rollupOptions: { input: 'installer.html' }
	},
	server: { proxy: { '/api': { target: 'http://127.0.0.1:7445' } } }
});
