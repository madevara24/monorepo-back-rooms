<script lang="ts">
	import { onMount } from 'svelte';
	import type { Plan } from '$lib/api';
	import StatusRail from '$lib/components/StatusRail.svelte';
	import Panel from '$lib/components/Panel.svelte';
	import Button from '$lib/components/Button.svelte';
	import Status from '$lib/components/Status.svelte';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';

	let apiState: 'reachable' | 'unreachable' | 'checking' = $state('checking');
	let endpoint: string = $state('/api');
	let signedIn: boolean = $state(false);
	let isAdmin: boolean = $state(false);
	let databaseCount: number | null = $state(null);
	let provisionedGB: number | null = $state(null);
	let plans: Plan[] = $state([]);
	let plansState: 'loading' | 'ready' | 'error' = $state('loading');

	const lifecycle = [
		{ step: '1', text: 'A create request writes a database row and a job.' },
		{ step: '2', text: 'The worker calls the data-plane agent with the wanted state.' },
		{ step: '3', text: 'The agent creates a sized volume, a limited container, a password, and a TLS certificate.' },
		{ step: '4', text: 'The agent reports running; the connection string is shown once.' },
		{
			step: '5',
			text: 'The agent reports container metrics on each heartbeat. Snapshots and restore are not implemented in this build.'
		}
	];

	async function checkHealth() {
		try {
			const res = await fetch('/healthz');
			const body = await res.json();
			apiState = body.status === 'ok' ? 'reachable' : 'unreachable';
		} catch {
			apiState = 'unreachable';
		}
	}

	async function loadPlans() {
		plansState = 'loading';
		try {
			const res = await fetch('/api/plans');
			if (!res.ok) throw new Error('could not load plans');
			plans = await res.json();
			plansState = 'ready';
		} catch {
			plansState = 'error';
		}
	}

	onMount(async () => {
		endpoint = `${location.host}/api`;
		await checkHealth();
		await loadPlans();
		try {
			const meRes = await fetch('/api/me');
			signedIn = meRes.ok;
			if (meRes.ok) {
				const me = await meRes.json();
				isAdmin = me.is_admin === true;
				const dbRes = await fetch('/api/databases');
				if (dbRes.ok) {
					const databases: { size_gb: number }[] = await dbRes.json();
					databaseCount = databases.length;
					provisionedGB = databases.reduce((sum, d) => sum + d.size_gb, 0);
				}
			}
		} catch {
			signedIn = false;
		}
	});
</script>

<div class="mx-auto flex min-h-screen max-w-3xl flex-col gap-8 px-4 py-6">
	<header class="flex items-center justify-between">
		<span class="text-lg font-semibold text-text">MBR</span>
		<ThemeToggle />
	</header>

	<StatusRail {apiState} {endpoint} {databaseCount} {provisionedGB} />

	<section class="flex flex-col gap-3">
		<h1 class="text-3xl font-semibold text-text">A self-serve Postgres marketplace.</h1>
		<p class="text-muted">
			Sign up, pick a package, get a running Postgres with TLS and a connection string. Payment
			is mocked in this build.
		</p>
		<nav class="flex flex-wrap gap-3">
			<a href="/"><Button variant="secondary">See packages</Button></a>
			{#if signedIn}
				<a href="/dashboard"><Button>Go to dashboard</Button></a>
				{#if isAdmin}
					<a href="/admin"><Button variant="secondary">Admin</Button></a>
				{/if}
			{:else}
				<a href="/signup"><Button>Create an account</Button></a>
				<a href="/login"><Button variant="secondary">Log in</Button></a>
			{/if}
		</nav>
	</section>

	<Panel title="Sample database">
		<p class="text-xs font-medium tracking-wide text-muted uppercase">Sample — not live data</p>
		<div class="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-4">
			<div>
				<p class="text-muted">Name</p>
				<p class="font-mono">orders-db</p>
			</div>
			<div>
				<p class="text-muted">Size</p>
				<p class="font-mono">5 GB</p>
			</div>
			<div>
				<p class="text-muted">Status</p>
				<Status state="running" />
			</div>
			<div>
				<p class="text-muted">Host</p>
				<p class="font-mono break-all">orders-db.db.mbr.madevara.com:5432</p>
			</div>
		</div>
	</Panel>

	<Panel title="Prices">
		{#if plansState === 'loading'}
			<p class="text-muted">Loading prices...</p>
		{:else if plansState === 'error' || plans.length === 0}
			<p class="text-muted">Prices are unavailable right now.</p>
		{:else}
			<table class="w-full text-left text-sm">
				<thead>
					<tr class="border-b border-line text-muted">
						<th class="py-1 font-medium">Package</th>
						<th class="py-1 font-medium">Size</th>
						<th class="py-1 font-medium">Monthly</th>
					</tr>
				</thead>
				<tbody>
					{#each plans as plan (plan.id)}
						<tr class="border-b border-line last:border-none">
							<td class="py-1">{plan.name}</td>
							<td class="py-1">{plan.size_gb} GB</td>
							<td class="py-1 font-mono">IDR {plan.price_idr_month.toLocaleString('id-ID')}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</Panel>

	<Panel title="One database's life">
		<ol class="flex flex-col gap-2 text-sm">
			{#each lifecycle as item (item.step)}
				<li class="flex gap-3">
					<span class="font-mono text-muted">{item.step}</span>
					<span class="text-text">{item.text}</span>
				</li>
			{/each}
		</ol>
	</Panel>
</div>
