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
			<a href="/signup"><Button>Create an account</Button></a>
			<a href="/login"><Button variant="secondary">Log in</Button></a>
		</nav>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">What the reset does</h2>
		<p class="text-muted">
			Every 30 minutes the demo deletes every visitor account, session, database, invoice, and
			tenant container. The plans and the node registration stay. You can create two databases
			per account, and they are deleted at the boundary.
		</p>
		<p class="text-muted">
			The account is deleted too, so you will sign in again or create a new one after a reset.
			Use a made-up email. No email is sent and no payment is taken.
		</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">The packages</h2>
		<p class="text-muted">
			The 1 GB and 5 GB packages are available. The 10 GB and 20 GB packages are visible but sold
			out. A database is capped at two per account so one visitor cannot fill the data node.
		</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">Connecting</h2>
		<p class="text-muted">
			After you create a database, the dashboard shows the connection string and a JDBC URL. The
			password is shown once. The string works until the next reset, then the database is gone.
		</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">Logs and the admin</h2>
		<p class="text-muted">
			The app keeps seven days of local access and process logs, capped by size. The data is
			deleted at the reset; the logs are not a second copy of it. The seller console is private.
			The code is public in the repository if you want to see how the rest works.
		</p>
	</section>

	<footer class="mt-auto flex flex-wrap items-center gap-3 text-xs text-muted">
		<a href="/" class="underline">Storefront</a>
		<a href="/about" class="underline">About</a>
		<a href="/login" class="underline">Log in</a>
	</footer>
</div>
