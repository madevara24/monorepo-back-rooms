<script lang="ts">
	import Button from '$lib/components/Button.svelte';
</script>

<svelte:head>
	<title>MBR demo</title>
</svelte:head>

<div class="mx-auto flex min-h-screen max-w-3xl flex-col gap-8 px-4 py-6">
	<section class="flex flex-col gap-3">
		<h1 class="text-3xl font-semibold text-text">A Postgres demo you can connect to.</h1>
		<p class="text-muted">
			Create up to two Postgres databases with TLS and a connection string. The 30-minute reset
			deletes them, and the demo does not charge you.
		</p>
		<nav class="flex flex-wrap gap-3">
			<a href="/signup?tour=1"><Button>Create an account</Button></a>
			<a href="/login?tour=1"><Button variant="secondary">Log in</Button></a>
		</nav>
		<p class="text-sm text-muted">The reset deletes accounts, so create a new one if yours is gone.</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">What you can do</h2>
		<ul class="flex list-disc flex-col gap-1 pl-5 text-muted">
			<li>Create up to two databases.</li>
			<li>Copy the connection string after you create a database.</li>
			<li>Connect with TLS. The connection string verifies the server.</li>
		</ul>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">Packages</h2>
		<p class="text-muted">
			1 GB and 5 GB are available. The 10 GB and 20 GB prices are shown for reference and marked
			sold out for this demo. You can hold two databases at once.
		</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">Connecting</h2>
		<p class="text-muted">
			After you create a database, the dashboard shows its connection string. Copy the password
			then; the dashboard will not show it again. If the first connection is refused, wait a few
			seconds and retry. The next reset deletes the database.
		</p>
	</section>

	<section class="flex flex-col gap-3">
		<h2 class="text-xl font-semibold text-text">The reset</h2>
		<p class="text-muted">
			This is a demo, so the reset clears every visitor account and its databases every 30
			minutes. The countdown shows the next reset.
		</p>
	</section>
</div>
