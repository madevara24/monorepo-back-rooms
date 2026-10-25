<script lang="ts">
	import Button from '$lib/components/Button.svelte';
	import DemoCountdown from '$lib/components/DemoCountdown.svelte';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';
</script>

<svelte:head>
	<title>MBR demo</title>
</svelte:head>

<div class="mx-auto flex min-h-screen max-w-3xl flex-col gap-8 px-4 py-6">
	<header class="flex items-center justify-between">
		<a href="/" class="text-lg font-semibold text-text">MBR</a>
		<ThemeToggle />
	</header>

	<section class="flex flex-col gap-3">
		<h1 class="text-3xl font-semibold text-text">A demo database marketplace</h1>
		<p class="text-muted">
			The control API, the Postgres containers, the TLS connection, and the dashboard are real.
			The payment is mocked, and everything resets every 30 minutes.
		</p>
		<DemoCountdown />
		<nav class="flex flex-wrap gap-3">
			<a href="/signup?tour=1"><Button>Create an account and take the dashboard tour</Button></a>
			<a href="/login?tour=1"
				><Button variant="secondary">Log in (only if you signed up in this cycle)</Button></a
			>
		</nav>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">What you can do here</h2>
		<ul class="flex list-disc flex-col gap-1 pl-5 text-muted">
			<li>Create up to two databases.</li>
			<li>Copy the connection string or JDBC URL from the create panel.</li>
			<li>
				Connect with TLS. The connection string verifies the server. The JDBC URL uses
				sslmode=require; add the ISRG root for verify-full.
			</li>
			<li>Watch the next successful reset remove the account and its databases.</li>
		</ul>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">The packages</h2>
		<p class="text-muted">
			The 1 GB and 5 GB packages are available. The 10 GB and 20 GB packages are visible but sold
			out. The demo allows two databases per account.
		</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">Connecting</h2>
		<p class="text-muted">
			After you create a database, the dashboard shows the connection string and a JDBC URL. The
			password is shown once. The first connection may be refused for a few seconds; retry. The
			string works until the next reset.
		</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">The reset</h2>
		<p class="text-muted">
			Every 30 minutes the demo deletes visitor accounts, sessions, databases, and tenant
			containers, then restores the plans. If a reset fails, the demo stays in maintenance and
			retries.
		</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">Logs and the admin</h2>
		<p class="text-muted">
			Logs are local and size-capped at 10 MB by 3. There is no central log store and no backup.
			The seller console is private. The code is public at
			<a
				class="underline"
				href="https://github.com/madevara24/monorepo-back-rooms"
				target="_blank"
				rel="noopener noreferrer">https://github.com/madevara24/monorepo-back-rooms</a
			>.
		</p>
	</section>

	<footer class="mt-auto flex flex-wrap items-center gap-3 text-xs text-muted">
		<a href="/" class="underline">Storefront</a>
		<a href="/about" class="underline">About</a>
		<a href="/login" class="underline">Log in</a>
	</footer>
</div>
