<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { env as publicEnv } from '$env/dynamic/public';
	import {
		errorMessage,
		postJSON,
		type AdminDatabase,
		type AdminJob,
		type AdminNode,
		type AdminOverview,
		type AdminUser,
		type CreateDatabaseResult
	} from '$lib/api';

	const dokployURL = publicEnv.PUBLIC_DOKPLOY_URL ?? '';

	let loading: boolean = $state(true);
	let error: string = $state('');

	let overview: AdminOverview | null = $state(null);
	let users: AdminUser[] = $state([]);
	let databases: AdminDatabase[] = $state([]);
	let nodes: AdminNode[] = $state([]);
	let jobs: AdminJob[] = $state([]);

	let createUserId: string = $state('');
	let createName: string = $state('');
	let createSizeGB: number = $state(1);
	let creating: boolean = $state(false);
	let createError: string = $state('');
	let created: CreateDatabaseResult | null = $state(null);

	let actionError: string = $state('');
	let actionTarget: string = $state('');

	async function loadJSON<T>(path: string): Promise<T> {
		const res = await fetch(path);
		if (!res.ok) {
			throw new Error(`could not load ${path} (${res.status})`);
		}
		return res.json();
	}

	async function reloadDatabasesAndOverview() {
		[overview, databases, jobs] = await Promise.all([
			loadJSON<AdminOverview>('/api/admin/overview'),
			loadJSON<AdminDatabase[]>('/api/admin/databases'),
			loadJSON<AdminJob[]>('/api/admin/jobs')
		]);
	}

	onMount(async () => {
		try {
			const meRes = await fetch('/api/me');
			if (!meRes.ok) {
				await goto('/login');
				return;
			}
			const me = await meRes.json();
			if (!me.is_admin) {
				await goto('/');
				return;
			}

			[overview, users, databases, nodes, jobs] = await Promise.all([
				loadJSON<AdminOverview>('/api/admin/overview'),
				loadJSON<AdminUser[]>('/api/admin/users'),
				loadJSON<AdminDatabase[]>('/api/admin/databases'),
				loadJSON<AdminNode[]>('/api/admin/nodes'),
				loadJSON<AdminJob[]>('/api/admin/jobs')
			]);
			if (users.length > 0) {
				createUserId = users[0].id;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'could not load admin data';
		} finally {
			loading = false;
		}
	});

	function formatCounts(counts: Record<string, number>): string {
		const entries = Object.entries(counts);
		if (entries.length === 0) return 'none';
		return entries.map(([status, count]) => `${status}: ${count}`).join(', ');
	}

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		createError = '';
		created = null;
		creating = true;
		try {
			const res = await postJSON('/api/admin/databases', {
				user_id: createUserId,
				name: createName,
				size_gb: createSizeGB
			});
			if (!res.ok) {
				createError = await errorMessage(res);
				return;
			}
			created = await res.json();
			createName = '';
			await reloadDatabasesAndOverview();
		} finally {
			creating = false;
		}
	}

	async function handleDelete(db: AdminDatabase) {
		if (!confirm(`Delete database "${db.name}" (owned by ${db.owner_email})? This cannot be undone.`)) {
			return;
		}
		actionError = '';
		actionTarget = db.id;
		try {
			const res = await fetch(`/api/admin/databases/${db.id}`, { method: 'DELETE' });
			if (!res.ok) {
				actionError = await errorMessage(res);
				return;
			}
			await reloadDatabasesAndOverview();
		} finally {
			actionTarget = '';
		}
	}

	async function handleRetry(db: AdminDatabase) {
		if (!confirm(`Retry database "${db.name}" (owned by ${db.owner_email})?`)) {
			return;
		}
		actionError = '';
		actionTarget = db.id;
		try {
			const res = await postJSON(`/api/admin/databases/${db.id}/retry`);
			if (!res.ok) {
				actionError = await errorMessage(res);
				return;
			}
			await reloadDatabasesAndOverview();
		} finally {
			actionTarget = '';
		}
	}
