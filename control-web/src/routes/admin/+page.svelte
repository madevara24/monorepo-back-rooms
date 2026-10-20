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
	import { pushToast } from '$lib/toast.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import StatusRail from '$lib/components/StatusRail.svelte';
	import Panel from '$lib/components/Panel.svelte';
	import Field from '$lib/components/Field.svelte';
	import Input from '$lib/components/Input.svelte';
	import Button from '$lib/components/Button.svelte';
	import Table from '$lib/components/Table.svelte';
	import Status from '$lib/components/Status.svelte';
	import CopyField from '$lib/components/CopyField.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

	const dokployURL = publicEnv.PUBLIC_DOKPLOY_URL ?? '';

	let loading: boolean = $state(true);
	let error: string = $state('');
	let endpoint: string = $state('/api');

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

	let actionTarget: string = $state('');
	let confirmAction: { kind: 'delete' | 'retry'; db: AdminDatabase } | null = $state(null);

	const databaseCount = $derived(databases.length);
	const provisionedGB = $derived(databases.reduce((sum, d) => sum + d.size_gb, 0));

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
		endpoint = `${location.host}/api`;
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

	function requestDelete(db: AdminDatabase) {
		confirmAction = { kind: 'delete', db };
	}

	function requestRetry(db: AdminDatabase) {
		confirmAction = { kind: 'retry', db };
	}

	async function confirmPending() {
		const action = confirmAction;
		if (!action) return;
		confirmAction = null;
		const { kind, db } = action;
		actionTarget = db.id;
		try {
			const res =
				kind === 'delete'
					? await fetch(`/api/admin/databases/${db.id}`, { method: 'DELETE' })
					: await postJSON(`/api/admin/databases/${db.id}/retry`);
			if (!res.ok) {
				pushToast('error', await errorMessage(res));
				return;
			}
			await reloadDatabasesAndOverview();
			pushToast('success', kind === 'delete' ? `Deleted "${db.name}".` : `Retrying "${db.name}".`);
		} finally {
			actionTarget = '';
		}
	}
</script>

<TopBar
	links={[
		...(dokployURL ? [{ href: dokployURL, label: 'Dokploy' }] : []),
		{ href: '/dashboard', label: 'Dashboard' }
	]}
/>
<StatusRail apiState="reachable" {endpoint} {databaseCount} {provisionedGB} dense />

