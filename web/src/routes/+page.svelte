<script lang="ts">
	import { onMount } from 'svelte';

	let status: string = $state('checking...');
	let signedIn: boolean = $state(false);

	async function checkHealth() {
		try {
			const res = await fetch('/healthz');
			const body = await res.json();
			status = body.status ?? 'unknown';
		} catch {
			status = 'unreachable';
		}
	}

	checkHealth();

	onMount(async () => {
		try {
			const res = await fetch('/api/me');
			signedIn = res.ok;
		} catch {
			signedIn = false;
		}
	});
</script>

<main class="flex min-h-screen flex-col items-center justify-center gap-6">
	<h1 class="text-2xl font-bold">MBR</h1>
	<p>A self-serve Postgres marketplace.</p>
	<p>API status: <span class="font-mono">{status}</span></p>

	{#if signedIn}
		<a class="font-semibold underline" href="/dashboard">Go to dashboard</a>
	{:else}
		<nav class="flex gap-4">
			<a class="font-semibold underline" href="/signup">Sign up</a>
			<a class="font-semibold underline" href="/login">Log in</a>
		</nav>
	{/if}
</main>
