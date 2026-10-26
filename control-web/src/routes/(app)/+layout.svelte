<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { postJSON } from '$lib/api';
	import TopBar from '$lib/components/TopBar.svelte';
	import DemoStatusBar from '$lib/components/DemoStatusBar.svelte';
	import AppFooter from '$lib/components/AppFooter.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	let isAdmin: boolean = $state(false);
	let userEmail: string | undefined = $state(undefined);

	const pathname = $derived($page.url.pathname);
	const links = $derived([
		{ href: '/dashboard', label: 'Dashboard' },
		{ href: '/billing', label: 'Billing' },
		...(isAdmin ? [{ href: '/admin', label: 'Admin' }] : []),
		{ href: '/demo', label: 'What is this?' }
	]);

	async function handleLogout() {
		await postJSON('/api/auth/logout');
		await goto('/');
	}

	onMount(async () => {
		try {
			const res = await fetch('/api/me');
			if (res.ok) {
				const me = await res.json();
				isAdmin = me.is_admin === true;
				userEmail = me.email;
			}
		} catch {
			isAdmin = false;
			userEmail = undefined;
		}
	});
</script>

<TopBar {links} {userEmail} onLogout={handleLogout} />
{#if pathname === '/dashboard'}
	<div data-tour="dashboard-status">
		<DemoStatusBar />
	</div>
{:else}
	<DemoStatusBar />
{/if}
{@render children()}
<AppFooter />
