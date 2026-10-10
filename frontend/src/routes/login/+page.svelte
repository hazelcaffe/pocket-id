<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import SignInWrapper from '#lib/components/login-wrapper.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { Label } from '#lib/components/ui/label/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import PasswordService from '#lib/services/password-service.ts';
	import WebAuthnService from '#lib/services/webauthn-service.ts';
	import appConfigStore from '#lib/stores/application-configuration-store.ts';
	import userStore from '#lib/stores/user-store.ts';
	import {
		getApiErrorCode,
		getAxiosErrorMessage,
		getWebauthnErrorMessage
	} from '#lib/utils/error-util.ts';
	import { startAuthentication } from '@simplewebauthn/browser';
	import { fade } from 'svelte/transition';
	import LoginLogoErrorSuccessIndicator from './components/login-logo-error-success-indicator.svelte';

	let { data } = $props();

	const webauthnService = new WebAuthnService();
	const passwordService = new PasswordService();

	let identifier = $state('');
	let password = $state('');
	let isLoading = $state(false);
	let isPasskeyLoading = $state(false);
	let error: string | undefined = $state(undefined);
	let notVerified = $state(false);
	let info: string | undefined = $state(undefined);

	async function signInWithPassword(e: SubmitEvent) {
		e.preventDefault();
		error = undefined;
		info = undefined;
		notVerified = false;
		isLoading = true;
		try {
			const user = await passwordService.login(identifier.trim(), password);
			await userStore.setUser(user);
			await goto(data.redirect || '/settings');
		} catch (e) {
			notVerified = getApiErrorCode(e) === 'email_not_verified';
			error = getAxiosErrorMessage(e);
		}
		isLoading = false;
	}

	async function signInWithPasskey() {
		error = undefined;
		info = undefined;
		notVerified = false;
		isPasskeyLoading = true;
		try {
			const loginOptions = await webauthnService.getLoginOptions();
			const authResponse = await startAuthentication({ optionsJSON: loginOptions });
			const user = await webauthnService.finishLogin(authResponse);

			await userStore.setUser(user);
			await goto(data.redirect || '/settings');
		} catch (e) {
			error = getWebauthnErrorMessage(e);
		}
		isPasskeyLoading = false;
	}

	async function resendVerification() {
		if (!identifier.includes('@')) {
			info = m.email_not_verified_message();
			return;
		}
		try {
			await passwordService.resendVerification(identifier.trim());
		} catch {
			// The response is intentionally generic
		}
		info = m.verification_email_resent();
	}
</script>

<svelte:head>
	<title>{m.sign_in()} · {$appConfigStore.appName}</title>
</svelte:head>

<SignInWrapper>
	<div class="flex justify-center">
		<LoginLogoErrorSuccessIndicator error={!!error} />
	</div>
	<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
		{m.sign_in_to_appname({ appName: $appConfigStore.appName })}
	</h1>
	{#if error}
		<p class="text-destructive mt-2 text-sm" role="alert" in:fade>
			{notVerified ? m.email_not_verified_message() : error}
		</p>
	{:else if info}
		<p class="text-muted-foreground mt-2 text-sm" in:fade>{info}</p>
	{/if}

	<form class="mt-8 w-full max-w-[320px] space-y-3 text-left" onsubmit={signInWithPassword}>
		<div class="space-y-1.5">
			<Label for="identifier" class="text-xs">{m.username_or_email()}</Label>
			<Input
				id="identifier"
				name="username"
				autocomplete="username webauthn"
				autocapitalize="none"
				spellcheck="false"
				required
				bind:value={identifier}
				autofocus
			/>
		</div>
		<div class="space-y-1.5">
			<div class="flex items-center justify-between">
				<Label for="password" class="text-xs">{m.password()}</Label>
				<a
					href="/forgot-password"
					class="text-muted-foreground hover:text-foreground text-xs transition-colors"
					>{m.forgot_password()}</a
				>
			</div>
			<Input
				id="password"
				name="password"
				type="password"
				autocomplete="current-password"
				required
				bind:value={password}
			/>
		</div>
		<div class="space-y-3 pt-2">
			<Button type="submit" class="w-full" {isLoading}>{m.sign_in()}</Button>
			<Button
				type="button"
				variant="outline"
				class="w-full"
				isLoading={isPasskeyLoading}
				onclick={signInWithPasskey}
			>
				{m.sign_in_with_passkey()}
			</Button>
		</div>
		{#if notVerified}
			<button
				type="button"
				class="text-primary block w-full text-center text-xs hover:underline"
				onclick={resendVerification}>{m.resend_verification_email()}</button
			>
		{/if}
		<div
			class="text-muted-foreground flex flex-wrap items-center justify-center gap-x-4 gap-y-1 pt-2 text-xs"
		>
			{#if $appConfigStore.allowUserSignups === 'open'}
				<span>
					{m.no_account()}
					<a href="/signup" class="text-primary font-medium hover:underline">{m.sign_up_link()}</a>
				</span>
				<span aria-hidden="true" class="opacity-40">·</span>
			{/if}
			<a
				href={'/login/alternative' + page.url.search}
				class="hover:text-foreground transition-colors">{m.alternative_sign_in_methods()}</a
			>
		</div>
	</form>
</SignInWrapper>
