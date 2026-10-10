<script lang="ts">
	import { page } from '$app/state';
	import SignInWrapper from '#lib/components/login-wrapper.svelte';
	import Logo from '#lib/components/logo.svelte';
	import * as Item from '#lib/components/ui/item/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import appConfigStore from '#lib/stores/application-configuration-store.ts';
	import {
		LucideChevronRight,
		LucideMail,
		LucideQrCode,
		LucideRectangleEllipsis
	} from '@lucide/svelte';

	const methods = [
		{
			icon: LucideQrCode,
			title: m.sign_in_with_another_device(),
			description: m.sign_in_with_another_device_description(),
			href: '/login/alternative/device'
		},
		{
			icon: LucideRectangleEllipsis,
			title: m.login_code(),
			description: m.enter_a_login_code_to_sign_in(),
			href: '/login/alternative/code'
		}
	];

	if ($appConfigStore.emailOneTimeAccessAsUnauthenticatedEnabled) {
		methods.push({
			icon: LucideMail,
			title: m.email_login(),
			description: m.request_a_login_code_via_email(),
			href: '/login/alternative/email'
		});
	}
</script>

<svelte:head>
	<title>{m.sign_in()}</title>
</svelte:head>

<SignInWrapper>
	<div class="flex h-full flex-col justify-center">
		<div class="flex justify-center">
			<Logo class="size-24 rounded-3xl border object-cover shadow-sm" />
		</div>
		<h1 class="font-display mt-5 text-3xl font-bold tracking-tight sm:text-4xl">
			{m.alternative_sign_in()}
		</h1>
		<Item.Group class="mt-8 gap-3">
			{#each methods as method (method.href)}
				<Item.Root variant="outline" class="gap-5">
					{#snippet child({ props })}
						<a href={method.href + page.url.search} {...props}>
							<Item.Media class="text-primary !translate-y-0 !self-center">
								<method.icon class="size-7" />
							</Item.Media>
							<Item.Content class="text-start">
								<Item.Title class="text-lg font-semibold">{method.title}</Item.Title>
								<Item.Description>{method.description}</Item.Description>
							</Item.Content>
							<Item.Actions>
								<LucideChevronRight class="size-5" />
							</Item.Actions>
						</a>
					{/snippet}
				</Item.Root>
			{/each}
		</Item.Group>
	</div>
</SignInWrapper>
