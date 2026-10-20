<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		children: Snippet;
	};

	let { children }: Props = $props();
</script>

<div class="overflow-x-auto rounded-md border border-line">
	<table class="w-full min-w-max border-collapse text-left text-sm">
		{@render children()}
	</table>
</div>

<style>
	:global(table thead tr) {
		border-bottom: 1px solid var(--color-line);
	}
	:global(table th) {
		padding: 0.5rem 0.75rem;
		font-weight: 500;
		color: var(--color-muted);
		white-space: nowrap;
	}
	:global(table td) {
		padding: 0.5rem 0.75rem;
		vertical-align: middle;
	}
	:global(table tbody tr) {
		border-bottom: 1px solid var(--color-line);
	}
	:global(table tbody tr:last-child) {
		border-bottom: none;
	}
</style>
