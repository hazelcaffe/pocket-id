<script lang="ts">
	import Logo from '#lib/components/logo.svelte';

	// YuriID: the logo always stays; errors/successes are shown as plain text by the page.
	// The props are kept so existing callers keep compiling.
	const {
		error: _error,
		success: _success
	}: {
		error?: boolean;
		success?: boolean;
	} = $props();
</script>

<Logo class="size-24 rounded-3xl border object-cover shadow-sm" />
