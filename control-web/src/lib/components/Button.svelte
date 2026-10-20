<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
		size?: 'sm' | 'md';
		type?: 'button' | 'submit';
		disabled?: boolean;
		onclick?: (event: MouseEvent) => void;
		children: Snippet;
	};

	let {
		variant = 'primary',
		size = 'md',
		type = 'button',
		disabled = false,
		onclick,
		children
	}: Props = $props();

	const variantClass: Record<string, string> = {
		primary: 'bg-accent text-on-accent border border-accent hover:opacity-90',
		secondary: 'bg-transparent text-text border border-line hover:border-accent',
		danger: 'bg-transparent text-error border border-error hover:bg-error/10',
		ghost: 'bg-transparent text-text border border-transparent hover:border-line'
	};

	const sizeClass: Record<string, string> = {
		sm: 'h-9 px-3 text-sm',
		md: 'h-11 px-4 text-sm'
	};
</script>

<button
	{type}
	{disabled}
	{onclick}
	class="inline-flex items-center justify-center gap-2 rounded-md font-medium whitespace-nowrap disabled:cursor-not-allowed disabled:opacity-50 {variantClass[
		variant
	]} {sizeClass[size]}"
>
	{@render children()}
</button>
