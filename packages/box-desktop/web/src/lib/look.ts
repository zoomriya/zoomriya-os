// The box's background and each admin's accent, as the page draws them
// (the native desktop's appearance.py).

import { ACCENTS, COLOURS, DEFAULT_ACCENT, DEFAULT_BACKGROUND, OWN, PICTURES } from './words';

/** The background key draws: one of ours, a plain colour, or the family's own picture. */
export function backgroundStyle(key: string, hasPicture: boolean, stamp: number): string {
	if (key in COLOURS) return `background: ${COLOURS[key][1]}`;
	if (key === OWN && hasPicture) return `background-image: url(/api/box/picture?v=${stamp})`;
	return `background-image: url(/backgrounds/${(PICTURES[key] ?? PICTURES[DEFAULT_BACKGROUND])[1]})`;
}

function tint(colour: string, alpha: number): string {
	const [r, g, b] = [1, 3, 5].map((i) => parseInt(colour.slice(i, i + 2), 16));
	return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

/** The page's theme and accent: Midnight's class, and the accent's colours
 * (the apps' own for blue, from tokens.css). */
export function applyLook(theme: string, accent: string): void {
	const root = document.documentElement;
	root.classList.toggle('midnight', theme === 'midnight');
	const style = root.style;
	if (accent === DEFAULT_ACCENT || !(accent in ACCENTS)) {
		for (const name of ['--accent', '--accent-hover', '--on-accent', '--selected']) style.removeProperty(name);
		return;
	}
	const [, day, dayHover, night, nightHover] = ACCENTS[accent];
	const midnight = theme === 'midnight';
	style.setProperty('--accent', midnight ? night : day);
	style.setProperty('--accent-hover', midnight ? nightHover : dayHover);
	style.setProperty('--on-accent', midnight ? '#0b0e1a' : '#ffffff');
	style.setProperty('--selected', midnight ? tint(night, 0.14) : tint(day, 0.12));
}
