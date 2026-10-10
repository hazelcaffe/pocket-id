<script module lang="ts">
	let backgroundImageExists = $state<boolean | undefined>(undefined);
</script>

<script lang="ts">
	import { afterNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { m } from '#lib/paraglide/messages.js';
	import appConfigStore from '#lib/stores/application-configuration-store.ts';
	import { cachedBackgroundImage } from '#lib/utils/cached-image-util.ts';
	import { cn } from '#lib/utils/style.ts';
	import { onMount, type Snippet } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { fade } from 'svelte/transition';
	import * as Card from './ui/card';

	let {
		children,
		showAlternativeSignInMethodButton = false
	}: {
		children: Snippet;
		showAlternativeSignInMethodButton?: boolean;
	} = $props();

	let isInitialLoad = $state(false);
	let animate = $derived(isInitialLoad && !$appConfigStore.disableAnimations);

	onMount(async () => {
		fetch(cachedBackgroundImage.getUrl(), {
			method: 'HEAD'
		})
			.then(async (res) => (backgroundImageExists = res.ok))
			.catch(() => (backgroundImageExists = false));
	});

	afterNavigate((e) => {
		if (e.shallow) return;
		isInitialLoad = !e?.from?.url;
	});

	const isDesktop = new MediaQuery('(min-width: 1024px)');
	let alternativeSignInButton = $derived({
		href:
			page.url.pathname === '/login'
				? `/login/alternative${page.url.search}`
				: `/login/alternative?redirect=${encodeURIComponent(page.url.pathname + page.url.search)}`,
		label: m.alternative_sign_in_methods()
	});
</script>

{#if backgroundImageExists === undefined}
	<div class="bg-background h-screen"></div>
{:else if isDesktop.current}
	<div
		in:fade={{ duration: 150 }}
		class="relative flex h-screen w-full items-center overflow-hidden text-center {backgroundImageExists
			? 'justify-start'
			: 'justify-center'}"
	>
		<div class="relative z-10 flex h-full w-full max-w-[600px] px-10 py-16 2xl:max-w-[720px]">
			<div class="flex h-full w-full flex-col overflow-hidden">
				<div class="relative flex grow flex-col items-center justify-center overflow-auto p-1">
					{@render children()}
					{#if showAlternativeSignInMethodButton}
						<!-- YuriID: small muted link directly under the primary action, like the reference layout -->
						<a
							href={alternativeSignInButton.href}
							class="text-muted-foreground hover:text-foreground mt-6 text-xs transition-colors"
						>
							{alternativeSignInButton.label}
						</a>
					{/if}
				</div>
			</div>
		</div>

		{#if backgroundImageExists}
			<!-- Background image -->
			<div
				class="absolute top-0 right-0 bottom-0 left-[600px] z-0 m-6 overflow-hidden rounded-[40px] 2xl:left-[720px]"
			>
				<img
					src={cachedBackgroundImage.getUrl()}
					class="{cn(animate && 'animate-bg-zoom')} h-full w-full object-cover"
					style="object-position: var(--login-bg-position, 42% 22%);"
					alt={m.login_background()}
				/>
			</div>
		{/if}
	</div>
{:else}
	<div
		class="flex min-h-dvh items-center justify-center bg-cover text-center"
		style="background-image: url({cachedBackgroundImage.getUrl()}); background-position: var(--login-bg-position, 42% 22%);"
	>
		<Card.Root
			class={{
				'mx-3 w-full max-w-md': true,
				'border-0 bg-transparent': !backgroundImageExists
			}}
		>
			<Card.CardContent
				class="px-4 py-10 sm:p-10 {showAlternativeSignInMethodButton ? 'pb-3 sm:pb-3' : ''} "
			>
				{@render children()}
				{#if showAlternativeSignInMethodButton}
					<a
						href={alternativeSignInButton.href}
						class="text-muted-foreground mt-7 flex justify-center text-xs transition-colors hover:underline"
					>
						{alternativeSignInButton.label}
					</a>
				{/if}
			</Card.CardContent>
		</Card.Root>
	</div>
{/if}
