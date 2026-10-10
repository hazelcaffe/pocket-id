<script lang="ts">
	import MoonIcon from '@lucide/svelte/icons/moon';
	import SunIcon from '@lucide/svelte/icons/sun';
	import { mode, setMode } from 'mode-watcher';

	// YuriID: plain icon button. Follows the system theme until clicked; the choice is then persisted by mode-watcher.
	const isDark = $derived(mode.current === 'dark');
</script>

<button
	type="button"
	class="text-muted-foreground hover:text-foreground focus-visible:ring-ring/40 inline-flex size-9 items-center justify-center rounded-full transition-colors outline-none focus-visible:ring-2"
	title={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
	aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
	onclick={() => setMode(isDark ? 'light' : 'dark')}
>
	{#if isDark}
		<MoonIcon class="size-[1.15rem]" />
	{:else}
		<SunIcon class="size-[1.15rem]" />
	{/if}
</button>
