// Support Center's helpers (the native desktop's support.py).

import { day } from './format';
import { _, fill } from './i18n.svelte';
import type { Answer } from './api';

export const SUPPORT_URL = 'zoomriya.com/support';

/** The OS's support period (os-release's SUPPORT_END), as a date in words. */
export function updatesUntil(status: Answer): string {
	const end = status.support_end ?? '';
	return end ? fill(_('Until {date}'), { date: day(end) }) : '';
}

/** A bundle's checks in a few words. */
export function findings(bundle: Answer): string {
	const errors = bundle.errors ?? 0, warnings = bundle.warnings ?? 0;
	if (!errors && !warnings) return _('No problems found');
	const parts = [];
	if (errors) parts.push(errors === 1 ? _('1 problem') : fill(_('{n} problems'), { n: errors }));
	if (warnings) parts.push(warnings === 1 ? _('1 warning') : fill(_('{n} warnings'), { n: warnings }));
	return parts.join(', ');
}
