<script lang="ts">
	import ThemeToggle from './ThemeToggle.svelte';

	type NavLink = { href: string; label: string };

	type Props = {
		links?: NavLink[];
		userEmail?: string;
		onLogout?: () => void;
	};

	let { links = [], userEmail, onLogout }: Props = $props();
	let mobileOpen: boolean = $state(false);

	function closeMobile() {
		mobileOpen = false;
	}
</script>

<header class="border-b border-line">
	<div class="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
		<a href="/" class="text-lg font-semibold text-text">MBR</a>

		<nav class="hidden items-center gap-4 sm:flex">
			{#each links as link (link.href)}
				<a class="text-sm text-text hover:text-accent" href={link.href}>{link.label}</a>
			{/each}
			{#if userEmail}
				<span class="font-mono text-xs text-muted">{userEmail}</span>
			{/if}
			{#if onLogout}
				<button type="button" class="text-sm text-text hover:text-accent" onclick={onLogout}>
					Log out
				</button>
			{/if}
			<ThemeToggle />
		</nav>

		<button
			type="button"
			class="flex h-11 w-11 items-center justify-center rounded-md border border-line text-text sm:hidden"
			aria-label="Menu"
			aria-expanded={mobileOpen}
			onclick={() => (mobileOpen = !mobileOpen)}
		>
			<span aria-hidden="true">{mobileOpen ? '×' : '☰'}</span>
		</button>
	</div>

	{#if mobileOpen}
		<nav class="flex flex-col gap-1 border-t border-line px-4 py-3 sm:hidden">
			{#each links as link (link.href)}
				<a
					class="flex h-11 items-center text-sm text-text hover:text-accent"
					href={link.href}
					onclick={closeMobile}
				>
					{link.label}
				</a>
			{/each}
			{#if userEmail}
				<span class="flex h-11 items-center font-mono text-xs text-muted">{userEmail}</span>
			{/if}
			{#if onLogout}
				<button
					type="button"
					class="flex h-11 items-center text-left text-sm text-text hover:text-accent"
					onclick={() => {
						closeMobile();
						onLogout?.();
					}}
				>
					Log out
				</button>
			{/if}
			<div class="pt-1">
				<ThemeToggle />
			</div>
		</nav>
	{/if}
</header>
