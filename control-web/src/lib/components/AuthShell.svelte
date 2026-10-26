<script lang="ts">
	import type { Snippet } from 'svelte';
	import ThemeToggle from './ThemeToggle.svelte';
	import SkipLink from './SkipLink.svelte';

	let { children }: { children: Snippet } = $props();
</script>

<SkipLink />
<div class="relative flex min-h-screen flex-col items-center justify-center gap-6 px-4">
	<a href="/" class="absolute top-4 left-4 text-lg font-semibold text-text">MBR</a>
	<div class="absolute top-4 right-4">
		<ThemeToggle />
	</div>
	<main id="main" class="flex w-full flex-col items-center gap-6">
		{@render children()}
	</main>
</div>
