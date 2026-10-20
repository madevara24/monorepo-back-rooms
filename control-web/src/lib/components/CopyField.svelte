<script lang="ts">
	type Props = {
		label: string;
		value: string;
	};

	let { label, value }: Props = $props();
	let copied: boolean = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copy() {
		if (!value) return;
		await navigator.clipboard.writeText(value);
		copied = true;
		clearTimeout(timer);
		timer = setTimeout(() => (copied = false), 2000);
	}
</script>

<div class="flex items-center gap-2">
	<span class="w-32 shrink-0 text-xs font-medium text-muted">{label}</span>
	<input
		class="h-9 min-w-0 flex-1 rounded-md border border-line bg-bg px-2 font-mono text-xs text-text"
		readonly
		{value}
		onclick={(e) => e.currentTarget.select()}
	/>
	<button
		class="h-9 shrink-0 rounded-md border border-line px-3 text-xs font-medium text-text hover:border-accent"
		onclick={copy}
		type="button"
	>
		{copied ? 'Copied' : 'Copy'}
	</button>
</div>
