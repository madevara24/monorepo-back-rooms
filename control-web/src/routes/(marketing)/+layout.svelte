<script lang="ts">
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import DemoStatusBar from '$lib/components/DemoStatusBar.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<SiteHeader />
<DemoStatusBar />
{@render children()}
<SiteFooter />
