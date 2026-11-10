export type Step = {
	id: string;
	route: string;
	selector: string;
	title: string;
	body: string;
};

export type Tour = {
	readonly steps: Step[];
	readonly step: Step | null;
	readonly isLast: boolean;
	readonly isDone: boolean;
	readonly isFinished: boolean;
	start: () => void;
	next: () => void;
	finish: () => void;
	skip: () => void;
	pause: () => void;
	resume: () => void;
	hydrate: () => void;
	clear: () => void;
};

const FINISHED = 'finished';
const SKIPPED = 'skipped';

function createTour(track: string, steps: Step[]): Tour {
	const storageKey = `mbr-tour-v1:${track}:step`;

	let stepId: string | null = $state(null);
	let activeFlag = $state(false);
	let hydrated = false;

	const index = $derived(stepId ? steps.findIndex((s) => s.id === stepId) : -1);
	const step = $derived(activeFlag && index >= 0 ? steps[index] : null);
	const isLast = $derived(index === steps.length - 1);
	const isDone = $derived(stepId === FINISHED || stepId === SKIPPED);
	const isFinished = $derived(stepId === FINISHED);

	function persist() {
		if (typeof sessionStorage === 'undefined') return;
		if (stepId) sessionStorage.setItem(storageKey, stepId);
		else sessionStorage.removeItem(storageKey);
	}

	function start() {
		stepId = steps[0]?.id ?? null;
		activeFlag = true;
		persist();
	}

	function next() {
		if (index < 0) return;
		if (index >= steps.length - 1) {
			finish();
			return;
		}
		stepId = steps[index + 1].id;
		persist();
	}

	function finish() {
		stepId = FINISHED;
		activeFlag = false;
		persist();
	}

	function skip() {
		stepId = SKIPPED;
		activeFlag = false;
		persist();
	}

	function pause() {
		activeFlag = false;
	}

	function resume() {
		if (index >= 0 && stepId !== FINISHED && stepId !== SKIPPED) {
			activeFlag = true;
		}
	}

	function hydrate() {
		if (hydrated) return;
		hydrated = true;
		if (typeof sessionStorage === 'undefined') return;
		const saved = sessionStorage.getItem(storageKey);
		if (!saved) return;
		if (saved === FINISHED || saved === SKIPPED) {
			stepId = saved;
			return;
		}
		if (steps.some((s) => s.id === saved)) {
			stepId = saved;
			activeFlag = true;
		}
	}

	function clear() {
		stepId = null;
		activeFlag = false;
		hydrated = true;
		if (typeof sessionStorage !== 'undefined') sessionStorage.removeItem(storageKey);
	}

	return {
		get steps() {
			return steps;
		},
		get step() {
			return step;
		},
		get isLast() {
			return isLast;
		},
		get isDone() {
			return isDone;
		},
		get isFinished() {
			return isFinished;
		},
		start,
		next,
		finish,
		skip,
		pause,
		resume,
		hydrate,
		clear
	};
}

export const storefrontTour = createTour('storefront', [
	{
		id: 'hero',
		route: '/',
		selector: '[data-tour="hero"]',
		title: 'Postgres, by the gigabyte.',
		body: 'Choose a size and get a running Postgres with a public hostname, TLS, and a connection string. The demo does not charge you.'
	},
	{
		id: 'packages',
		route: '/',
		selector: '[data-tour="packages"]',
		title: 'Two sizes are available.',
		body: '1 GB and 5 GB are available. 10 GB and 20 GB are sold out.'
	},
	{
		id: 'account-cta',
		route: '/',
		selector: '[data-tour="account-cta"]',
		title: 'Create an account to continue.',
		body: 'Use any email address; the demo does not send mail. The next reset deletes the account and its databases.'
	}
]);

export const dashboardTour = createTour('dashboard', [
	{
		id: 'dashboard-status',
		route: '/dashboard',
		selector: '[data-tour="dashboard-status"]',
		title: 'Your database lasts until the reset.',
		body: 'The countdown shows the next reset. When it runs, the demo deletes this account and every database in it.'
	},
	{
		id: 'create-form',
		route: '/dashboard',
		selector: '[data-tour="create-form"]',
		title: 'Create a database.',
		body: 'Choose 1 GB or 5 GB, name it, and press Create. You can hold two databases at once.'
	},
	{
		id: 'create-panel',
		route: '/dashboard',
		selector: '[data-tour="create-panel"]',
		title: 'Copy the password now.',
		body: 'The create panel shows the password and connection string once. Copy them before you dismiss the panel. The next reset deletes the database.'
	}
]);
