<script lang="ts">
	import SignInWrapper from '#lib/components/login-wrapper.svelte';
	import TokenSignup from '#lib/components/signup/token-signup.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { Label } from '#lib/components/ui/label/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import PasswordService from '#lib/services/password-service.ts';
	import appConfigStore from '#lib/stores/application-configuration-store.ts';
	import { getAxiosErrorMessage } from '#lib/utils/error-util.ts';
	import { fade } from 'svelte/transition';
	import LoginLogoErrorSuccessIndicator from '../login/components/login-logo-error-success-indicator.svelte';

	let { data } = $props();
	const passwordService = new PasswordService();

	let username = $state('');
	let email = $state('');
	let password = $state('');
	let confirm = $state('');
	let isLoading = $state(false);
	let error: string | undefined = $state();
	let sentTo: string | undefined = $state();

	const usernamePattern = /^[a-zA-Z0-9]([a-zA-Z0-9_.@-]*[a-zA-Z0-9])?$/;

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = undefined;
		if (!usernamePattern.test(username.trim())) {
			error = m.username_can_only_contain();
			return;
		}
		if ([...password].length < 10) {
			error = m.password_too_short();
			return;
		}
		if (password !== confirm) {
			error = m.passwords_do_not_match();
			return;
		}
		isLoading = true;
		try {
			await passwordService.signup({ username: username.trim(), email: email.trim(), password });
			sentTo = email.trim();
			password = '';
			confirm = '';
		} catch (e) {
			error = getAxiosErrorMessage(e);
		}
		isLoading = false;
	}
</script>

<svelte:head>
	<title>{m.create_account()} · {$appConfigStore.appName}</title>
</svelte:head>

{#if data.token}
	<TokenSignup token={data.token} />
{:else}
	<SignInWrapper>
		<div class="flex justify-center">
			<LoginLogoErrorSuccessIndicator error={!!error} success={!!sentTo} />
		</div>

		{#if sentTo}
			<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
				{m.check_your_email()}
			</h1>
			<p class="text-muted-foreground mt-2 max-w-[360px]" in:fade>
				{m.signup_sent_prefix()} <b class="text-foreground font-semibold">{sentTo}</b>.
			</p>
			<div class="mt-8 w-full max-w-[320px]">
				<Button class="w-full" href="/login">{m.back_to_sign_in()}</Button>
			</div>
		{:else if $appConfigStore.allowUserSignups !== 'open'}
			<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
				{m.create_account()}
			</h1>
			<p class="text-muted-foreground mt-2">{m.user_signups_are_disabled()}.</p>
			<div class="mt-8 w-full max-w-[320px]">
				<Button class="w-full" href="/login">{m.back_to_sign_in()}</Button>
			</div>
		{:else}
			<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
				{m.signup()}
			</h1>
			{#if error}
				<p class="text-destructive mt-2 text-sm" role="alert" in:fade>{error}</p>
			{/if}

			<form class="mt-8 w-full max-w-[320px] space-y-3 text-left" onsubmit={submit}>
				<div class="space-y-1.5">
					<Label for="username" class="text-xs">{m.username()}</Label>
					<Input
						id="username"
						name="username"
						autocomplete="username"
						autocapitalize="none"
						spellcheck="false"
						maxlength={50}
						required
						bind:value={username}
						autofocus
					/>
				</div>
				<div class="space-y-1.5">
					<Label for="email" class="text-xs">{m.email()}</Label>
					<Input
						id="email"
						name="email"
						type="email"
						autocomplete="email"
						required
						bind:value={email}
					/>
				</div>
				<div class="space-y-1.5">
					<Label for="password" class="text-xs">{m.password()}</Label>
					<Input
						id="password"
						name="new-password"
						type="password"
						autocomplete="new-password"
						minlength={10}
						maxlength={256}
						required
						bind:value={password}
					/>
					<p class="text-muted-foreground text-xs">{m.password_requirements()}</p>
				</div>
				<div class="space-y-1.5">
					<Label for="confirm" class="text-xs">{m.confirm_password()}</Label>
					<Input
						id="confirm"
						type="password"
						autocomplete="new-password"
						required
						bind:value={confirm}
					/>
				</div>
				<div class="pt-2">
					<Button type="submit" class="w-full" {isLoading}>{m.create_account()}</Button>
				</div>
				<p class="text-muted-foreground pt-2 text-center text-xs">
					{m.already_have_account()}
					<a href="/login" class="text-primary font-medium hover:underline">{m.sign_in()}</a>
				</p>
			</form>
		{/if}
	</SignInWrapper>
{/if}
