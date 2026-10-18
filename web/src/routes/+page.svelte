<script lang="ts">
	let status: string = $state('checking...');

	async function checkHealth() {
		try {
			const res = await fetch('/healthz');
			const body = await res.json();
			status = body.status ?? 'unknown';
		} catch {
			status = 'unreachable';
		}
	}

	checkHealth();
</script>

<main class="flex min-h-screen flex-col items-center justify-center gap-4">
	<h1 class="text-2xl font-bold">MBR</h1>
	<p>API status: <span class="font-mono">{status}</span></p>
</main>
