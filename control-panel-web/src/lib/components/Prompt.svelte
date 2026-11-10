<script lang="ts">
	import { onMount } from 'svelte';

	type Props = {
		open: boolean;
		onStart: () => void;
	};

	let { open, onStart }: Props = $props();

	const SEEN_KEY = 'mbr-popup-v1:seen';

	let seen = $state(false);
	let dismissed = $state(false);
	let cardEl: HTMLDivElement | undefined = $state();
	let triggerEl: HTMLElement | null = null;

	onMount(() => {
		seen = sessionStorage.getItem(SEEN_KEY) === '1';
	});

	const visible = $derived(open && !seen && !dismissed);

	$effect(() => {
		if (visible) {
			triggerEl = document.activeElement as HTMLElement | null;
			cardEl?.focus();
		}
	});

	function markSeen() {
		seen = true;
		if (typeof sessionStorage !== 'undefined') sessionStorage.setItem(SEEN_KEY, '1');
	}

	function dismiss() {
		dismissed = true;
		markSeen();
		triggerEl?.focus?.();
	}

	function start() {
		dismissed = true;
		markSeen();
		onStart();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Escape') return;
		event.preventDefault();
		dismiss();
	}
</script>

<svelte:window onkeydown={visible ? handleKeydown : undefined} />

{#if visible}
	<div
		bind:this={cardEl}
		role="dialog"
		aria-labelledby="mbr-popup-title"
		tabindex="-1"
		class="fixed right-4 bottom-4 z-50 flex w-[calc(100%-2rem)] max-w-xs flex-col gap-3 rounded-md border border-line bg-surface p-4 text-text shadow-lg outline-none sm:right-6 sm:bottom-6"
	>
		<h2 id="mbr-popup-title" class="text-sm font-semibold">A demo Postgres marketplace</h2>
		<p class="text-sm text-muted">
			Choose a size and connect over TLS. The 30-minute reset deletes the database, and the demo
			does not charge you.
		</p>
		<div class="flex justify-end gap-2">
			<button type="button" class="text-sm text-muted underline" onclick={dismiss}>
				See packages
			</button>
			<button
				type="button"
				class="rounded-md bg-accent px-3 py-2 text-sm font-medium text-on-accent hover:opacity-90"
				onclick={start}
			>
				Show me how
			</button>
		</div>
	</div>
{/if}
