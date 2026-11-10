<script lang="ts">
	import Status from './Status.svelte';

	type Props = {
		apiState: 'reachable' | 'unreachable' | 'checking';
		endpoint: string;
		databaseCount?: number | null;
		provisionedGB?: number | null;
		dense?: boolean;
	};

	let { apiState, endpoint, databaseCount = null, provisionedGB = null, dense = false }: Props =
		$props();
</script>

<div
	class="flex flex-wrap items-center gap-x-6 gap-y-2 border-line {dense
		? 'border-b px-4 py-2 text-xs'
		: 'rounded-md border bg-surface px-4 py-3 text-sm'}"
>
	<Status state={apiState} label={apiState === 'checking' ? 'checking…' : `API ${apiState}`} />
	{#if databaseCount !== null}
		<span class="text-muted">{databaseCount} database{databaseCount === 1 ? '' : 's'}</span>
	{/if}
	{#if provisionedGB !== null}
		<span class="text-muted">{provisionedGB} GB provisioned</span>
	{/if}
	<span class="font-mono text-muted break-all">{endpoint}</span>
</div>