<main class="mx-auto flex max-w-5xl flex-col gap-6 px-4 py-6">
	{#if loading}
		<p class="text-muted">Loading...</p>
	{:else if error}
		<p class="text-sm text-error">{error}</p>
	{:else}
		<Panel title="Overview">
			{#if overview}
				<Table>
					<tbody>
						<tr>
							<th>Users</th>
							<td>{overview.users}</td>
						</tr>
						<tr>
							<th>Databases by status</th>
							<td>{formatCounts(overview.databases_by_status)}</td>
						</tr>
						<tr>
							<th>Nodes</th>
							<td>{overview.nodes_fresh} fresh, {overview.nodes_stale} stale</td>
						</tr>
						<tr>
							<th>Jobs by status</th>
							<td>{formatCounts(overview.jobs_by_status)}</td>
						</tr>
					</tbody>
				</Table>
			{/if}
		</Panel>

		<Panel title="Nodes">
			{#if nodes.length === 0}
				<EmptyState message="No nodes yet." />
			{:else}
				<Table>
					<thead>
						<tr>
							<th>ID</th>
							<th>Version</th>
							<th>Last heartbeat</th>
							<th>Capacity</th>
							<th>Containers</th>
						</tr>
					</thead>
					<tbody>
						{#each nodes as node (node.id)}
							<tr>
								<td class="font-mono text-xs">{node.id}</td>
								<td class="font-mono text-xs">{node.version || '-'}</td>
								<td>{node.last_heartbeat ? new Date(node.last_heartbeat).toLocaleString() : 'never'}</td>
								<td class="font-mono text-xs">{JSON.stringify(node.capacity)}</td>
								<td>
									{#if node.containers.length === 0}
										<span class="text-muted">none</span>
									{:else}
										<ul class="flex flex-col gap-1">
											{#each node.containers as container (container.database_id)}
												<li class="font-mono text-xs">
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
				</Table>
			{/if}
		</Panel>

		<Panel title="Databases">
			<form class="flex flex-wrap items-end gap-3" onsubmit={handleCreate}>
				<Field label="User" id="admin-create-user">
					<select
						id="admin-create-user"
						class="h-11 w-full min-w-0 rounded-md border border-line bg-surface px-3 text-sm text-text"
						bind:value={createUserId}
						required
					>
						{#each users as u (u.id)}
							<option value={u.id}>{u.email}</option>
						{/each}
					</select>
				</Field>
				<Field label="Name" id="admin-create-name">
					<Input id="admin-create-name" type="text" bind:value={createName} required />
				</Field>
				<Field label="Size (GB)" id="admin-create-size">
					<Input id="admin-create-size" type="number" min={1} max={20} bind:value={createSizeGB} required />
				</Field>
				<Button type="submit" disabled={creating || users.length === 0}>Create</Button>
			</form>

			{#if createError}
				<p class="text-sm text-error">{createError}</p>
			{/if}

			{#if created}
				<div class="flex flex-col gap-2 rounded-md border border-warning bg-bg p-3 text-sm">
					<p>Database created. This password is shown once and will not be shown again.</p>
					<CopyField label="Host" value={created.host} />
					<CopyField label="Port" value={String(created.port)} />
					<CopyField label="Database" value={created.database} />
					<CopyField label="User" value={created.user} />
					<CopyField label="Password" value={created.password} />
					<CopyField label="Connection string" value={created.connection_string} />
					<p>DBeaver / JDBC URL (DBeaver does not accept the connection string above):</p>
					<CopyField label="JDBC URL" value={created.jdbc_url} />
					<p class="text-xs text-muted">
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
						<code>sslmode=verify-full&amp;sslrootcert=&lt;path to the downloaded PEM&gt;</code>.
					</p>
				</div>
			{/if}

			{#if databases.length === 0}
				<EmptyState message="No databases yet." />
			{:else}
				<Table>
					<thead>
						<tr>
							<th>Name</th>
							<th>Owner</th>
							<th>Status</th>
							<th>Node</th>
							<th>Host</th>
							<th>Port</th>
							<th>Created</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each databases as db (db.id)}
							<tr>
								<td>{db.name}</td>
								<td>{db.owner_email}</td>
								<td><Status state={db.status} /></td>
								<td class="font-mono text-xs">{db.node_id ?? '-'}</td>
								<td class="font-mono text-xs">{db.host ?? '-'}</td>
								<td class="font-mono text-xs">{db.port ?? '-'}</td>
								<td>{new Date(db.created_at).toLocaleString()}</td>
								<td>
									<div class="flex gap-2">
										{#if db.status === 'running' || db.status === 'failed'}
											<Button
												variant="danger"
												size="sm"
												disabled={actionTarget === db.id}
												onclick={() => requestDelete(db)}
											>
												Delete
											</Button>
										{/if}
										{#if db.status === 'failed'}
											<Button
												variant="secondary"
												size="sm"
												disabled={actionTarget === db.id}
												onclick={() => requestRetry(db)}
											>
												Retry
											</Button>
										{/if}
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			{/if}
		</Panel>

		<Panel title="Users">
			{#if users.length === 0}
				<EmptyState message="No users yet." />
			{:else}
				<Table>
					<thead>
						<tr>
							<th>Email</th>
							<th>Databases</th>
							<th>Created</th>
						</tr>
					</thead>
					<tbody>
						{#each users as u (u.id)}
							<tr>
								<td>{u.email}</td>
								<td>{u.database_count}</td>
								<td>{new Date(u.created_at).toLocaleString()}</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			{/if}
		</Panel>

		<Panel title="Jobs">
			{#if jobs.length === 0}
				<EmptyState message="No jobs yet." />
			{:else}
				<Table>
					<thead>
						<tr>
							<th>Kind</th>
							<th>Status</th>
							<th>Attempts</th>
							<th>Last error</th>
							<th>Created</th>
						</tr>
					</thead>
					<tbody>
						{#each jobs as job (job.id)}
							<tr>
								<td>{job.kind}</td>
								<td><Status state={job.status} /></td>
								<td>{job.attempts}</td>
								<td class="font-mono text-xs">{job.last_error ?? '-'}</td>
								<td>{new Date(job.created_at).toLocaleString()}</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			{/if}
		</Panel>
	{/if}
</main>

<ConfirmDialog
	open={confirmAction !== null}
	title={confirmAction?.kind === 'delete' ? 'Delete database' : 'Retry database'}
	message={confirmAction?.kind === 'delete'
		? `Delete database "${confirmAction.db.name}" (owned by ${confirmAction.db.owner_email})? This cannot be undone.`
		: `Retry database "${confirmAction?.db.name}" (owned by ${confirmAction?.db.owner_email})?`}
	confirmLabel={confirmAction?.kind === 'delete' ? 'Delete' : 'Retry'}
	danger={confirmAction?.kind === 'delete'}
	onconfirm={confirmPending}
	oncancel={() => (confirmAction = null)}
/>
