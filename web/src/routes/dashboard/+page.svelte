<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { errorMessage, postJSON, type CreateDatabaseResult, type Database, type User } from '$lib/api';

	let user: User | null = $state(null);
	let databases: Database[] = $state([]);
	let loading: boolean = $state(true);
	let error: string = $state('');

	let createName: string = $state('');
	let createSizeGB: number = $state(1);
	let creating: boolean = $state(false);
	let createError: string = $state('');

	let createdID: string = $state('');
	let createdPassword: string = $state('');
	let copied: boolean = $state(false);

	let actionError: string = $state('');
	let actionTarget: string = $state('');

	const pollTimers = new Map<string, ReturnType<typeof setTimeout>>();

	const monthlyPriceIDR = $derived(createSizeGB * 10000);
	const createdDatabase = $derived(databases.find((d) => d.id === createdID) ?? null);
	const connectionString = $derived(
		createdDatabase && createdDatabase.host && createdDatabase.port
			? `postgres://postgres:${createdPassword}@${createdDatabase.host}:${createdDatabase.port}/postgres?sslmode=disable`
			: ''
	);

	function pollDatabase(id: string) {
		if (pollTimers.has(id)) return;
		const timer = setTimeout(async () => {
			pollTimers.delete(id);
			const res = await fetch(`/api/databases/${id}`);
			if (!res.ok) {
				databases = databases.filter((d) => d.id !== id);
				return;
			}
			const updated: Database = await res.json();
			databases = databases.map((d) => (d.id === id ? updated : d));
			if (updated.status === 'provisioning') pollDatabase(id);
		}, 2000);
		pollTimers.set(id, timer);
	}

	async function loadDatabases() {
		const res = await fetch('/api/databases');
		if (!res.ok) {
			error = `could not load databases (${res.status})`;
			return;
		}
		databases = await res.json();
		for (const db of databases) {
			if (db.status === 'provisioning') pollDatabase(db.id);
		}
	}

	onMount(async () => {
		try {
			const meRes = await fetch('/api/me');
			if (!meRes.ok) {
				await goto('/login');
				return;
			}
			user = await meRes.json();
			await loadDatabases();
		} finally {
			loading = false;
		}
	});

	onDestroy(() => {
		for (const timer of pollTimers.values()) clearTimeout(timer);
		pollTimers.clear();
	});

	async function handleLogout() {
		await postJSON('/api/auth/logout');
		await goto('/');
	}

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		createError = '';
		creating = true;
		try {
			const res = await postJSON('/api/databases', { name: createName, size_gb: createSizeGB });
			if (!res.ok) {
				createError = await errorMessage(res);
				return;
			}
			const result: CreateDatabaseResult = await res.json();
			createdID = result.id;
			createdPassword = result.password;
			copied = false;
			createName = '';
			createSizeGB = 1;
			await loadDatabases();
		} finally {
			creating = false;
		}
	}

	async function handleCopy() {
		if (!connectionString) return;
		await navigator.clipboard.writeText(connectionString);
		copied = true;
	}

	function dismissCreated() {
		createdID = '';
		createdPassword = '';
	}

	async function handleDelete(db: Database) {
		if (!confirm(`Delete database "${db.name}"? This cannot be undone.`)) return;
		actionError = '';
		actionTarget = db.id;
		try {
			const res = await fetch(`/api/databases/${db.id}`, { method: 'DELETE' });
			if (!res.ok) {
				actionError = await errorMessage(res);
				return;
			}
			databases = databases.filter((d) => d.id !== db.id);
			if (db.id === createdID) dismissCreated();
			const timer = pollTimers.get(db.id);
			if (timer) {
				clearTimeout(timer);
				pollTimers.delete(db.id);
			}
		} finally {
			actionTarget = '';
		}
	}
</script>

<main class="mx-auto flex min-h-screen max-w-3xl flex-col gap-6 p-6">
	<header class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">Dashboard</h1>
		<nav class="flex items-center gap-4">
			<a class="underline" href="/billing">Billing</a>
			{#if user?.is_admin}
				<a class="underline" href="/admin">Admin</a>
			{/if}
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
		<section class="flex flex-col gap-2">
			<h2 class="text-xl font-semibold">New database</h2>
			<form class="flex flex-wrap items-end gap-3 rounded border p-3" onsubmit={handleCreate}>
				<label class="flex flex-col gap-1 text-sm">
					Name
					<input class="rounded border px-3 py-2" type="text" bind:value={createName} required />
				</label>
				<label class="flex flex-col gap-1 text-sm">
					Size (GB)
					<input
						class="w-24 rounded border px-3 py-2"
						type="number"
						min="1"
						max="20"
						bind:value={createSizeGB}
						required
					/>
				</label>
				<p class="text-sm text-gray-600">
					~ IDR {monthlyPriceIDR.toLocaleString('id-ID')} / month
				</p>
				<button class="rounded bg-black px-4 py-2 text-white disabled:opacity-50" disabled={creating}>
					Create
				</button>
			</form>

			{#if createError}
				<p class="text-sm text-red-600">{createError}</p>
			{/if}

			{#if createdID}
				<div class="flex flex-col gap-2 rounded border border-amber-400 bg-amber-50 p-3 text-sm">
					<p>Database created. This password is shown once and will not be shown again.</p>
					<p>Password: <span class="font-mono">{createdPassword}</span></p>
					{#if connectionString}
						<div class="flex items-center gap-2">
							<input
								class="flex-1 rounded border px-2 py-1 font-mono text-xs"
								readonly
								value={connectionString}
								onclick={(e) => e.currentTarget.select()}
							/>
							<button class="rounded border px-2 py-1 text-xs" onclick={handleCopy}>
								{copied ? 'Copied' : 'Copy'}
							</button>
						</div>
					{:else if createdDatabase?.status === 'failed'}
						<p class="text-red-600">Provisioning failed. Delete this database and try again.</p>
					{:else}
						<p class="text-xs text-gray-600">
							Provisioning... the connection string will appear here once the database is running.
						</p>
					{/if}
					<button class="self-start text-xs underline" onclick={dismissCreated}>Dismiss</button>
				</div>
			{/if}
		</section>

		<section class="flex flex-col gap-2">
			<h2 class="text-xl font-semibold">Your databases</h2>

			{#if actionError}
				<p class="text-sm text-red-600">{actionError}</p>
			{/if}

			{#if databases.length === 0}
				<p>No databases yet.</p>
			{:else}
				<table class="w-full border-collapse text-left">
					<thead>
						<tr class="border-b">
							<th class="py-2">Name</th>
							<th class="py-2">Size (GB)</th>
							<th class="py-2">Status</th>
							<th class="py-2">Host</th>
							<th class="py-2">Port</th>
							<th class="py-2">Created</th>
							<th class="py-2">Actions</th>
						</tr>
					</thead>
					<tbody>
						{#each databases as db (db.id)}
							<tr class="border-b">
								<td class="py-2">{db.name}</td>
								<td class="py-2">{db.size_gb}</td>
								<td class="py-2" class:text-red-600={db.status === 'failed'}>{db.status}</td>
								<td class="py-2">{db.host ?? '-'}</td>
								<td class="py-2">{db.port ?? '-'}</td>
								<td class="py-2">{new Date(db.created_at).toLocaleString()}</td>
								<td class="py-2">
									{#if db.status === 'running' || db.status === 'failed'}
										<button
											class="rounded border border-red-600 px-2 py-1 text-xs text-red-600 disabled:opacity-50"
											disabled={actionTarget === db.id}
											onclick={() => handleDelete(db)}
										>
											Delete
										</button>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>
	{/if}
</main>
