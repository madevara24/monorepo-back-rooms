<script lang="ts">
	import { onMount, getContext } from 'svelte';
	import type { Plan } from '$lib/api';
	import Button from '$lib/components/Button.svelte';
	import DemoCountdown from '$lib/components/DemoCountdown.svelte';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
	import { storefrontTour } from '$lib/tour.svelte';

	const prompt = getContext<{ show: () => void; hide: () => void }>('mbr-prompt');

	let signedIn: boolean = $state(false);
	let isAdmin: boolean = $state(false);
	let plans: Plan[] = $state([]);
	let plansState: 'loading' | 'ready' | 'error' = $state('loading');

	const cardBullets = ['TLS on a public hostname', 'Connection string and JDBC URL'];

	const tourActive = $derived(storefrontTour.step !== null || storefrontTour.isFinished);

	function withTour(href: string): string {
		if (!tourActive) return href;
		return `${href}${href.includes('?') ? '&' : '?'}tour=1`;
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

	function buyHref(slug: string) {
		const path = signedIn ? '/dashboard' : '/signup';
		return withTour(`${path}?plan=${encodeURIComponent(slug)}`);
	}

	onMount(async () => {
		await loadPlans();
		try {
			const meRes = await fetch('/api/me');
			signedIn = meRes.ok;
			if (meRes.ok) {
				const me = await meRes.json();
				isAdmin = me.is_admin === true;
			}
		} catch {
			signedIn = false;
		} finally {
			if (!signedIn) prompt?.show();
		}
	});
</script>

<svelte:head>
	<title>MBR — Postgres, by the gigabyte.</title>
</svelte:head>

<div class="mx-auto flex min-h-screen max-w-5xl flex-col gap-10 px-4 py-6">
	<header class="flex items-center justify-between">
		<span class="text-lg font-semibold text-text">MBR</span>
		<ThemeToggle />
	</header>

	<section class="flex flex-col gap-3" data-tour="hero">
		<h1 class="text-3xl font-semibold text-text">Postgres, by the gigabyte.</h1>
		<p class="text-muted">
			Pick a package and get a running Postgres with TLS, a public hostname, and a connection
			string.
		</p>
		<nav class="flex flex-wrap gap-3" data-tour="account-cta">
			{#if signedIn}
				<a href={withTour('/dashboard')}><Button>Go to dashboard</Button></a>
				{#if isAdmin}
					<a href="/admin"><Button variant="secondary">Admin</Button></a>
				{/if}
			{:else}
				<a href={withTour('/signup')}><Button>Create an account</Button></a>
				<a href={withTour('/login')}><Button variant="secondary">Log in</Button></a>
			{/if}
		</nav>
	</section>

	<section class="flex flex-col gap-4" data-tour="packages">
		<h2 class="text-xl font-semibold text-text">Packages</h2>

		{#if plansState === 'loading'}
			<p class="text-muted">Loading packages...</p>
		{:else if plansState === 'error' || plans.length === 0}
			<p class="text-muted">
				Packages are unavailable right now. <a href="/" data-sveltekit-reload class="underline"
					>Reload</a
				>
			</p>
		{:else}
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
				{#each plans as plan (plan.id)}
					<div class="flex flex-col gap-3 rounded-md border border-line bg-surface p-4">
						<div class="flex items-center justify-between gap-2">
							<h3 class="text-lg font-semibold text-text">{plan.name}</h3>
							{#if plan.badge}
								<span
									class="rounded-full bg-accent px-2 py-0.5 text-xs font-medium text-on-accent"
									>{plan.badge}</span
								>
							{/if}
						</div>
						<p class="flex items-baseline gap-1">
							<span class="font-mono text-lg text-text"
								>IDR {plan.price_idr_month.toLocaleString('id-ID')}</span
							>
							<span class="text-sm text-muted">/ month</span>
						</p>
						<p class="text-sm text-muted">{plan.size_gb} GB</p>
						<p class="text-sm text-text">{plan.tagline}</p>
						<ul class="flex flex-col gap-1 text-sm text-muted">
							{#each cardBullets as bullet (bullet)}
								<li>{bullet}</li>
							{/each}
						</ul>
						{#if plan.purchasable}
							<a href={buyHref(plan.slug)} class="mt-auto">
								<Button class="w-full">Get {plan.name}</Button>
							</a>
						{:else}
							<Button class="mt-auto w-full" disabled>Sold out</Button>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

		<p class="text-xs text-muted">
			Prices are monthly. 1 GB and 5 GB are available. The 10 GB and 20 GB prices are shown for
			reference and marked sold out for this demo. The demo does not charge you.
		</p>
	</section>

	<footer class="mt-auto flex flex-wrap items-center gap-3 text-xs text-muted">
		<DemoCountdown />
		<a href="/demo" class="underline">What is this?</a>
		<a href="/about" class="underline">About</a>
	</footer>
</div>
