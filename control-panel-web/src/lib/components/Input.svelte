<script lang="ts">
	type Props = {
		type?: string;
		value?: string | number;
		id?: string;
		placeholder?: string;
		required?: boolean;
		readonly?: boolean;
		disabled?: boolean;
		min?: number;
		max?: number;
		minlength?: number;
		mono?: boolean;
		autocomplete?: HTMLInputElement['autocomplete'];
		onclick?: (event: MouseEvent) => void;
	};

	let {
		type = 'text',
		value = $bindable(''),
		id,
		placeholder,
		required = false,
		readonly = false,
		disabled = false,
		min,
		max,
		minlength,
		mono = false,
		autocomplete,
		onclick
	}: Props = $props();
</script>

<input
	{id}
	{type}
	{placeholder}
	{required}
	{readonly}
	{disabled}
	{min}
	{max}
	{minlength}
	{autocomplete}
	{onclick}
	bind:value
	class="h-11 w-full min-w-0 rounded-md border border-line bg-surface px-3 text-text placeholder:text-muted focus-visible:border-accent disabled:opacity-50 {mono
		? 'font-mono text-sm'
		: 'text-sm'}"
/>
