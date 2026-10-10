<script lang="ts">
	import { page } from '$app/state';
	import SignInWrapper from '#lib/components/login-wrapper.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import PasswordService from '#lib/services/password-service.ts';
	import appConfigStore from '#lib/stores/application-configuration-store.ts';
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import LoginLogoErrorSuccessIndicator from '../login/components/login-logo-error-success-indicator.svelte';

	const passwordService = new PasswordService();
	let status: 'pending' | 'ok' | 'error' = $state('pending');

	onMount(async () => {
		const token = page.url.searchParams.get('token') ?? '';
		// Remove the token from the address bar and history
		history.replaceState(history.state, '', '/verify-account');
		try {
			if (!token) throw new Error('missing token');
			await passwordService.verifyEmail(token);
			status = 'ok';
		} catch {
			status = 'error';
		}
	});
</script>

<svelte:head>
	<title>{m.verify_your_email()} · {$appConfigStore.appName}</title>
</svelte:head>

<SignInWrapper>
	<div class="flex justify-center">
		<LoginLogoErrorSuccessIndicator error={status === 'error'} success={status === 'ok'} />
	</div>
	<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
		{m.verify_your_email()}
	</h1>
	<p class="text-muted-foreground mt-2 max-w-[360px]" in:fade>
		{status === 'pending'
			? m.verifying_email()
			: status === 'ok'
				? m.email_verified_you_can_sign_in()
				: m.verification_link_invalid()}
	</p>
	{#if status !== 'pending'}
		<div class="mt-8 w-full max-w-[320px]">
			<Button class="w-full" href="/login">{m.back_to_sign_in()}</Button>
		</div>
	{/if}
</SignInWrapper>
