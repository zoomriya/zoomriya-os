// Sizes, rates, times and durations, as the native desktop wrote them
// (tools.py's size, rate, when and day; app.py's duration), in the box's
// language and its time zone.

import { _, fill, locale } from './i18n.svelte';

/** The box's time zone (setup-status's zone), once known; the browser's until then. */
let zone: string | undefined;

export function setZone(name: string | undefined): void {
	zone = name || undefined;
}

export function size(n: number): string {
	const units = ['bytes', _('KB'), _('MB'), _('GB'), _('TB')];
	for (const unit of units) {
		if (Math.abs(n) < 1000 || unit === units[units.length - 1]) {
			return unit === 'bytes' ? `${n.toFixed(0)} ${unit}` : `${n.toFixed(1)} ${unit}`;
		}
		n /= 1000;
	}
	return '';
}

export function rate(bps: number): string {
	return `${size(bps)}/s`;
}

function parse(stamp: string): Date | null {
	const moment = new Date(stamp);
	return Number.isNaN(moment.getTime()) ? null : moment;
}

function format(moment: Date, options: Intl.DateTimeFormatOptions): string {
	try {
		return new Intl.DateTimeFormat(locale(), { ...options, timeZone: zone }).format(moment);
	} catch {
		return new Intl.DateTimeFormat(locale(), options).format(moment);
	}
}

/** An ISO time as the box's people read it, or "Never". */
export function when(stamp: string | null | undefined): string {
	if (!stamp) return _('Never');
	const moment = parse(stamp);
	if (!moment) return stamp;
	return format(moment, {
		weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit',
		hourCycle: 'h23'
	});
}

/** An ISO time as a date in words, November 2, 2026, and with at, its time too. */
export function day(stamp: string | null | undefined, at = false): string {
	if (!stamp) return '';
	const moment = parse(stamp);
	if (!moment) return stamp;
	const words = format(moment, { day: 'numeric', month: 'long', year: 'numeric' });
	return at ? fill(_('{date} at {time}'), { date: words, time: clockTime(moment) }) : words;
}

export function clockTime(moment: Date): string {
	return format(moment, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
}

/** The top bar's clock: Thu 12 Nov  17:05. */
export function topBarClock(moment: Date): string {
	return `${format(moment, { weekday: 'short', day: 'numeric', month: 'short' })}  ${clockTime(moment)}`;
}

/** The lock screen's date: Thursday 12 November. */
export function longDate(moment: Date): string {
	return format(moment, { weekday: 'long', day: 'numeric', month: 'long' });
}

export function duration(seconds: number): string {
	const days = Math.floor(seconds / 86400);
	let rest = seconds % 86400;
	const hours = Math.floor(rest / 3600);
	rest %= 3600;
	const minutes = Math.floor(rest / 60);
	const parts = days ? [days === 1 ? _('1 day') : fill(_('{n} days'), { n: days })] : [];
	parts.push(
		hours || days
			? hours === 1 ? _('1 hour') : fill(_('{n} hours'), { n: hours })
			: fill(_('{n} minutes'), { n: minutes })
	);
	return parts.join(', ');
}

export function initials(name: string): string {
	const words = name.replaceAll('_', ' ').split(/\s+/).filter(Boolean);
	const letters = words.length > 1 ? words.slice(0, 2).map((w) => w[0]).join('') : name.slice(0, 2);
	return letters.toUpperCase();
}

/** "Photos" Ph, "Family Hub" FH: one or two letters for an app's tile. */
export function appLetters(name: string): string {
	const words = name.split(/\s+/).filter((w) => /^[\p{L}\p{N}]/u.test(w));
	if (words.length > 1) return (words[0][0] + words[1][0]).toUpperCase();
	const word = words[0] ?? '?';
	return word.slice(0, 1).toUpperCase() + word.slice(1, 2).toLowerCase();
}
