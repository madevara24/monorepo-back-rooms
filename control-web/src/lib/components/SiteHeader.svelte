<script lang="ts">
	import { onMount } from 'svelte';
	import ThemeToggle from './ThemeToggle.svelte';

	let signedIn: boolean = $state(false);

	onMount(async () => {
		try {
			const res = await fetch('/api/me');
			signedIn = res.ok;
		} catch {
			signedIn = false;
		}
	});
</script>

<header class="border-b border-line">
	<div class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-4 py-3">
		<a href="/" class="text-lg font-semibold text-text">MBR</a>
		<nav class="flex flex-wrap items-center gap-3 text-sm">
			<a class="text-text hover:text-accent" href="/demo">Demo</a>
			<a class="text-text hover:text-accent" href="/about">About</a>
			{#if signedIn}
				<a class="text-text hover:text-accent" href="/dashboard">Dashboard</a>
			{:else}
				<a class="text-text hover:text-accent" href="/login">Log in</a>
			{/if}
			<ThemeToggle />
		</nav>
	</div>
</header>
