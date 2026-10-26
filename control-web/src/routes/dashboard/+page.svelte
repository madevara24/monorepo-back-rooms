<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { goto, replaceState } from '$app/navigation';
	import { page } from '$app/stores';
	import {
		errorMessage,
		postJSON,
		type CreateDatabaseResult,
		type Database,
		type DemoStatus,
		type Plan,
		type User
	} from '$lib/api';
	import { pushToast } from '$lib/toast.svelte';
	import { dashboardTour } from '$lib/tour.svelte';
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
	import DemoCountdown from '$lib/components/DemoCountdown.svelte';

	let user: User | null = $state(null);
	let databases: Database[] = $state([]);
	let loading: boolean = $state(true);
	let error: string = $state('');
	let endpoint: string = $state('/api');

	let demoStatus = $state<DemoStatus | null>(null);
	let demoNow: number = $state(Date.now());
	let demoTimers: ReturnType<typeof setInterval>[] = [];

	let createName: string = $state('');
	let plans: Plan[] = $state([]);
	let selectedPlanId: string = $state('');
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
	let capturedTour: string | null = null;

	const selectedPlan = $derived(plans.find((p) => p.id === selectedPlanId) ?? null);
	const createdDatabase = $derived(databases.find((d) => d.id === createdID) ?? null);
	const connectionString = $derived(createdDatabase?.status === 'running' ? createdConnectionString : '');
	const jdbcURL = $derived(createdDatabase?.status === 'running' ? createdJDBCURL : '');
	const databaseCount = $derived(databases.length);
	const provisionedGB = $derived(databases.reduce((sum, d) => sum + d.size_gb, 0));
	const maintenance = $derived(demoStatus?.maintenance ?? false);
	const resetInSeconds = $derived(
		demoStatus
			? Math.max(0, Math.floor((new Date(demoStatus.next_reset_at).getTime() - demoNow) / 1000))
			: null
	);
	const resetWarning = $derived(
		resetInSeconds !== null && resetInSeconds > 0 && resetInSeconds <= 120
	);
	const atLimit = $derived(databases.length >= 2);
	const purchasablePlans = $derived(plans.filter((p) => p.purchasable));
	const nextResetLabel = $derived(
		demoStatus
			? new Date(demoStatus.next_reset_at).toLocaleTimeString('en-GB', {
					hour: '2-digit',
					minute: '2-digit',
					timeZone: 'Asia/Jakarta'
				})
			: ''
	);

	async function loadDemoStatus() {
		try {
			const res = await fetch('/api/demo/status');
			if (!res.ok) {
				if (requireAuth(res)) return;
				demoStatus = null;
				return;
			}
			demoStatus = await res.json();
		} catch {
			demoStatus = null;
		}
	}

	function requireAuth(res: Response): boolean {
		if (res.status !== 401) return false;
		dashboardTour.clear();
		void goto('/login?reset=1&tour=1');
		return true;
	}

	$effect(() => {
		if (maintenance) {
			dashboardTour.pause();
		} else {
			dashboardTour.resume();
		}
	});

	function pollDatabase(id: string) {
		if (pollTimers.has(id)) return;
		const timer = setTimeout(async () => {
			pollTimers.delete(id);
			const res = await fetch(`/api/databases/${id}`);
			if (!res.ok) {
				if (requireAuth(res)) return;
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
			if (requireAuth(res)) return;
			error = `could not load databases (${res.status})`;
			return;
		}
		databases = await res.json();
		for (const db of databases) {
			if (db.status === 'provisioning') pollDatabase(db.id);
		}
	}

	async function loadPlans() {
		const res = await fetch('/api/plans');
		if (!res.ok) {
			requireAuth(res);
			return;
		}
		plans = await res.json();
		const wantedSlug = $page.url.searchParams.get('plan');
		const wanted = wantedSlug
			? plans.find((p) => p.slug === wantedSlug && p.purchasable)
			: null;
		selectedPlanId = (wanted ?? plans.find((p) => p.purchasable))?.id ?? '';
	}

	function packageLabel(db: Database): string {
		const plan = plans.find((p) => p.id === db.plan_id);
		return plan ? `${plan.name} · ${db.size_gb} GB` : `${db.size_gb} GB`;
	}

	onMount(async () => {
		endpoint = `${location.host}/api`;

		const params = new URLSearchParams(location.search);
		capturedTour = params.get('tour');
		if (params.has('tour')) {
			params.delete('tour');
			const query = params.toString();
			replaceState(`${location.pathname}${query ? `?${query}` : ''}`, {});
		}

		try {
			const meRes = await fetch('/api/me');
			if (!meRes.ok) {
				if (!requireAuth(meRes)) await goto('/login');
				return;
			}
			user = await meRes.json();
			await loadDemoStatus();
			demoTimers.push(setInterval(() => (demoNow = Date.now()), 1000));
			demoTimers.push(setInterval(loadDemoStatus, 15000));
			await loadPlans();
			await loadDatabases();
		} finally {
			loading = false;
			if (user && !error && capturedTour === '1') {
				dashboardTour.start();
			}
		}
	});

	onDestroy(() => {
		for (const timer of pollTimers.values()) clearTimeout(timer);
		pollTimers.clear();
		for (const timer of demoTimers) clearInterval(timer);
		demoTimers = [];
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
			const res = await postJSON('/api/databases', { name: createName, plan_id: selectedPlanId });
			if (!res.ok) {
				if (requireAuth(res)) return;
				if (res.status === 503) {
					await loadDemoStatus();
					createError = 'The demo is resetting. Try again shortly.';
					return;
				}
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
			selectedPlanId = plans[0]?.id ?? '';
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
				if (requireAuth(res)) return;
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
<div data-tour="dashboard-status">
	<StatusRail apiState="reachable" {endpoint} {databaseCount} {provisionedGB} dense />
	<div class="mx-auto max-w-3xl px-4 pt-3">
		<DemoCountdown />
	</div>
</div>

<main class="mx-auto flex max-w-3xl flex-col gap-6 px-4 py-6">
	{#if loading}
		<p class="text-muted">Loading...</p>
	{:else if error}
		<p class="text-sm text-error">{error}</p>
	{:else}
		<Panel title="New database" tourId="create-panel">
			<p class="text-xs text-muted">
				{databases.length} of 2 databases. The next reset deletes all of them.
			</p>
			{#if maintenance}
				<p class="text-sm text-warning">The demo is resetting. The form will unlock when it finishes.</p>
			{:else if resetWarning}
				<p class="text-sm text-warning">The demo resets in under two minutes.</p>
			{/if}
			<form class="flex flex-wrap items-end gap-3" data-tour="create-form" onsubmit={handleCreate}>
				<Field label="Name" id="create-name">
					<Input id="create-name" type="text" bind:value={createName} required />
				</Field>
				{#if plans.length === 0}
					<p class="text-muted">No packages available.</p>
				{:else}
					<Field label="Package" id="create-plan">
						<select
							id="create-plan"
							bind:value={selectedPlanId}
							required
							class="h-11 w-full min-w-0 rounded-md border border-line bg-surface px-3 text-sm text-text focus-visible:border-accent"
						>
							{#each plans as plan (plan.id)}
								<option value={plan.id} disabled={!plan.purchasable}>
									{plan.name} · {plan.size_gb} GB{plan.purchasable ? '' : ' (sold out)'}
								</option>
							{/each}
						</select>
					</Field>
					<p class="pb-3 text-sm text-muted">
						{#if selectedPlan}
							IDR {selectedPlan.price_idr_month.toLocaleString('id-ID')} / month
						{/if}
					</p>
				{/if}
				<Button
					type="submit"
					disabled={creating || maintenance || atLimit || purchasablePlans.length === 0}
				>
					Create
				</Button>
			</form>

			{#if createError}
				<p class="text-sm text-error">{createError}</p>
			{/if}

			{#if createdID}
				<div class="flex flex-col gap-2 rounded-md border border-warning bg-bg p-3 text-sm">
					<p>Database created. Copy the password now; the dashboard will not show it again.</p>
					{#if nextResetLabel}
						<p class="text-xs text-muted">The password works until {nextResetLabel} GMT+7.</p>
					{/if}
					<CopyField label="Host" value={createdHost} />
					<CopyField label="Port" value={String(createdPort ?? '')} />
					<CopyField label="Database" value={createdDatabaseName} />
					<CopyField label="User" value={createdUser} />
					<CopyField label="Password" value={createdPassword} />
					{#if connectionString}
						<CopyField label="Connection string" value={connectionString} />
						<CopyField label="JDBC URL" value={jdbcURL} />
					{:else if createdDatabase?.status === 'failed'}
						<p class="text-error">Provisioning failed. Delete this database and try again.</p>
					{:else}
						<Status state="provisioning" label="Provisioning. The connection string appears when the database is ready." />
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
							<th>Package</th>
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
								<td>{packageLabel(db)}</td>
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
