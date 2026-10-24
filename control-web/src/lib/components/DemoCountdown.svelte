<script lang="ts">
	import { onMount } from 'svelte';
	import type { DemoStatus } from '$lib/api';

	let status = $state<DemoStatus | null>(null);
	let now: number = $state(Date.now());
	let hidden: boolean = $state(false);

	async function load() {
		try {
			const res = await fetch('/api/demo/status');
			if (!res.ok) {
				hidden = true;
				return;
			}
			status = await res.json();
			hidden = false;
		} catch {
			hidden = true;
		}
	}

	const secondsLeft = $derived(
		status ? Math.max(0, Math.floor((new Date(status.next_reset_at).getTime() - now) / 1000)) : null
	);
	const countdown = $derived(
		secondsLeft === null
			? ''
			: `${String(Math.floor(secondsLeft / 60)).padStart(2, '0')}:${String(secondsLeft % 60).padStart(2, '0')}`
	);
	const nextLabel = $derived(
		status
			? new Date(status.next_reset_at).toLocaleTimeString('en-GB', {
					hour: '2-digit',
					minute: '2-digit',
					timeZone: 'Asia/Jakarta'
				})
			: ''
	);

	onMount(() => {
		load();
		const tick = setInterval(() => (now = Date.now()), 1000);
		const refresh = setInterval(load, 15000);
		return () => {
			clearInterval(tick);
			clearInterval(refresh);
		};
	});
</script>

{#if !hidden && status}
	<span class="text-xs text-muted">
		{#if status.maintenance}
			Demo · resetting now
		{:else}
			Demo · resets in {countdown} · next {nextLabel} GMT+7
		{/if}
	</span>
{/if}
