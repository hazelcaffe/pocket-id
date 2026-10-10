<script lang="ts">
	import { goto } from '$app/navigation';
	import SignInWrapper from '#lib/components/login-wrapper.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import WebAuthnService from '#lib/services/webauthn-service.ts';
	import appConfigStore from '#lib/stores/application-configuration-store.ts';
	import userStore from '#lib/stores/user-store.ts';
	import { getWebauthnErrorMessage } from '#lib/utils/error-util.ts';
	import { startAuthentication } from '@simplewebauthn/browser';
	import { fade } from 'svelte/transition';
	import LoginLogoErrorSuccessIndicator from './components/login-logo-error-success-indicator.svelte';

	let { data } = $props();

	const webauthnService = new WebAuthnService();

	let isLoading = $state(false);
	let error: string | undefined = $state(undefined);

	async function authenticate() {
		error = undefined;
		isLoading = true;
		try {
			const loginOptions = await webauthnService.getLoginOptions();
			const authResponse = await startAuthentication({ optionsJSON: loginOptions });
			const user = await webauthnService.finishLogin(authResponse);

			await userStore.setUser(user);
			await goto(data.redirect || '/settings');
		} catch (e) {
			error = getWebauthnErrorMessage(e);
		}
		isLoading = false;
	}
</script>

<svelte:head>
	<title>{m.sign_in()} · {$appConfigStore.appName}</title>
</svelte:head>

<SignInWrapper showAlternativeSignInMethodButton>
	<div class="flex justify-center">
		<LoginLogoErrorSuccessIndicator error={!!error} />
	</div>
	<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
		{m.sign_in_to_appname({ appName: $appConfigStore.appName })}
	</h1>
	{#if error}
		<p class="text-muted-foreground mt-2" in:fade>
			{error}. {m.please_try_to_sign_in_again()}
		</p>
	{:else}
		<p class="text-muted-foreground mt-2" in:fade>
			{m.authenticate_with_passkey_to_access_account()}
		</p>
	{/if}
	<div class="mt-8 w-full max-w-[320px] space-y-3">
		<Button class="w-full" {isLoading} onclick={authenticate} autofocus={true}>
			{error ? m.try_again() : m.authenticate()}
		</Button>
		{#if $appConfigStore.allowUserSignups === 'open'}
			<a
				href="/signup"
				class="text-muted-foreground hover:text-foreground block text-xs transition-colors"
				>{m.signup()}</a
			>
		{/if}
	</div>
</SignInWrapper>
