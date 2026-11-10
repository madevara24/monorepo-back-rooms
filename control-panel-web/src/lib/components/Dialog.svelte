<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		open: boolean;
		title: string;
		children: Snippet;
		onclose?: () => void;
	};

	let { open, title, children, onclose }: Props = $props();
	let dialogEl: HTMLDialogElement | undefined = $state();

	$effect(() => {
		if (!dialogEl) return;
		if (open && !dialogEl.open) {
			dialogEl.showModal();
		} else if (!open && dialogEl.open) {
			dialogEl.close();
		}
	});

	function handleClose() {
		onclose?.();
	}

	function handleBackdropClick(event: MouseEvent) {
		if (event.target === dialogEl) {
			dialogEl?.close();
		}
	}
</script>

<dialog
	bind:this={dialogEl}
	onclose={handleClose}
	onclick={handleBackdropClick}
	class="w-full max-w-md rounded-md border border-line bg-surface p-0 text-text backdrop:bg-black/50"
>
	<div class="flex flex-col gap-4 p-5">
		<h2 class="text-lg font-semibold">{title}</h2>
		{@render children()}
	</div>
</dialog>
