// Control Panel › Devices: what connectd says about paired phones and
// laptops, through box-desktopd (/api/connect/..., connect.go), and its
// refusals in Box Desktop's own words. No ZOOMRIYA server is involved.

import { Problem, del, get, post } from '../lib/api';
import { _ } from '../lib/i18n.svelte';

export interface DevicesStatus {
	box_key: string;
	devices: number;
	connected: number;
	pairing: boolean;
}

export interface Device {
	id: string;
	name: string;
	paired_at: number;
	last_seen: number;
	connected: boolean;
	/** A laptop, paired with a short code in the ZOOMRIYA laptop app. */
	laptop?: boolean;
}

/** connectd's codes in Box Desktop's words; anything else, the box's own sentence. */
export function deviceWords(e: unknown): string {
	const code = e instanceof Problem ? e.code : '';
	switch (code) {
		case 'connect_unavailable': return _('Phones and laptops cannot reach this box right now. Restart the box; if this stays, open Support Center.');
		case 'not_found': return _('That device is not paired any more.');
		default: return e instanceof Error ? e.message : _("The box couldn't do that.");
	}
}

export const status = () => get<DevicesStatus>('/api/connect/v1/status');
export const devices = () => get<Device[]>('/api/connect/v1/devices');
export const pairing = (name: string) =>
	post<{ url: string; expires_in: number }>('/api/connect/v1/pairing', { name });
export const laptopPairing = (name: string) =>
	post<{ code: string; expires_in: number }>('/api/connect/v1/pairing', { name, laptop: true });
export const stopPairing = () => del('/api/connect/v1/pairing');
export const removeDevice = (id: string, password: string) => del(`/api/connect/v1/devices/${id}`, { password });

/** Seconds since 1970 as an ISO time, for format.ts. */
export const iso = (seconds: number) => new Date(seconds * 1000).toISOString();
