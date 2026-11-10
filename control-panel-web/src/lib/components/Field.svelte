<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		label: string;
		id: string;
		hint?: string;
		children: Snippet;
	};

	let { label, id, hint, children }: Props = $props();
</script>

<div class="flex flex-col gap-1">
	<label for={id} class="text-sm font-medium text-text">{label}</label>
	{@render children()}
	{#if hint}
		<p class="text-xs text-muted">{hint}</p>
	{/if}
</div>
