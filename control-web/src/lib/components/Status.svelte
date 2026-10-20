<script lang="ts">
	type Tone = 'good' | 'pending' | 'bad' | 'neutral';

	type Props = {
		state: string;
		label?: string;
	};

	let { state, label }: Props = $props();

	const toneByState: Record<string, Tone> = {
		running: 'good',
		reachable: 'good',
		fresh: 'good',
		paid: 'good',
		open: 'pending',
		provisioning: 'pending',
		queued: 'pending',
		pending: 'pending',
		deleting: 'pending',
		stale: 'bad',
		failed: 'bad',
		unreachable: 'bad',
		error: 'bad'
	};

	const tone = $derived(toneByState[state] ?? 'neutral');

	const toneClass: Record<Tone, string> = {
		good: 'text-accent',
		pending: 'text-warning',
		bad: 'text-error',
		neutral: 'text-muted'
	};

	const marker: Record<Tone, string> = {
		good: '●',
		pending: '◐',
		bad: '○',
		neutral: '○'
	};
</script>

<span class="inline-flex items-center gap-1.5 text-sm {toneClass[tone]}">
	<span aria-hidden="true">{marker[tone]}</span>
	<span>{label ?? state}</span>
</span>
