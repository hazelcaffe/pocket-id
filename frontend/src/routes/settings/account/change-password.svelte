<script lang="ts">
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import * as Item from '#lib/components/ui/item/index.js';
	import { Label } from '#lib/components/ui/label/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import PasswordService from '#lib/services/password-service.ts';
	import { getAxiosErrorMessage } from '#lib/utils/error-util.ts';
	import { LockKeyhole } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let { hasPassword = $bindable(false) }: { hasPassword: boolean } = $props();

	const passwordService = new PasswordService();
	let open = $state(false);
	let current = $state('');
	let next = $state('');
	let confirm = $state('');
	let isLoading = $state(false);
	let error: string | undefined = $state();

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		error = undefined;
		if ([...next].length < 10) {
			error = m.password_too_short();
			return;
		}
		if (next !== confirm) {
			error = m.passwords_do_not_match();
			return;
		}
		isLoading = true;
		try {
			await passwordService.changeOwn(current, next);
			toast.success(m.password_updated());
			hasPassword = true;
			open = false;
			current = next = confirm = '';
		} catch (e) {
			error = getAxiosErrorMessage(e);
		}
		isLoading = false;
	}
</script>

<Item.Group class="bg-card rounded-4xl border p-5 shadow-sm">
	<Item.Root class="border-none bg-transparent p-0">
		<Item.Media class="text-primary/80">
			<LockKeyhole class="size-5" />
		</Item.Media>
		<Item.Content class="min-w-52">
			<Item.Title class="text-xl font-semibold">{m.password()}</Item.Title>
			<Item.Description>
				{hasPassword ? m.password_card_description() : m.password_card_description_no_password()}
			</Item.Description>
		</Item.Content>
		<Item.Actions>
			{#if !open}
				<Button variant="outline" onclick={() => (open = true)}>
					{hasPassword ? m.change_password() : m.set_a_password()}
				</Button>
			{/if}
		</Item.Actions>
	</Item.Root>
	{#if open}
		<form class="mt-4 grid max-w-md gap-3" onsubmit={submit}>
			{#if hasPassword}
				<div class="space-y-1.5">
					<Label for="current-password" class="text-xs">{m.current_password()}</Label>
					<Input
						id="current-password"
						type="password"
						autocomplete="current-password"
						required
						bind:value={current}
					/>
				</div>
			{/if}
			<div class="space-y-1.5">
				<Label for="new-password" class="text-xs">{m.new_password()}</Label>
				<Input
					id="new-password"
					type="password"
					autocomplete="new-password"
					minlength={10}
					maxlength={256}
					required
					bind:value={next}
				/>
				<p class="text-muted-foreground text-xs">{m.password_requirements()}</p>
			</div>
			<div class="space-y-1.5">
				<Label for="confirm-password" class="text-xs">{m.confirm_password()}</Label>
				<Input
					id="confirm-password"
					type="password"
					autocomplete="new-password"
					required
					bind:value={confirm}
				/>
			</div>
			{#if error}
				<p class="text-destructive text-sm" role="alert">{error}</p>
			{/if}
			<div class="flex gap-2">
				<Button type="submit" {isLoading}>
					{hasPassword ? m.change_password() : m.set_a_password()}
				</Button>
				<Button variant="ghost" onclick={() => (open = false)}>{m.cancel()}</Button>
			</div>
		</form>
	{/if}
</Item.Group>
