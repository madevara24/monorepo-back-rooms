<script lang="ts">
	import { getTheme, toggleTheme } from '$lib/theme.svelte';

	const theme = $derived(getTheme());
</script>

<button
	type="button"
	onclick={toggleTheme}
	class="inline-flex h-11 items-center gap-2 rounded-md border border-line px-3 text-sm text-text hover:border-accent"
	aria-pressed={theme === 'light'}
>
	<span aria-hidden="true">{theme === 'dark' ? '☾' : '☀'}</span>
	<span>{theme === 'dark' ? 'Dark' : 'Light'}</span>
</button>
