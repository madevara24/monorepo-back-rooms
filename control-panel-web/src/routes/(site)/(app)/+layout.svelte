<script lang="ts">
	import { getContext, setContext } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { dashboardTour } from '$lib/tour.svelte';
	import { SESSION_CONTEXT_KEY, type Session } from '$lib/session.svelte';
	import { AUTH_GUARD_CONTEXT_KEY, type AuthGuard } from '$lib/authGuard';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	const session = getContext<Session>(SESSION_CONTEXT_KEY);
	const pathname = $derived($page.url.pathname);

	function handleUnauthenticated() {
		dashboardTour.clear();
		if (pathname === '/admin') {
			void goto('/login');
		} else {
			void goto('/signup?reset=1&tour=1');
		}
	}

	const authGuard: AuthGuard = { handleUnauthenticated };
	setContext(AUTH_GUARD_CONTEXT_KEY, authGuard);

	const authorized = $derived(session.signedIn && (pathname !== '/admin' || session.isAdmin));
	const ready = $derived(!session.loading && authorized);

	$effect(() => {
		if (session.loading) return;
		if (!session.signedIn) {
			handleUnauthenticated();
			return;
		}
		if (pathname === '/admin' && !session.isAdmin) {
			void goto('/');
		}
	});
</script>

{#if ready}
	{@render children()}
{:else if session.loading}
	<p class="mx-auto max-w-3xl px-4 py-6 text-muted">Loading...</p>
{/if}
