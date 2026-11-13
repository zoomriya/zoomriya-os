// Adding an SSH key: its owner can sign in as admin while advanced access is on (access.py's add_key).

import { ask } from '../lib/api';
import { desk } from '../lib/desk.svelte';
import { _ } from '../lib/i18n.svelte';

export function addKey(key: string, name: string, done: () => void): void {
	desk.confirm({
		title: _('Add this key?'), text: _('Its owner can sign in as admin while advanced access is on.'), button: _('Add'),
		action: (password) => ask('advanced-access', { action: 'key-add', key, name: name || undefined }, password), done
	});
}
