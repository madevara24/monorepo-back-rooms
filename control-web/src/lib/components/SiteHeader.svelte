<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { postJSON } from '$lib/api';
	import type { Session } from '$lib/session.svelte';
	import ThemeToggle from './ThemeToggle.svelte';

	type Props = {
		session: Session;
	};

	let { session }: Props = $props();

	let mobileOpen: boolean = $state(false);

	const pathname = $derived($page.url.pathname);

	type NavLink = { href: string; label: string };

	const navLinks = $derived<NavLink[]>([
		{ href: '/demo', label: 'Demo' },
		{ href: '/about', label: 'About' },
		...(session.signedIn
			? [
					{ href: '/dashboard', label: 'Dashboard' },
					{ href: '/billing', label: 'Billing' },
					...(session.isAdmin ? [{ href: '/admin', label: 'Admin' }] : [])
				]
			: [])
	]);

	function closeMobile() {
		mobileOpen = false;
	}

	async function handleLogout() {
		await postJSON('/api/auth/logout');
		closeMobile();
		// Navigate away from any auth-guarded (app) route before clearing the
		// session: clearing first would flip session.signedIn to false while
		// still on e.g. /dashboard, racing the (app) layout's guard effect
		// (which treats signedIn -> false as an unauthenticated redirect) against
		// this goto. Clearing after the navigation settles avoids that race.
		await goto('/');
		session.clear();
	}
</script>

<header class="border-b border-line">
	<div class="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
		<a href="/" class="text-lg font-semibold text-text">MBR</a>

		<nav class="hidden items-center gap-4 sm:flex" aria-label="Main">
			{#each navLinks as link (link.href)}
				<a
					class="text-sm text-text hover:text-accent {pathname === link.href
						? 'font-semibold text-accent'
						: ''}"
					href={link.href}
					aria-current={pathname === link.href ? 'page' : undefined}
				>
					{link.label}
				</a>
			{/each}
			{#if session.signedIn}
				{#if session.user?.email}
					<span class="font-mono text-xs text-muted">{session.user.email}</span>
				{/if}
				<button type="button" class="text-sm text-text hover:text-accent" onclick={handleLogout}>
					Log out
				</button>
			{:else}
				<a
					class="text-sm text-text hover:text-accent"
					href="/signup"
					aria-current={pathname === '/signup' ? 'page' : undefined}>Create account</a
				>
				<a
					class="text-sm text-text hover:text-accent"
					href="/login"
					aria-current={pathname === '/login' ? 'page' : undefined}>Log in</a
				>
			{/if}
			<ThemeToggle />
		</nav>

		<button
			type="button"
			class="flex h-11 w-11 items-center justify-center rounded-md border border-line text-text sm:hidden"
			aria-label="Menu"
			aria-expanded={mobileOpen}
			aria-controls="site-mobile-nav"
			onclick={() => (mobileOpen = !mobileOpen)}
		>
			<span aria-hidden="true">{mobileOpen ? '×' : '☰'}</span>
		</button>
	</div>

	{#if mobileOpen}
		<nav id="site-mobile-nav" class="flex flex-col gap-1 border-t border-line px-4 py-3 sm:hidden" aria-label="Main">
			{#each navLinks as link (link.href)}
				<a
					class="flex h-11 items-center text-sm text-text hover:text-accent {pathname === link.href
						? 'font-semibold text-accent'
						: ''}"
					href={link.href}
					aria-current={pathname === link.href ? 'page' : undefined}
					onclick={closeMobile}
				>
					{link.label}
				</a>
			{/each}
			{#if session.signedIn}
				{#if session.user?.email}
					<span class="flex h-11 items-center font-mono text-xs text-muted">{session.user.email}</span>
				{/if}
				<button
					type="button"
					class="flex h-11 items-center text-left text-sm text-text hover:text-accent"
					onclick={handleLogout}
				>
					Log out
				</button>
			{:else}
				<a
					class="flex h-11 items-center text-sm text-text hover:text-accent"
					href="/signup"
					aria-current={pathname === '/signup' ? 'page' : undefined}
					onclick={closeMobile}>Create account</a
				>
				<a
					class="flex h-11 items-center text-sm text-text hover:text-accent"
					href="/login"
					aria-current={pathname === '/login' ? 'page' : undefined}
					onclick={closeMobile}>Log in</a
				>
			{/if}
			<div class="pt-1">
				<ThemeToggle />
			</div>
		</nav>
	{/if}
</header>
