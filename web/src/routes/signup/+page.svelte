<script lang="ts">
	import { goto } from '$app/navigation';
	import { errorMessage, postJSON } from '$lib/api';

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

<main class="flex min-h-screen flex-col items-center justify-center gap-4">
	<h1 class="text-2xl font-bold">Sign up</h1>

	<form class="flex w-full max-w-sm flex-col gap-3" onsubmit={handleSubmit}>
		<label class="flex flex-col gap-1">
			Email
			<input
				class="rounded border px-3 py-2"
				type="email"
				bind:value={email}
				required
				autocomplete="email"
			/>
		</label>
		<label class="flex flex-col gap-1">
			Password
			<input
				class="rounded border px-3 py-2"
				type="password"
				bind:value={password}
				required
				minlength="8"
				autocomplete="new-password"
			/>
		</label>

		{#if error}
			<p class="text-sm text-red-600">{error}</p>
		{/if}

		<button class="rounded bg-black px-4 py-2 text-white disabled:opacity-50" disabled={submitting}>
			Create account
		</button>
	</form>

	<p>Already have an account? <a class="underline" href="/login">Log in</a></p>
</main>
