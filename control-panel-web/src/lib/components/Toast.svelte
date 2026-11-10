<script lang="ts">
	import { getToasts, dismissToast } from '$lib/toast.svelte';

	const toasts = $derived(getToasts());
</script>

{#if toasts.length > 0}
	<div class="fixed bottom-4 left-1/2 z-50 flex w-full max-w-sm -translate-x-1/2 flex-col gap-2 px-4">
		{#each toasts as toast (toast.id)}
			<div
				role="status"
				class="flex items-center justify-between gap-3 rounded-md border px-4 py-3 text-sm shadow-lg {toast.kind ===
				'error'
					? 'border-error bg-surface text-error'
					: 'border-accent bg-surface text-accent'}"
			>
				<span>{toast.message}</span>
				<button
					type="button"
					class="text-muted hover:text-text"
					onclick={() => dismissToast(toast.id)}
					aria-label="Dismiss"
				>
					×
				</button>
			</div>
		{/each}
	</div>
{/if}
