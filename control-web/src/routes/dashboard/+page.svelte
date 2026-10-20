<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { errorMessage, postJSON, type CreateDatabaseResult, type Database, type User } from '$lib/api';
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

	let user: User | null = $state(null);
	let databases: Database[] = $state([]);
	let loading: boolean = $state(true);
	let error: string = $state('');
	let endpoint: string = $state('/api');

	let createName: string = $state('');
	let createSizeGB: number = $state(1);
	let creating: boolean = $state(false);
	let createError: string = $state('');

	let createdID: string = $state('');
	let createdHost: string = $state('');
	let createdPort: number | null = $state(null);
	let createdDatabaseName: string = $state('');
	let createdUser: string = $state('');
	let createdPassword: string = $state('');
	let createdConnectionString: string = $state('');
	let createdJDBCURL: string = $state('');

	let deleteTarget: Database | null = $state(null);
	let actionTarget: string = $state('');

	const pollTimers = new Map<string, ReturnType<typeof setTimeout>>();

	const monthlyPriceIDR = $derived(createSizeGB * 10000);
	const createdDatabase = $derived(databases.find((d) => d.id === createdID) ?? null);
	const connectionString = $derived(createdDatabase?.status === 'running' ? createdConnectionString : '');
	const jdbcURL = $derived(createdDatabase?.status === 'running' ? createdJDBCURL : '');
	const databaseCount = $derived(databases.length);
	const provisionedGB = $derived(databases.reduce((sum, d) => sum + d.size_gb, 0));

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
		endpoint = `${location.host}/api`;
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
			createdHost = result.host;
			createdPort = result.port;
			createdDatabaseName = result.database;
			createdUser = result.user;
			createdPassword = result.password;
			createdConnectionString = result.connection_string;
			createdJDBCURL = result.jdbc_url;
			createName = '';
			createSizeGB = 1;
			await loadDatabases();
		} finally {
			creating = false;
		}
	}

	function dismissCreated() {
		createdID = '';
		createdHost = '';
		createdPort = null;
		createdDatabaseName = '';
		createdUser = '';
		createdPassword = '';
		createdConnectionString = '';
		createdJDBCURL = '';
	}

	function requestDelete(db: Database) {
		deleteTarget = db;
	}

	async function confirmDelete() {
		const db = deleteTarget;
		if (!db) return;
		deleteTarget = null;
		actionTarget = db.id;
		try {
			const res = await fetch(`/api/databases/${db.id}`, { method: 'DELETE' });
			if (!res.ok) {
				pushToast('error', await errorMessage(res));
				return;
			}
			databases = databases.filter((d) => d.id !== db.id);
			if (db.id === createdID) dismissCreated();
			const timer = pollTimers.get(db.id);
			if (timer) {
				clearTimeout(timer);
				pollTimers.delete(db.id);
			}
			pushToast('success', `Deleted "${db.name}".`);
		} finally {
			actionTarget = '';
		}
	}
</script>

<TopBar
	links={[
		{ href: '/billing', label: 'Billing' },
		...(user?.is_admin ? [{ href: '/admin', label: 'Admin' }] : [])
	]}
	userEmail={user?.email}
	onLogout={handleLogout}
/>
<StatusRail apiState="reachable" {endpoint} {databaseCount} {provisionedGB} dense />

<main class="mx-auto flex max-w-3xl flex-col gap-6 px-4 py-6">
	{#if loading}
		<p class="text-muted">Loading...</p>
	{:else if error}
		<p class="text-sm text-error">{error}</p>
	{:else}
		<Panel title="New database">
			<form class="flex flex-wrap items-end gap-3" onsubmit={handleCreate}>
				<Field label="Name" id="create-name">
					<Input id="create-name" type="text" bind:value={createName} required />
				</Field>
				<Field label="Size (GB)" id="create-size">
					<Input id="create-size" type="number" min={1} max={20} bind:value={createSizeGB} required />
				</Field>
				<p class="pb-3 text-sm text-muted">~ IDR {monthlyPriceIDR.toLocaleString('id-ID')} / month</p>
				<Button type="submit" disabled={creating}>Create</Button>
			</form>

			{#if createError}
				<p class="text-sm text-error">{createError}</p>
			{/if}

			{#if createdID}
				<div class="flex flex-col gap-2 rounded-md border border-warning bg-bg p-3 text-sm">
					<p>Database created. This password is shown once and will not be shown again.</p>
					<CopyField label="Host" value={createdHost} />
					<CopyField label="Port" value={String(createdPort ?? '')} />
					<CopyField label="Database" value={createdDatabaseName} />
					<CopyField label="User" value={createdUser} />
					<CopyField label="Password" value={createdPassword} />
					{#if connectionString}
						<CopyField label="Connection string" value={connectionString} />
						<p>DBeaver / JDBC URL (DBeaver does not accept the connection string above):</p>
						<CopyField label="JDBC URL" value={jdbcURL} />
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
					{:else if createdDatabase?.status === 'failed'}
						<p class="text-error">Provisioning failed. Delete this database and try again.</p>
					{:else}
						<Status state="provisioning" label="Provisioning... the connection string will appear here once running." />
					{/if}
					<Button variant="ghost" size="sm" onclick={dismissCreated}>Dismiss</Button>
				</div>
			{/if}
		</Panel>

		<Panel title="Your databases">
			{#if databases.length === 0}
				<EmptyState message="No databases yet." />
			{:else}
				<Table>
					<thead>
						<tr>
							<th>Name</th>
							<th>Size (GB)</th>
							<th>Status</th>
							<th>Host</th>
							<th>Created</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each databases as db (db.id)}
							<tr>
								<td>{db.name}</td>
								<td>{db.size_gb}</td>
								<td><Status state={db.status} /></td>
								<td class="font-mono text-xs">{db.public_host ? `${db.public_host}:5432` : '-'}</td>
								<td>{new Date(db.created_at).toLocaleString()}</td>
								<td>
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
								</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			{/if}
		</Panel>
	{/if}
</main>

<ConfirmDialog
	open={deleteTarget !== null}
	title="Delete database"
	message={`Delete database "${deleteTarget?.name}"? This cannot be undone.`}
	confirmLabel="Delete"
	onconfirm={confirmDelete}
	oncancel={() => (deleteTarget = null)}
/>
