<script lang="ts">
	import SignInWrapper from '#lib/components/login-wrapper.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { Label } from '#lib/components/ui/label/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import PasswordService from '#lib/services/password-service.ts';
	import appConfigStore from '#lib/stores/application-configuration-store.ts';
	import { getApiErrorCode, getAxiosErrorMessage } from '#lib/utils/error-util.ts';
	import { fade } from 'svelte/transition';
	import LoginLogoErrorSuccessIndicator from '../login/components/login-logo-error-success-indicator.svelte';

	const passwordService = new PasswordService();
	let email = $state('');
	let isLoading = $state(false);
	let sent = $state(false);
	let error: string | undefined = $state();

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = undefined;
		isLoading = true;
		try {
			await passwordService.forgot(email.trim());
			sent = true;
		} catch (e) {
			// Only rate limiting / validation errors can surface here; account existence never does
			error =
				getApiErrorCode(e) === 'rate_limited' ? m.please_try_again() : getAxiosErrorMessage(e);
		}
		isLoading = false;
	}
</script>

<svelte:head>
	<title>{m.reset_your_password()} · {$appConfigStore.appName}</title>
</svelte:head>

<SignInWrapper>
	<div class="flex justify-center">
		<LoginLogoErrorSuccessIndicator error={!!error} success={sent} />
	</div>
	<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
		{sent ? m.check_your_email() : m.reset_your_password()}
	</h1>
	{#if error}
		<p class="text-destructive mt-2 text-sm" role="alert" in:fade>{error}</p>
	{:else}
		{#if sent}
			<p class="text-muted-foreground mt-2 max-w-[360px]" in:fade>{m.reset_link_sent()}</p>
		{/if}
	{/if}
	{#if !sent}
		<form class="mt-8 w-full max-w-[320px] space-y-3 text-left" onsubmit={submit}>
			<div class="space-y-1.5">
				<Label for="email" class="text-xs">{m.email()}</Label>
				<Input id="email" type="email" autocomplete="email" required bind:value={email} autofocus />
			</div>
			<div class="pt-2">
				<Button type="submit" class="w-full" {isLoading}>{m.send_reset_link()}</Button>
			</div>
		</form>
	{/if}
	<a
		href="/login"
		class="text-muted-foreground hover:text-foreground mt-6 text-xs transition-colors"
		>{m.back_to_sign_in()}</a
	>
</SignInWrapper>
