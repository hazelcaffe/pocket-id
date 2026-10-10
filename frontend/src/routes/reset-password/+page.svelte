<script lang="ts">
	import { page } from '$app/state';
	import SignInWrapper from '#lib/components/login-wrapper.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { Label } from '#lib/components/ui/label/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import PasswordService from '#lib/services/password-service.ts';
	import appConfigStore from '#lib/stores/application-configuration-store.ts';
	import { getApiErrorCode, getAxiosErrorMessage } from '#lib/utils/error-util.ts';
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import LoginLogoErrorSuccessIndicator from '../login/components/login-logo-error-success-indicator.svelte';

	const passwordService = new PasswordService();
	let token = $state('');
	let password = $state('');
	let confirm = $state('');
	let isLoading = $state(false);
	let done = $state(false);
	let invalidLink = $state(false);
	let error: string | undefined = $state();

	onMount(() => {
		token = page.url.searchParams.get('token') ?? '';
		history.replaceState(history.state, '', '/reset-password');
		invalidLink = !token;
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = undefined;
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
			await passwordService.reset(token, password);
			done = true;
			password = '';
			confirm = '';
		} catch (e) {
			if (getApiErrorCode(e) === 'token_invalid_or_expired') {
				invalidLink = true;
			} else {
				error = getAxiosErrorMessage(e);
			}
		}
		isLoading = false;
	}
</script>

<svelte:head>
	<title>{m.choose_a_new_password()} · {$appConfigStore.appName}</title>
</svelte:head>

<SignInWrapper>
	<div class="flex justify-center">
		<LoginLogoErrorSuccessIndicator error={!!error || invalidLink} success={done} />
	</div>
	<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
		{m.choose_a_new_password()}
	</h1>
	{#if done || invalidLink}
		<p class="text-muted-foreground mt-2 max-w-[360px]" in:fade>
			{done ? m.password_reset_success() : m.reset_link_invalid()}
		</p>
		<div class="mt-8 w-full max-w-[320px] space-y-3">
			<Button class="w-full" href={done ? '/login' : '/forgot-password'}>
				{done ? m.back_to_sign_in() : m.send_reset_link()}
			</Button>
		</div>
	{:else}
		{#if error}
			<p class="text-destructive mt-2 text-sm" role="alert" in:fade>{error}</p>
		{:else}
			<p class="text-muted-foreground mt-2" in:fade>{m.password_requirements()}</p>
		{/if}
		<form class="mt-8 w-full max-w-[320px] space-y-3 text-left" onsubmit={submit}>
			<div class="space-y-1.5">
				<Label for="password" class="text-xs">{m.new_password()}</Label>
				<Input
					id="password"
					type="password"
					autocomplete="new-password"
					minlength={10}
					maxlength={256}
					required
					bind:value={password}
					autofocus
				/>
			</div>
			<div class="space-y-1.5">
				<Label for="confirm" class="text-xs">{m.confirm_password()}</Label>
				<Input id="confirm" type="password" autocomplete="new-password" required bind:value={confirm} />
			</div>
			<div class="pt-2">
				<Button type="submit" class="w-full" {isLoading}>{m.set_new_password()}</Button>
			</div>
		</form>
	{/if}
</SignInWrapper>
