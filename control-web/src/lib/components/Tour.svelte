<script lang="ts">
	import { onMount } from 'svelte';
	import type { Step } from '$lib/tour.svelte';

	type TourController = {
		readonly step: Step | null;
		readonly isLast: boolean;
		next: () => void;
		finish: () => void;
		skip: () => void;
		hydrate: () => void;
	};

	type Props = {
		tour: TourController;
		currentPath: string;
		navigate: (path: string) => void;
	};

	let { tour, currentPath, navigate }: Props = $props();

	let rect: { left: number; top: number; width: number; height: number } | null = $state(null);
	let cardEl: HTMLDivElement | undefined = $state();

	function targetElement(): Element | null {
		const step = tour.step;
		if (!step || step.route !== currentPath) return null;
		return document.querySelector(step.selector);
	}

	function measure() {
		const el = targetElement();
		if (!el) {
			rect = null;
			return;
		}
		const box = el.getBoundingClientRect();
		rect = { left: box.left, top: box.top, width: box.width, height: box.height };
	}

	$effect(() => {
		if (!tour.step) {
			rect = null;
			return;
		}
		const el = targetElement();
		if (el) {
			el.scrollIntoView({ block: 'center' });
			requestAnimationFrame(measure);
		} else {
			rect = null;
		}
		cardEl?.focus();
	});

	onMount(() => {
		tour.hydrate();
		const onReflow = () => measure();
		window.addEventListener('scroll', onReflow, true);
		window.addEventListener('resize', onReflow);
		return () => {
			window.removeEventListener('scroll', onReflow, true);
			window.removeEventListener('resize', onReflow);
		};
	});

	function handleKeydown(event: KeyboardEvent) {
		if (event.key !== 'Escape') return;
		event.preventDefault();
		tour.skip();
	}
</script>

<svelte:window onkeydown={tour.step ? handleKeydown : undefined} />

{#if tour.step}
	{#if rect}
		<div
			class="fixed"
			style="left:{rect.left}px; top:{rect.top}px; width:{rect.width}px; height:{rect.height}px; outline: 2px solid var(--mbr-accent); outline-offset: 8px; background: transparent; box-shadow: none; pointer-events: none; z-index: 2147483000;"
			aria-hidden="true"
		></div>
	{/if}

	<div
		bind:this={cardEl}
		role="dialog"
		aria-labelledby="mbr-tour-title"
		aria-describedby="mbr-tour-body"
		tabindex="-1"
		class="fixed inset-x-4 bottom-4 mx-auto flex max-w-sm flex-col gap-3 rounded-md border border-line bg-surface p-4 text-text outline-none sm:inset-x-auto sm:right-6"
		style="z-index: 2147483001;"
	>
		<h2 id="mbr-tour-title" class="text-base font-semibold">{tour.step.title}</h2>
		<p id="mbr-tour-body" class="text-sm text-muted">{tour.step.body}</p>
		<div class="flex justify-end gap-2">
			<button type="button" class="text-sm text-muted underline" onclick={() => tour.skip()}>
				Skip tour
			</button>
			<button
				type="button"
				class="rounded-md bg-accent px-3 py-2 text-sm font-medium text-on-accent hover:opacity-90"
				onclick={() => (tour.isLast ? tour.finish() : tour.next())}
			>
				{tour.isLast ? 'Done' : 'Next'}
			</button>
		</div>
	</div>
{/if}
