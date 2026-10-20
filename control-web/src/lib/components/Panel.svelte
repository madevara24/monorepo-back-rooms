<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		title?: string;
		children: Snippet;
	};

	let { title, children }: Props = $props();
</script>

<section class="flex flex-col gap-3 rounded-md border border-line bg-surface p-4">
	{#if title}
		<h2 class="text-lg font-semibold text-text">{title}</h2>
	{/if}
	{@render children()}
</section>
