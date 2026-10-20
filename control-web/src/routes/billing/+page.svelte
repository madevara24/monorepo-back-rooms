<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { errorMessage, postJSON, type Invoice } from '$lib/api';
	import { pushToast } from '$lib/toast.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import Panel from '$lib/components/Panel.svelte';
	import Table from '$lib/components/Table.svelte';
	import Status from '$lib/components/Status.svelte';
	import Button from '$lib/components/Button.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';

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
		payingID = invoice.id;
		try {
			const res = await postJSON(`/api/billing/invoices/${invoice.id}/pay`);
			if (!res.ok) {
				pushToast('error', await errorMessage(res));
				return;
			}
			await loadInvoices();
			pushToast('success', 'Invoice paid (mock).');
		} finally {
			payingID = '';
		}
	}

	function formatIDR(amount: number): string {
		return new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(amount);
	}
</script>

<TopBar links={[{ href: '/dashboard', label: 'Dashboard' }]} />

<main class="mx-auto flex max-w-2xl flex-col gap-6 px-4 py-6">
	<h1 class="text-2xl font-semibold text-text">Billing</h1>

	{#if loading}
		<p class="text-muted">Loading...</p>
	{:else}
		{#if error}
			<p class="text-sm text-error">{error}</p>
		{/if}

		<Panel title="Invoices">
			{#if invoices.length === 0}
				<EmptyState message="No invoices yet." />
			{:else}
				<Table>
					<thead>
						<tr>
							<th>Period</th>
							<th>Amount</th>
							<th>Status</th>
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each invoices as invoice (invoice.id)}
							<tr>
								<td>{invoice.period_start} &ndash; {invoice.period_end}</td>
								<td class="font-mono">{formatIDR(invoice.total_amount_idr)}</td>
								<td><Status state={invoice.status} /></td>
								<td>
									{#if invoice.status === 'open'}
										<Button size="sm" disabled={payingID === invoice.id} onclick={() => handlePay(invoice)}>
											Pay
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
