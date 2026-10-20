<script lang="ts">
	import { goto } from '$app/navigation';
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
			const res = await postJSON('/api/auth/signup', { email, password });
			if (!res.ok) {
				error = await errorMessage(res);
				return;
			}
			await goto('/dashboard');
		} finally {
			submitting = false;
		}
	}
</script>

<div class="flex min-h-screen flex-col items-center justify-center gap-6 px-4">
	<div class="absolute top-4 right-4">
		<ThemeToggle />
	</div>

	<h1 class="text-2xl font-semibold text-text">Sign up</h1>

	<form class="flex w-full max-w-sm flex-col gap-3" onsubmit={handleSubmit}>
		<Field label="Email" id="signup-email">
			<Input id="signup-email" type="email" bind:value={email} required autocomplete="email" />
		</Field>
		<Field label="Password" id="signup-password">
			<Input
				id="signup-password"
				type="password"
				bind:value={password}
				required
				minlength={8}
				autocomplete="new-password"
			/>
		</Field>

		{#if error}
			<p class="text-sm text-error">{error}</p>
		{/if}

		<Button type="submit" disabled={submitting}>Create account</Button>
	</form>

	<p class="text-sm text-muted">Already have an account? <a class="text-accent underline" href="/login">Log in</a></p>
</div>
