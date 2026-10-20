<script lang="ts">
	type Props = {
		message: string;
	};

	let { message }: Props = $props();
</script>

<p class="rounded-md border border-dashed border-line px-4 py-8 text-center text-sm text-muted">
	{message}
</p>
