<script lang="ts">
	import './layout.css';
	import { setContext } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import favicon from '$lib/assets/favicon.svg';
	import Toast from '$lib/components/Toast.svelte';
	import Tour from '$lib/components/Tour.svelte';
	import Prompt from '$lib/components/Prompt.svelte';
	import { storefrontTour, dashboardTour } from '$lib/tour.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	let showPrompt = $state(false);

	setContext('mbr-prompt', {
		show() {
			showPrompt = true;
		},
		hide() {
			showPrompt = false;
		}
	});

	const pathname = $derived($page.url.pathname);
	const activeTour = $derived(
		pathname === '/' ? storefrontTour : pathname === '/dashboard' ? dashboardTour : null
	);
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
<Toast />

{#if activeTour}
	<Tour tour={activeTour} currentPath={pathname} navigate={(path: string) => goto(path)} />
{/if}

{#if pathname === '/'}
	<Prompt open={showPrompt} onStart={() => storefrontTour.start()} />
{/if}
