// The family's email account that sends sign-in codes (ZOOMRIYA ID's
// app.mail): the server is guessed from the address, so most admins type
// only the address and its password; More settings shows the rest.

export interface MailAccount {
	host: string;
	port: number;
	security: 'ssl' | 'starttls';
}

/** Common providers' sending servers, by the address's domain. */
const KNOWN: Record<string, MailAccount> = {
	'gmail.com': { host: 'smtp.gmail.com', port: 465, security: 'ssl' },
	'googlemail.com': { host: 'smtp.gmail.com', port: 465, security: 'ssl' },
	'outlook.com': { host: 'smtp-mail.outlook.com', port: 587, security: 'starttls' },
	'hotmail.com': { host: 'smtp-mail.outlook.com', port: 587, security: 'starttls' },
	'live.com': { host: 'smtp-mail.outlook.com', port: 587, security: 'starttls' },
	'msn.com': { host: 'smtp-mail.outlook.com', port: 587, security: 'starttls' },
	'yahoo.com': { host: 'smtp.mail.yahoo.com', port: 465, security: 'ssl' },
	'yahoo.ca': { host: 'smtp.mail.yahoo.com', port: 465, security: 'ssl' },
	'icloud.com': { host: 'smtp.mail.me.com', port: 587, security: 'starttls' },
	'me.com': { host: 'smtp.mail.me.com', port: 587, security: 'starttls' },
	'aol.com': { host: 'smtp.aol.com', port: 465, security: 'ssl' },
	'gmx.com': { host: 'mail.gmx.com', port: 587, security: 'starttls' },
	'proton.me': { host: 'smtp.protonmail.ch', port: 587, security: 'starttls' }
};

/** The server to try for an address; most providers use smtp.<domain>. */
export function guessServer(address: string): MailAccount {
	const domain = address.trim().toLowerCase().split('@')[1] ?? '';
	return KNOWN[domain] ?? { host: domain ? `smtp.${domain}` : '', port: 587, security: 'starttls' };
}

export function isAddress(text: string): boolean {
	return /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(text.trim());
}