</script>

<main class="mx-auto flex min-h-screen max-w-5xl flex-col gap-8 p-6">
	<header class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">Admin</h1>
		<nav class="flex items-center gap-4">
			{#if dokployURL}
				<a class="underline" href={dokployURL} target="_blank" rel="noopener noreferrer">
					Dokploy
				</a>
			{/if}
			<a class="underline" href="/dashboard">Dashboard</a>
		</nav>
	</header>

	{#if loading}
		<p>Loading...</p>
	{:else if error}
		<p class="text-sm text-red-600">{error}</p>
	{:else}
		<section class="flex flex-col gap-2">
			<h2 class="text-xl font-semibold">Overview</h2>
			{#if overview}
				<table class="w-full border-collapse text-left">
					<tbody>
						<tr class="border-b">
							<th class="py-2 pr-4 font-medium">Users</th>
							<td class="py-2">{overview.users}</td>
						</tr>
						<tr class="border-b">
							<th class="py-2 pr-4 font-medium">Databases by status</th>
							<td class="py-2">{formatCounts(overview.databases_by_status)}</td>
						</tr>
						<tr class="border-b">
							<th class="py-2 pr-4 font-medium">Nodes</th>
							<td class="py-2">{overview.nodes_fresh} fresh, {overview.nodes_stale} stale</td>
						</tr>
						<tr class="border-b">
							<th class="py-2 pr-4 font-medium">Jobs by status</th>
							<td class="py-2">{formatCounts(overview.jobs_by_status)}</td>
						</tr>
					</tbody>
				</table>
			{/if}
		</section>

		<section class="flex flex-col gap-2">
			<h2 class="text-xl font-semibold">Nodes</h2>
			{#if nodes.length === 0}
				<p>No nodes yet.</p>
			{:else}
				<table class="w-full border-collapse text-left">
					<thead>
						<tr class="border-b">
							<th class="py-2">ID</th>
							<th class="py-2">Version</th>
							<th class="py-2">Last heartbeat</th>
							<th class="py-2">Capacity</th>
							<th class="py-2">Containers</th>
						</tr>
					</thead>
					<tbody>
						{#each nodes as node (node.id)}
							<tr class="border-b align-top">
								<td class="py-2">{node.id}</td>
								<td class="py-2">{node.version || '-'}</td>
								<td class="py-2">
									{node.last_heartbeat ? new Date(node.last_heartbeat).toLocaleString() : 'never'}
								</td>
								<td class="py-2 font-mono text-xs">{JSON.stringify(node.capacity)}</td>
								<td class="py-2">
									{#if node.containers.length === 0}
										<span class="text-sm text-gray-500">none</span>
									{:else}
										<ul class="flex flex-col gap-1">
											{#each node.containers as container (container.database_id)}
												<li class="text-sm">
													{container.database_id.slice(0, 8)} &middot; {container.status} &middot; port
													{container.port}
												</li>
											{/each}
										</ul>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>

		<section class="flex flex-col gap-2">
			<h2 class="text-xl font-semibold">Databases</h2>

			<form class="flex flex-wrap items-end gap-3 rounded border p-3" onsubmit={handleCreate}>
				<label class="flex flex-col gap-1 text-sm">
					User
					<select class="rounded border px-3 py-2" bind:value={createUserId} required>
						{#each users as u (u.id)}
							<option value={u.id}>{u.email}</option>
						{/each}
					</select>
				</label>
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
				<button
					class="rounded bg-black px-4 py-2 text-white disabled:opacity-50"
					disabled={creating || users.length === 0}
				>
					Create
				</button>
			</form>

			{#if createError}
				<p class="text-sm text-red-600">{createError}</p>
			{/if}

			{#if created}
				<div class="flex flex-col gap-1 rounded border border-amber-400 bg-amber-50 px-3 py-2 text-sm">
					<p>Database created. This password is shown once and will not be shown again.</p>
					<p>Password: <span class="font-mono">{created.password}</span></p>
					<p class="break-all font-mono text-xs">{created.connection_string}</p>
					<p class="text-xs text-gray-600">
						Needs a PostgreSQL 17+ client (psql 17+, or a recent driver version with direct SSL
						support). Older client versions may fail to connect.
					</p>

					<p>DBeaver / JDBC URL (DBeaver does not accept the connection string above):</p>
					<p class="break-all font-mono text-xs">{created.jdbc_url}</p>
					<p class="text-xs text-gray-600">
						Enter the username (<code>postgres</code>) and password separately in DBeaver. This
						URL's <code>sslmode=require</code> encrypts the connection but does not verify the
						server's identity. For <code>sslmode=verify-full</code>, which also verifies the
						server, download the
						<a
							class="underline"
							href="https://letsencrypt.org/certs/isrgrootx1.pem"
							target="_blank"
							rel="noopener noreferrer">ISRG Root X1 certificate</a
						>
						and replace the query string with
						<code>sslmode=verify-full&amp;sslNegotiation=direct&amp;sslrootcert=&lt;path to the downloaded PEM&gt;</code
						>. Requires pgJDBC 42.7.4 or newer.
					</p>
				</div>
			{/if}

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
							<th class="py-2">Owner</th>
							<th class="py-2">Status</th>
							<th class="py-2">Node</th>
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
								<td class="py-2">{db.owner_email}</td>
								<td class="py-2">{db.status}</td>
								<td class="py-2">{db.node_id ?? '-'}</td>
								<td class="py-2">{db.host ?? '-'}</td>
								<td class="py-2">{db.port ?? '-'}</td>
								<td class="py-2">{new Date(db.created_at).toLocaleString()}</td>
								<td class="py-2">
									<div class="flex gap-2">
										{#if db.status === 'running' || db.status === 'failed'}
											<button
												class="rounded border border-red-600 px-2 py-1 text-xs text-red-600 disabled:opacity-50"
												disabled={actionTarget === db.id}
												onclick={() => handleDelete(db)}
											>
												Delete
											</button>
										{/if}
										{#if db.status === 'failed'}
											<button
												class="rounded border px-2 py-1 text-xs disabled:opacity-50"
												disabled={actionTarget === db.id}
												onclick={() => handleRetry(db)}
											>
												Retry
											</button>
										{/if}
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>

		<section class="flex flex-col gap-2">
			<h2 class="text-xl font-semibold">Users</h2>
			{#if users.length === 0}
				<p>No users yet.</p>
			{:else}
				<table class="w-full border-collapse text-left">
					<thead>
						<tr class="border-b">
							<th class="py-2">Email</th>
							<th class="py-2">Databases</th>
							<th class="py-2">Created</th>
						</tr>
					</thead>
					<tbody>
						{#each users as u (u.id)}
							<tr class="border-b">
								<td class="py-2">{u.email}</td>
								<td class="py-2">{u.database_count}</td>
								<td class="py-2">{new Date(u.created_at).toLocaleString()}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>

		<section class="flex flex-col gap-2">
			<h2 class="text-xl font-semibold">Jobs</h2>
			{#if jobs.length === 0}
				<p>No jobs yet.</p>
			{:else}
				<table class="w-full border-collapse text-left">
					<thead>
						<tr class="border-b">
							<th class="py-2">Kind</th>
							<th class="py-2">Status</th>
							<th class="py-2">Attempts</th>
							<th class="py-2">Last error</th>
							<th class="py-2">Created</th>
						</tr>
					</thead>
					<tbody>
						{#each jobs as job (job.id)}
							<tr class="border-b">
								<td class="py-2">{job.kind}</td>
								<td class="py-2">{job.status}</td>
								<td class="py-2">{job.attempts}</td>
								<td class="py-2">{job.last_error ?? '-'}</td>
								<td class="py-2">{new Date(job.created_at).toLocaleString()}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>
	{/if}
</main>
