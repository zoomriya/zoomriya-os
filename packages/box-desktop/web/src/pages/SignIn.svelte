<script lang="ts">
	// Signing in (app.py's sign_in_page): the admin's ZOOMRIYA ID, on a card
	// of frosted glass over the box's background. A temporary password asks
	// for their own; an admin with sign-in codes by email turned on enters the
	// code we emailed (not again in this browser for 30 days). Can't sign in?
	// lets an admin back in with the setup code, at the box's own screen only.
	import { post, Problem } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import { passwordProblem } from '../lib/people';
	import Logo from '../components/Logo.svelte';
	import Password from '../components/Password.svelte';
	import Spinner from '../components/Spinner.svelte';
	import Backdrop from './Backdrop.svelte';

	type Step = 'password' | 'new' | 'code' | 'recover';
	let step = $state<Step>('password');
	let username = $state('');
	let password = $state('');
	let newPassword = $state('');
	let newAgain = $state('');
	let code = $state('');
	let ticket = '';
	let busy = $state(false);
	let problem = $state('');
	let note = $state('');
	let sentTo = $state('');
	let remember = $state(true);
	let recoverCode = $state('');
	let recoverUser = $state('');
	let userField = $state<HTMLInputElement>();
	let codeField = $state<HTMLInputElement>();

	$effect(() => {
		userField?.focus();
	});

	const buttonWords = $derived(
		({ password: _('Sign in'), new: _('Sign in'), code: _('Sign in'), recover: _('Let them back in') } as Record<Step, string>)[step]
	);

	function back(): void {
		step = 'password';
		ticket = '';
		code = password = recoverCode = newPassword = newAgain = '';
		problem = note = '';
		sentTo = '';
		remember = true;
	}

	function showCode(given: string, to: string): void {
		step = 'code';
		ticket = given;
		sentTo = to;
		problem = '';
		code = '';
		setTimeout(() => codeField?.focus());
	}

	async function sendAgain(): Promise<void> {
		if (busy) return;
		busy = true;
		problem = note = '';
		try {
			await post('/api/sign-in/code/resend', { ticket });
			note = _('A new code is on its way.');
			code = '';
			codeField?.focus();
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	async function go(): Promise<void> {
		if (busy) return;
		problem = '';
		if (step === 'recover') return recover();
		if (step === 'code') return secondStep();
		note = '';
		if (!username.trim() || !password) return;
		if (step === 'new') {
			const words = passwordProblem(newPassword, newAgain);
			if (words) {
				problem = words;
				return;
			}
		}
		busy = true;
		try {
			const answer = step === 'new'
				? await post<Record<string, unknown>>('/api/sign-in/new-password', { username, password, new_password: newPassword })
				: await post<Record<string, unknown>>('/api/sign-in', { username, password });
			password = newPassword = newAgain = '';
			step = 'password';
			desk.signedIn(answer as never);
		} catch (e) {
			const p = e as Problem;
			if (p.code === 'new_password_needed') {
				// Their password is temporary: their own, then signed in with both.
				step = 'new';
			} else if (p.code === 'second_step_needed') {
				newPassword = newAgain = '';
				showCode(p.ticket, p.sentTo);
			} else {
				problem = p.message;
				if (step === 'new') newPassword = newAgain = '';
				else password = '';
			}
		} finally {
			busy = false;
		}
	}

	async function secondStep(): Promise<void> {
		if (!code.trim()) return;
		busy = true;
		note = '';
		try {
			const answer = await post<Record<string, unknown>>('/api/sign-in/code', { ticket, code, remember });
			back();
			desk.signedIn(answer as never);
		} catch (e) {
			problem = (e as Error).message;
			code = '';
			codeField?.focus();
		} finally {
			busy = false;
		}
	}

	function showRecover(): void {
		step = 'recover';
		recoverUser = username.trim();
		problem = note = '';
	}

	/** The setup code, an admin and their new password: the agent checks the
	 * code (five wrong ones an hour at most) and ZOOMRIYA ID sets it. */
	async function recover(): Promise<void> {
		const name = recoverUser.trim();
		const words = !recoverCode.trim() || !name
			? _("Type the setup code and the admin's username.")
			: passwordProblem(newPassword, newAgain);
		if (words) {
			problem = words;
			return;
		}
		busy = true;
		try {
			await post('/api/recover', { setup_code: recoverCode.trim(), username: name, password: newPassword });
			back();
			newPassword = newAgain = '';
			username = name;
			note = fill(_('{name} has a new password. Sign in with it. Sign-in codes by email are off; turn them on again in Control Panel if you like.'), { name });
		} catch (e) {
			problem = (e as Error).message;
		} finally {
			busy = false;
		}
	}

	const enter = (e: KeyboardEvent) => e.key === 'Enter' && go();
</script>

<Backdrop />
<div class="signin-place">
	<form class="glass-card signin-card" onsubmit={(e) => { e.preventDefault(); go(); }}>
		<div style:align-self="center"><Logo height={22} /></div>
		<p class="lead" style:text-align="center">{_('Sign in with your ZOOMRIYA ID')}</p>
		{#if step !== 'recover'}
			<input bind:this={userField} type="text" bind:value={username} placeholder={_('Username')} aria-label={_('Username')}
				autocomplete="username" disabled={step !== 'password'} autocapitalize="none" spellcheck="false" />
			<Password bind:value={password} placeholder={_('Password')} disabled={step !== 'password'} enter={go} />
		{/if}
		{#if step === 'new'}
			<p class="lead">{_('Choose your own password. At least 8 characters.')}</p>
			<Password bind:value={newPassword} placeholder={_('New password')} autofocus autocomplete="new-password" />
			<Password bind:value={newAgain} placeholder={_('New password again')} enter={go} autocomplete="new-password" />
			<div><button type="button" class="flat" onclick={() => { step = 'password'; newPassword = newAgain = password = ''; }}>{_('Not now')}</button></div>
		{:else if step === 'code'}
			<p class="lead">{sentTo ? fill(_('We sent a 6-digit code to {address}. Enter it to sign in.'), { address: sentTo }) : _('Enter the code we sent to your email.')}</p>
			<input bind:this={codeField} type="text" inputmode="numeric" bind:value={code} placeholder={_('6-digit code')}
				aria-label={_('6-digit code')} autocomplete="one-time-code" onkeydown={enter} />
			<label class="check"><input type="checkbox" bind:checked={remember} />{_("Don't ask again on this device for 30 days")}</label>
			<div class="row-between">
				<button type="button" class="flat" onclick={back}>{_('Back')}</button>
				<button type="button" class="flat" onclick={sendAgain} disabled={busy}>{_('Send a new code')}</button>
			</div>
		{:else if step === 'recover'}
			<p class="lead">{_("Forgot the password, or can't get to your email? Type the box's setup code: it's on the sticker, or the one you wrote down at the first setup. The admin gets a new password, and their sign-in codes by email are turned off.")}</p>
			{#if desk.side !== 'screen'}
				<p class="error">{_("This works at the box's own screen only.")}</p>
			{/if}
			<input type="text" bind:value={recoverCode} placeholder={_('Setup code')} aria-label={_('Setup code')} autocapitalize="characters" />
			<input type="text" bind:value={recoverUser} placeholder={_("The admin's username")} aria-label={_("The admin's username")} autocapitalize="none" />
			<Password bind:value={newPassword} placeholder={_('New password')} autocomplete="new-password" />
			<Password bind:value={newAgain} placeholder={_('New password again')} enter={go} autocomplete="new-password" />
			<div><button type="button" class="flat" onclick={back}>{_('Back')}</button></div>
		{/if}
		{#if problem}<p class="error" role="alert">{problem}</p>{/if}
		<div class="row-between">
			{#if step === 'password'}
				<button type="button" class="flat" onclick={showRecover}>{_("Can't sign in?")}</button>
			{/if}
			<span class="grow"></span>
			{#if busy}<Spinner />{/if}
			<button type="submit" class="primary" disabled={busy || (step === 'recover' && desk.side !== 'screen')}>{buttonWords}</button>
		</div>
		{#if note}<p class="lead">{note}</p>{/if}
	</form>
</div>
