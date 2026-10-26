<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { errorMessage, postJSON } from '$lib/api';
	import Field from '$lib/components/Field.svelte';
	import Input from '$lib/components/Input.svelte';
	import Button from '$lib/components/Button.svelte';

	let email: string = $state('');
	let password: string = $state('');
	let error: string = $state('');
	let submitting: boolean = $state(false);
	let loginHref = $state('/login');
	let resetNotice = $state(false);

	onMount(() => {
		const params = new URLSearchParams(location.search);
		resetNotice = params.get('reset') === '1';
		loginHref = params.get('tour') === '1' ? '/login?tour=1' : '/login';
	});

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
			const params = new URLSearchParams();
			const plan = $page.url.searchParams.get('plan');
			const tour = $page.url.searchParams.get('tour');
			if (plan) params.set('plan', plan);
			if (tour) params.set('tour', tour);
			const query = params.toString();
			await goto(query ? `/dashboard?${query}` : '/dashboard');
		} finally {
			submitting = false;
		}
	}
</script>

<h1 class="text-2xl font-semibold text-text">Sign up</h1>

{#if resetNotice}
	<p class="max-w-sm text-center text-sm text-muted">
		That account was deleted by the 30-minute reset. Create a new one.
	</p>
{/if}

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

<p class="text-sm text-muted">Already have an account? <a class="text-accent underline" href={loginHref}>Log in</a></p>
