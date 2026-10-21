<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { errorMessage, postJSON } from '$lib/api';
	import Field from '$lib/components/Field.svelte';
	import Input from '$lib/components/Input.svelte';
	import Button from '$lib/components/Button.svelte';
	import ThemeToggle from '$lib/components/ThemeToggle.svelte';

	let email: string = $state('');
	let password: string = $state('');
	let error: string = $state('');
	let submitting: boolean = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		submitting = true;
		try {
			const res = await postJSON('/api/auth/login', { email, password });
			if (!res.ok) {
				error = await errorMessage(res);
				return;
			}
			const plan = $page.url.searchParams.get('plan');
			await goto(plan ? `/dashboard?plan=${encodeURIComponent(plan)}` : '/dashboard');
		} finally {
			submitting = false;
		}
	}
</script>

<div class="flex min-h-screen flex-col items-center justify-center gap-6 px-4">
	<div class="absolute top-4 right-4">
		<ThemeToggle />
	</div>

	<h1 class="text-2xl font-semibold text-text">Log in</h1>

	<form class="flex w-full max-w-sm flex-col gap-3" onsubmit={handleSubmit}>
		<Field label="Email" id="login-email">
			<Input id="login-email" type="email" bind:value={email} required autocomplete="email" />
		</Field>
		<Field label="Password" id="login-password">
			<Input
				id="login-password"
				type="password"
				bind:value={password}
				required
				autocomplete="current-password"
			/>
		</Field>

		{#if error}
			<p class="text-sm text-error">{error}</p>
		{/if}

		<Button type="submit" disabled={submitting}>Log in</Button>
	</form>

	<p class="text-sm text-muted">Need an account? <a class="text-accent underline" href="/signup">Sign up</a></p>
</div>
