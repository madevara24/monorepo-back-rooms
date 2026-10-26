<script lang="ts">
	import { setContext, onMount } from 'svelte';
	import { page } from '$app/stores';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import DemoStatusBar from '$lib/components/DemoStatusBar.svelte';
	import SkipLink from '$lib/components/SkipLink.svelte';
	import { createSession, SESSION_CONTEXT_KEY } from '$lib/session.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	const session = createSession();
	setContext(SESSION_CONTEXT_KEY, session);

	const pathname = $derived($page.url.pathname);

	onMount(() => {
		session.refresh();
	});
</script>

<SkipLink />
<SiteHeader {session} />
{#if pathname === '/dashboard'}
	<div data-tour="dashboard-status">
		<DemoStatusBar />
	</div>
{:else}
	<DemoStatusBar />
{/if}
<main id="main">
	{@render children()}
</main>
<SiteFooter />
