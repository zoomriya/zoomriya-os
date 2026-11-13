// Open on a phone or computer: the address, and a QR code a phone's camera
// opens it from (app.py's open_elsewhere).

import { desk } from '../lib/desk.svelte';
import Elsewhere from './Elsewhere.svelte';

export function openElsewhere(name: string, address: string): void {
	desk.menuOpen = false;
	desk.sheet(Elsewhere, { name, address });
}
