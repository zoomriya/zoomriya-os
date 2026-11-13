// Each tool's section and what draws it, by "tool section".

import type { Component } from 'svelte';
import About from './About.svelte';
import Access from './Access.svelte';
import Activity from './Activity.svelte';
import Appearance from './Appearance.svelte';
import AppsInstalled from './AppsInstalled.svelte';
import AppsUsb from './AppsUsb.svelte';
import Backups from './Backups.svelte';
import ByApp from './ByApp.svelte';
import Contact from './Contact.svelte';
import Diagnostics from './Diagnostics.svelte';
import Display from './Display.svelte';
import Drives from './Drives.svelte';
import Help from './Help.svelte';
import Keyboard from './Keyboard.svelte';
import Language from './Language.svelte';
import LogNotifications from './LogNotifications.svelte';
import Logs from './Logs.svelte';
import Network from './Network.svelte';
import Notifications from './Notifications.svelte';
import People from './People.svelte';
import Resources from './Resources.svelte';
import Restarts from './Restarts.svelte';
import Restore from './Restore.svelte';
import Security from './Security.svelte';
import Sound from './Sound.svelte';
import Space from './Space.svelte';
import StorageOverview from './StorageOverview.svelte';
import Temperature from './Temperature.svelte';
import Time from './Time.svelte';

type Section = Component<{ shown?: boolean }>;

/** A log of one kind, as a section. */
function log(kind: string): Section {
	return ((anchor: unknown, props: { shown?: boolean }) =>
		(Logs as unknown as (a: unknown, p: unknown) => unknown)(anchor, { ...props, kind })) as unknown as Section;
}

export const SECTIONS: Record<string, Section> = {
	'control network': Network, 'control display': Display, 'control appearance': Appearance, 'control sound': Sound,
	'control keyboard': Keyboard, 'control language': Language, 'control time': Time, 'control people': People,
	'control notifications': Notifications, 'control security': Security, 'control access': Access,
	'control restarts': Restarts, 'control restore': Restore, 'control about': About,
	'storage overview': StorageOverview, 'storage drives': Drives, 'storage space': Space, 'storage backups': Backups,
	'apps installed': AppsInstalled, 'apps usb': AppsUsb,
	'resources overview': Resources, 'resources apps': ByApp, 'resources temperature': Temperature,
	'logs events': log('events'), 'logs notifications': LogNotifications, 'logs audit': log('audit'),
	'logs signins': log('signins'), 'logs activity': Activity, 'logs system': log('system'),
	'logs diagnostics': Diagnostics,
	'support diagnostics': Diagnostics, 'support contact': Contact, 'support help': Help
};
