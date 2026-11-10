<script lang="ts">
	import { page } from '$app/stores';
	import DemoCountdown from './DemoCountdown.svelte';

	const pathname = $derived($page.url.pathname);
</script>

<div class="border-b border-line">
	<div class="mx-auto flex max-w-5xl flex-wrap items-center gap-3 px-4 py-2 text-xs">
		<DemoCountdown />
		{#if pathname !== '/demo'}
			<a href="/demo" class="underline">Demo</a>
		{/if}
	</div>
</div>
