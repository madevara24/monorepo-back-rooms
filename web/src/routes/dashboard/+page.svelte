<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { postJSON, type Database, type User } from '$lib/api';

	let user: User | null = $state(null);
	let databases: Database[] = $state([]);
	let loading: boolean = $state(true);
	let error: string = $state('');

	onMount(async () => {
		try {
			const meRes = await fetch('/api/me');
			if (!meRes.ok) {
				await goto('/login');
				return;
			}
			user = await meRes.json();

			const dbRes = await fetch('/api/databases');
			if (!dbRes.ok) {
				error = `could not load databases (${dbRes.status})`;
				return;
			}
			databases = await dbRes.json();
		} finally {
			loading = false;
		}
	});

	async function handleLogout() {
		await postJSON('/api/auth/logout');
		await goto('/');
	}
</script>

<main class="mx-auto flex min-h-screen max-w-2xl flex-col gap-6 p-6">
	<header class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">Dashboard</h1>
		<nav class="flex items-center gap-4">
			<a class="underline" href="/billing">Billing</a>
			{#if user}
				<span class="font-mono text-sm">{user.email}</span>
			{/if}
			<button class="underline" onclick={handleLogout}>Log out</button>
		</nav>
	</header>

	{#if loading}
		<p>Loading...</p>
	{:else if error}
		<p class="text-sm text-red-600">{error}</p>
	{:else}
		<p class="rounded border border-amber-400 bg-amber-50 px-3 py-2 text-sm">
			Provisioning new databases is not available yet. The data plane is not live.
		</p>

		{#if databases.length === 0}
			<p>No databases yet.</p>
		{:else}
			<table class="w-full border-collapse text-left">
				<thead>
					<tr class="border-b">
						<th class="py-2">Name</th>
						<th class="py-2">Size (GB)</th>
						<th class="py-2">Status</th>
						<th class="py-2">Created</th>
					</tr>
				</thead>
				<tbody>
					{#each databases as db (db.id)}
						<tr class="border-b">
							<td class="py-2">{db.name}</td>
							<td class="py-2">{db.size_gb}</td>
							<td class="py-2">{db.status}</td>
							<td class="py-2">{new Date(db.created_at).toLocaleString()}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	{/if}
</main>
