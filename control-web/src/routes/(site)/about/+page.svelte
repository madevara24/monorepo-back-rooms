<script lang="ts">
	import { onMount, getContext } from 'svelte';
	import type { Plan } from '$lib/api';
	import Panel from '$lib/components/Panel.svelte';
	import Button from '$lib/components/Button.svelte';
	import Status from '$lib/components/Status.svelte';
	import { SESSION_CONTEXT_KEY, type Session } from '$lib/session.svelte';

	const session = getContext<Session>(SESSION_CONTEXT_KEY);

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

	onMount(() => {
		loadPlans();
	});
</script>

<div class="mx-auto flex min-h-screen max-w-3xl flex-col gap-8 px-4 py-6">
	<section class="flex flex-col gap-3">
		<h1 class="text-3xl font-semibold text-text">A self-serve Postgres marketplace.</h1>
		<p class="text-muted">
			Sign up, pick a package, get a running Postgres with TLS and a connection string. Payment
			is mocked in this build.
		</p>
		<nav class="flex flex-wrap gap-3">
			<a href="/"><Button variant="secondary">See packages</Button></a>
			{#if session.signedIn}
				<a href="/dashboard"><Button>Go to dashboard</Button></a>
				{#if session.isAdmin}
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
