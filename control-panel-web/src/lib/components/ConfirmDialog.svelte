<script lang="ts">
	import Dialog from './Dialog.svelte';
	import Button from './Button.svelte';

	type Props = {
		open: boolean;
		title: string;
		message: string;
		confirmLabel?: string;
		danger?: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	};

	let {
		open,
		title,
		message,
		confirmLabel = 'Confirm',
		danger = true,
		onconfirm,
		oncancel
	}: Props = $props();
</script>

<Dialog {open} {title} onclose={oncancel}>
	<p class="text-sm text-muted">{message}</p>
	<div class="flex justify-end gap-2">
		<Button variant="ghost" onclick={oncancel}>Cancel</Button>
		<Button variant={danger ? 'danger' : 'primary'} onclick={onconfirm}>{confirmLabel}</Button>
	</div>
</Dialog>
