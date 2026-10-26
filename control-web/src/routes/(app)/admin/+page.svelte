<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { env as publicEnv } from '$env/dynamic/public';
	import {
		errorMessage,
		postJSON,
		patchJSON,
		type AdminDatabase,
		type AdminDatabaseMetric,
		type AdminJob,
		type AdminNode,
		type AdminOverview,
		type AdminPlan,
		type AdminUser,
		type CreateDatabaseResult
	} from '$lib/api';
	import { pushToast } from '$lib/toast.svelte';
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
	import Dialog from '$lib/components/Dialog.svelte';

	const dokployURL = publicEnv.PUBLIC_DOKPLOY_URL ?? '';

	let loading: boolean = $state(true);
	let error: string = $state('');
	let endpoint: string = $state('/api');

	let overview: AdminOverview | null = $state(null);
	let users: AdminUser[] = $state([]);
	let databases: AdminDatabase[] = $state([]);
	let nodes: AdminNode[] = $state([]);
	let jobs: AdminJob[] = $state([]);
	let databaseMetrics: AdminDatabaseMetric[] = $state([]);
	let metricsError: string = $state('');
	let plans: AdminPlan[] = $state([]);

	let createUserId: string = $state('');
	let createName: string = $state('');
	let createSizeGB: number = $state(1);
	let creating: boolean = $state(false);
	let createError: string = $state('');
	let created: CreateDatabaseResult | null = $state(null);

	let actionTarget: string = $state('');
	let confirmAction: { kind: 'delete' | 'retry'; db: AdminDatabase } | null = $state(null);
	let confirmReset: boolean = $state(false);

	let planDialogOpen: boolean = $state(false);
	let editingPlan: AdminPlan | null = $state(null);
	let planSlug: string = $state('');
	let planName: string = $state('');
	let planSizeGB: number = $state(1);
	let planPriceIDRMonth: number = $state(0);
	let planTagline: string = $state('');
	let planBadge: string = $state('');
	let planSortOrder: number = $state(0);
	let planPurchasable: boolean = $state(true);
	let planSaving: boolean = $state(false);

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

			[overview, users, databases, nodes, jobs, databaseMetrics, plans] = await Promise.all([
				loadJSON<AdminOverview>('/api/admin/overview'),
				loadJSON<AdminUser[]>('/api/admin/users'),
				loadJSON<AdminDatabase[]>('/api/admin/databases'),
				loadJSON<AdminNode[]>('/api/admin/nodes'),
				loadJSON<AdminJob[]>('/api/admin/jobs'),
				loadJSON<AdminDatabaseMetric[]>('/api/admin/database-metrics'),
				loadJSON<AdminPlan[]>('/api/admin/plans')
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

	async function reloadMetrics() {
		metricsError = '';
		try {
			databaseMetrics = await loadJSON<AdminDatabaseMetric[]>('/api/admin/database-metrics');
		} catch (e) {
			metricsError = e instanceof Error ? e.message : 'could not load database metrics';
		}
	}

	function warningBadgeClass(level: string): string {
		if (level === 'critical') return 'rounded px-2 py-1 text-xs font-semibold text-[var(--error)]';
		if (level === 'warning') return 'rounded px-2 py-1 text-xs font-semibold text-[var(--warning)]';
		return 'rounded px-2 py-1 text-xs font-semibold text-[var(--accent)]';
	}
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

	async function resetDemo() {
		confirmReset = false;
		const res = await postJSON('/api/admin/demo/reset');
		if (!res.ok) {
			pushToast('error', await errorMessage(res));
			return;
		}
		location.reload();
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

	async function reloadPlans() {
		plans = await loadJSON<AdminPlan[]>('/api/admin/plans');
	}

	function openCreatePlanDialog() {
		editingPlan = null;
		planSlug = '';
		planName = '';
		planSizeGB = 1;
		planPriceIDRMonth = 10000;
		planTagline = '';
		planBadge = '';
		planSortOrder = 0;
		planPurchasable = true;
		planDialogOpen = true;
	}

	function openEditPlanDialog(plan: AdminPlan) {
		editingPlan = plan;
		planSlug = plan.slug;
		planName = plan.name;
		planSizeGB = plan.size_gb;
		planPriceIDRMonth = plan.price_idr_month;
		planTagline = plan.tagline;
		planBadge = plan.badge;
		planSortOrder = plan.sort_order;
		planPurchasable = plan.purchasable;
		planDialogOpen = true;
	}

	function closePlanDialog() {
		planDialogOpen = false;
	}

	async function submitPlanForm(event: SubmitEvent) {
		event.preventDefault();
		planSaving = true;
		try {
			const res = editingPlan
				? await patchJSON(`/api/admin/plans/${editingPlan.id}`, {
						name: planName,
						size_gb: planSizeGB,
						price_idr_month: planPriceIDRMonth,
						tagline: planTagline,
						badge: planBadge,
						sort_order: planSortOrder,
						purchasable: planPurchasable
					})
				: await postJSON('/api/admin/plans', {
						slug: planSlug,
						name: planName,
						size_gb: planSizeGB,
						price_idr_month: planPriceIDRMonth,
						tagline: planTagline,
						badge: planBadge,
						sort_order: planSortOrder,
						purchasable: planPurchasable
					});
			if (!res.ok) {
				pushToast('error', await errorMessage(res));
				return;
			}
			planDialogOpen = false;
			await reloadPlans();
			pushToast('success', editingPlan ? `Updated "${planName}".` : `Created "${planName}".`);
		} finally {
			planSaving = false;
		}
	}

	async function togglePlanActive(plan: AdminPlan) {
		actionTarget = plan.id;
		try {
			const res = await patchJSON(`/api/admin/plans/${plan.id}`, { active: !plan.active });
			if (!res.ok) {
				pushToast('error', await errorMessage(res));
				return;
			}
			await reloadPlans();
			pushToast('success', plan.active ? `Deactivated "${plan.name}".` : `Activated "${plan.name}".`);
		} finally {
			actionTarget = '';
		}
	}
</script>

<StatusRail apiState="reachable" {endpoint} {databaseCount} {provisionedGB} dense />
<div class="mx-auto flex max-w-5xl items-center justify-between px-4 pt-3">
	{#if dokployURL}
		<a href={dokployURL} class="text-sm text-text underline hover:text-accent">Dokploy</a>
	{:else}
		<span></span>
	{/if}
	<Button variant="danger" size="sm" onclick={() => (confirmReset = true)}>Reset demo</Button>
</div>

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

		<Panel title="Packages">
			<div class="mb-2 flex justify-end">
				<Button size="sm" onclick={openCreatePlanDialog}>New package</Button>
			</div>
			{#if plans.length === 0}
				<EmptyState message="No packages yet." />
			{:else}
				<Table>
					<thead>
						<tr>
							<th>Name</th>
							<th>Slug</th>
							<th>Size (GB)</th>
							<th>Price / month</th>
							<th>Badge</th>
							<th>Order</th>
							<th>Active</th>
							<th>Purchasable</th>
							<th>Databases</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each plans as plan (plan.id)}
							<tr class={plan.active ? '' : 'opacity-50'}>
								<td>{plan.name}</td>
								<td class="font-mono text-xs">{plan.slug}</td>
								<td>{plan.size_gb}</td>
								<td>Rp {plan.price_idr_month.toLocaleString('id-ID')}</td>
								<td>{plan.badge || '-'}</td>
								<td>{plan.sort_order}</td>
								<td>{plan.active ? 'active' : 'inactive'}</td>
								<td>{plan.purchasable ? 'buyable' : 'sold out'}</td>
								<td>{plan.database_count}</td>
								<td>
									<div class="flex gap-2">
										<Button variant="secondary" size="sm" onclick={() => openEditPlanDialog(plan)}>
											Edit
										</Button>
										<Button
											variant={plan.active ? 'danger' : 'secondary'}
											size="sm"
											disabled={actionTarget === plan.id}
											onclick={() => togglePlanActive(plan)}
										>
											{plan.active ? 'Deactivate' : 'Reactivate'}
										</Button>
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</Table>
			{/if}
		</Panel>

		<Panel title="Connection pressure">
			<div class="mb-2 flex justify-end">
				<Button variant="secondary" size="sm" onclick={reloadMetrics}>Refresh</Button>
			</div>
			{#if metricsError}
				<p class="text-sm text-[var(--error)]">{metricsError}</p>
			{/if}
			{#if databaseMetrics.length === 0}
				<EmptyState message="No metrics yet." />
			{:else}
				<Table>
					<thead>
						<tr>
							<th>Database</th>
							<th>Owner</th>
							<th>Used / Max</th>
							<th>%</th>
							<th>Active</th>
							<th>Idle</th>
							<th>Idle in txn</th>
							<th>Last collected</th>
							<th>Status</th>
						</tr>
					</thead>
					<tbody>
						{#each databaseMetrics as m (m.database_id)}
							<tr>
								<td>{m.database_name}</td>
								<td>{m.owner_email}</td>
								<td>{m.used} / {m.max_connections}</td>
								<td>{m.used_percent.toFixed(0)}%</td>
								<td>{m.active}</td>
								<td>{m.idle}</td>
								<td>{m.idle_in_transaction}</td>
								<td>{new Date(m.collected_at).toLocaleString()}</td>
								<td><span class={warningBadgeClass(m.warning_level)}>{m.warning_level}</span></td>
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

<ConfirmDialog
	open={confirmReset}
	title="Reset the demo"
	message="Delete every visitor account, database, invoice, and tenant container now?"
	confirmLabel="Reset"
	danger
	onconfirm={resetDemo}
	oncancel={() => (confirmReset = false)}
/>

<Dialog
	open={planDialogOpen}
	title={editingPlan ? 'Edit package' : 'New package'}
	onclose={closePlanDialog}
>
	<form class="flex flex-col gap-3" onsubmit={submitPlanForm}>
		<Field label="Name" id="plan-name">
			<Input id="plan-name" type="text" bind:value={planName} required />
		</Field>
		<Field label="Slug" id="plan-slug">
			{#if editingPlan}
				<Input id="plan-slug" type="text" value={planSlug} readonly />
			{:else}
				<Input id="plan-slug" type="text" bind:value={planSlug} required />
			{/if}
		</Field>
		<Field label="Size (GB)" id="plan-size">
			<Input id="plan-size" type="number" min={1} max={20} bind:value={planSizeGB} required />
		</Field>
		<Field label="Price / month" id="plan-price">
			<Input id="plan-price" type="number" min={1} bind:value={planPriceIDRMonth} required />
		</Field>
		<Field label="Tagline" id="plan-tagline">
			<Input id="plan-tagline" type="text" bind:value={planTagline} />
		</Field>
		<Field label="Badge" id="plan-badge">
			<Input id="plan-badge" type="text" bind:value={planBadge} />
		</Field>
		<Field label="Order" id="plan-order">
			<Input id="plan-order" type="number" bind:value={planSortOrder} required />
		</Field>
		<label class="flex items-center gap-2 text-sm text-text">
			<input type="checkbox" bind:checked={planPurchasable} />
			Purchasable
		</label>
		<div class="flex justify-end gap-2">
			<Button type="button" variant="secondary" onclick={closePlanDialog}>Cancel</Button>
			<Button type="submit" disabled={planSaving}>{editingPlan ? 'Save' : 'Create'}</Button>
		</div>
	</form>
</Dialog>
