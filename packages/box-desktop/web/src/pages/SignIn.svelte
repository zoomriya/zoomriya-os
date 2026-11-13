<script lang="ts">
	// Signing in (app.py's sign_in_page): the admin's ZOOMRIYA ID, on a card
	// of frosted glass over the box's background. A temporary password asks
	// for their own; the second step asks for the authenticator app's code,
	// or sets one up and shows the backup codes once. Can't sign in? lets an
	// admin back in with the setup code, at the box's own screen only.
	import { post, Problem } from '../lib/api';
	import { desk } from '../lib/desk.svelte';
	import { _, fill } from '../lib/i18n.svelte';
	import { passwordProblem } from '../lib/people';
	import Logo from '../components/Logo.svelte';
	import Password from '../components/Password.svelte';
	import Spinner from '../components/Spinner.svelte';
	import Backdrop from './Backdrop.svelte';

	type Step = 'password' | 'new' | 'code' | 'setup' | 'codes' | 'recover';
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
	let setupQr = $state('');
	let setupKey = $state('');
	let backupCodes = $state<string[]>([]);
	let kept = $state(false);
	let recoverCode = $state('');
	let recoverUser = $state('');
	let signedAnswer: Record<string, unknown> | null = null;
	let userField = $state<HTMLInputElement>();
	let codeField = $state<HTMLInputElement>();

	$effect(() => {
		userField?.focus();
	});

	const buttonWords = $derived(
		({ password: _('Sign in'), new: _('Sign in'), code: _('Sign in'), setup: _('Turn it on'), codes: _('Continue'),
			recover: _('Let them back in') } as Record<Step, string>)[step]
	);

	function back(): void {
		step = 'password';
		ticket = '';
		signedAnswer = null;
		code = password = recoverCode = newPassword = newAgain = '';
		problem = '';
		backupCodes = [];
		setupQr = setupKey = '';
		kept = false;
	}

	async function showSecondStep(next: 'code' | 'setup', given: string): Promise<void> {
		step = next;
		ticket = given;
		problem = '';
		code = '';
		if (next === 'code') {
			setTimeout(() => codeField?.focus());
			return;
		}
		setupQr = setupKey = '';
		try {
			const answer = await post<{ qr_svg: string; key: string }>('/api/sign-in/second-step/setup', { ticket });
			setupQr = answer.qr_svg;
			setupKey = fill(_("Can't scan it? Type this key: {key}"), { key: answer.key });
			setTimeout(() => codeField?.focus());
		} catch (e) {
			problem = (e as Error).message;
		}
	}

	async function go(): Promise<void> {
		if (busy) return;
		problem = '';
		if (step === 'codes') {
			// The backup codes are written down: on to the desktop.
			if (kept && signedAnswer) desk.signedIn(signedAnswer as never);
			return;
		}
		if (step === 'recover') return recover();
		if (step === 'code' || step === 'setup') return secondStep();
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
			} else if (p.code === 'second_step_needed' || p.code === 'second_step_setup_needed') {
				newPassword = newAgain = '';
				await showSecondStep(p.code === 'second_step_needed' ? 'code' : 'setup', p.ticket);
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
		try {
			if (step === 'code') {
				const answer = await post<Record<string, unknown>>('/api/sign-in/code', { ticket, code });
				back();
				desk.signedIn(answer as never);
			} else {
				const answer = await post<Record<string, unknown>>('/api/sign-in/second-step/finish', { ticket, code });
				signedAnswer = answer;
				backupCodes = (answer.backup_codes as string[]) ?? [];
				kept = false;
				step = 'codes';
			}
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
			note = fill(_('{name} has a new password. Sign in with it, then set up the code from your phone again.'), { name });
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
			<p class="lead">{_('Open your authenticator app and enter the code it shows for ZOOMRIYA. No phone? Enter one of your backup codes.')}</p>
			<input bind:this={codeField} type="text" bind:value={code} placeholder={_('Code')} aria-label={_('Code')} autocomplete="one-time-code" onkeydown={enter} />
			<div><button type="button" class="flat" onclick={back}>{_('Back')}</button></div>
		{:else if step === 'setup'}
			<p class="lead">{_('Admins look after the whole family\'s box, so they sign in with a password and a code from their phone. Set it up once.')}</p>
			<p style:white-space="pre-line" style:margin="0">{_('1. On your phone, open an authenticator app, any app that makes sign-in codes.\n2. Add an account and scan this code.\n3. Enter the 6-digit code it shows.')}</p>
			<div class="qr" style:width="198px" style:height="198px">{#if setupQr}{@html setupQr}{/if}</div>
			<span class="dim selectable">{setupKey}</span>
			<input bind:this={codeField} type="text" inputmode="numeric" bind:value={code} placeholder={_('6-digit code')}
				aria-label={_('6-digit code')} autocomplete="one-time-code" onkeydown={enter} />
			<div><button type="button" class="flat" onclick={back}>{_('Back')}</button></div>
		{:else if step === 'codes'}
			<p class="lead">{_("Your backup codes. If you ever don't have your phone, each one signs you in once. Write them down and keep them somewhere safe at home. They won't be shown again.")}</p>
			<div class="codes selectable">
				{#each backupCodes as backup (backup)}<span class="mono">{backup}</span>{/each}
			</div>
			<label class="check"><input type="checkbox" bind:checked={kept} />{_("I've written them down")}</label>
		{:else if step === 'recover'}
			<p class="lead">{_("Forgot the password, or lost your phone and backup codes? Type the box's setup code: it's on the sticker, or the one you wrote down at the first setup. The admin gets a new password and sets up their phone again.")}</p>
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
			<button type="submit" class="primary" disabled={busy || (step === 'codes' && !kept) || (step === 'recover' && desk.side !== 'screen')}>{buttonWords}</button>
		</div>
		{#if note}<p class="lead">{note}</p>{/if}
	</form>
</div>

<style>
	.codes { display: grid; grid-template-columns: auto auto; gap: 6px 24px; justify-content: center; }
	.qr { align-self: center; overflow: hidden; }
	.qr :global(svg) { width: 100%; height: 100%; }
</style>
