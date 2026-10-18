<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { errorMessage, postJSON, type Invoice } from '$lib/api';

	let invoices: Invoice[] = $state([]);
	let loading: boolean = $state(true);
	let error: string = $state('');
	let payingID: string = $state('');

	async function loadInvoices() {
		const res = await fetch('/api/billing/invoices');
		if (!res.ok) {
			error = `could not load invoices (${res.status})`;
			return;
		}
		invoices = await res.json();
	}

	onMount(async () => {
		try {
			const meRes = await fetch('/api/me');
			if (!meRes.ok) {
				await goto('/login');
				return;
			}
			await loadInvoices();
		} finally {
			loading = false;
		}
	});

	async function handlePay(invoice: Invoice) {
		error = '';
		payingID = invoice.id;
		try {
			const res = await postJSON(`/api/billing/invoices/${invoice.id}/pay`);
			if (!res.ok) {
				error = await errorMessage(res);
				return;
			}
			await loadInvoices();
		} finally {
			payingID = '';
		}
	}

	function formatIDR(amount: number): string {
		return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(amount);
	}
</script>

<main class="mx-auto flex min-h-screen max-w-2xl flex-col gap-6 p-6">
	<header class="flex items-center justify-between">
		<h1 class="text-2xl font-bold">Billing</h1>
		<a class="underline" href="/dashboard">Dashboard</a>
	</header>

	{#if loading}
		<p>Loading...</p>
	{:else}
		{#if error}
			<p class="text-sm text-red-600">{error}</p>
		{/if}

		{#if invoices.length === 0}
			<p>No invoices yet.</p>
		{:else}
			<table class="w-full border-collapse text-left">
				<thead>
					<tr class="border-b">
						<th class="py-2">Period</th>
						<th class="py-2">Amount</th>
						<th class="py-2">Status</th>
						<th class="py-2"></th>
					</tr>
				</thead>
				<tbody>
					{#each invoices as invoice (invoice.id)}
						<tr class="border-b">
							<td class="py-2">{invoice.period_start} &ndash; {invoice.period_end}</td>
							<td class="py-2">{formatIDR(invoice.total_amount_idr)}</td>
							<td class="py-2">{invoice.status}</td>
							<td class="py-2">
								{#if invoice.status === 'open'}
									<button
										class="rounded bg-black px-3 py-1 text-white disabled:opacity-50"
										disabled={payingID === invoice.id}
										onclick={() => handlePay(invoice)}
									>
										Pay
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	{/if}
</main>
