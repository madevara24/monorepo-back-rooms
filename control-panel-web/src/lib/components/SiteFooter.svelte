<script lang="ts">
	import { page } from '$app/stores';

	const pathname = $derived($page.url.pathname);
</script>

<footer class="border-t border-line">
	<div class="mx-auto flex max-w-5xl flex-wrap items-center gap-3 px-4 py-4 text-xs text-muted">
		{#if pathname !== '/'}
			<a href="/" class="underline">Storefront</a>
		{/if}
		{#if pathname !== '/demo'}
			<a href="/demo" class="underline">Demo</a>
		{/if}
		{#if pathname !== '/about'}
			<a href="/about" class="underline">About</a>
		{/if}
	</div>
</footer>
