// The first setup's fixed address rules (zoomriya_setup/app.py's fixed_settings).

import { _ } from './i18n.svelte';

function ipv4(text: string): number[] | null {
	const parts = text.trim().split('.');
	if (parts.length !== 4 || !parts.every((p) => /^\d{1,3}$/.test(p) && Number(p) <= 255 && (p === '0' || !p.startsWith('0')))) return null;
	return parts.map(Number);
}

/** A mask's length (255.255.255.0 is 24), or a length typed as is; null if neither. */
export function prefixLength(mask: string): number | null {
	const m = mask.trim();
	if (/^\d{1,2}$/.test(m) && Number(m) <= 32) return Number(m);
	const parts = ipv4(m);
	if (!parts) return null;
	const bits = parts.map((p) => p.toString(2).padStart(8, '0')).join('');
	return /^1*0*$/.test(bits) ? bits.indexOf('0') === -1 ? 32 : bits.indexOf('0') : null;
}

export function maskOf(bits: number): string {
	const all = '1'.repeat(bits).padEnd(32, '0');
	return [0, 8, 16, 24].map((i) => parseInt(all.slice(i, i + 8), 2)).join('.');
}

/** The agent's network-set fields, or an Error worded for the person. */
export function fixedSettings(address: string, mask: string, router: string, dns: string): Record<string, unknown> {
	const bits = prefixLength(mask);
	if (bits === null) throw new Error(_('The subnet mask should look like 255.255.255.0.'));
	if (!ipv4(address)) throw new Error(_("The box's address should look like 192.168.1.40."));
	const servers = dns.replaceAll(',', ' ').split(/\s+/).filter(Boolean);
	return { mode: 'fixed', address: `${address.trim()}/${bits}`, gateway: router.trim(), dns: servers.length ? servers : [router.trim()] };
}
