import type { User } from '$lib/api';

export type Session = {
	readonly user: User | null;
	readonly signedIn: boolean;
	readonly isAdmin: boolean;
	readonly loading: boolean;
	refresh: () => Promise<void>;
	clear: () => void;
};

export const SESSION_CONTEXT_KEY = 'mbr-session';

export function createSession(): Session {
	let user: User | null = $state<User | null>(null);
	let loading: boolean = $state(true);

	const signedIn = $derived(user !== null);
	const isAdmin = $derived(user?.is_admin === true);

	async function refresh() {
		loading = true;
		try {
			const res = await fetch('/api/me');
			user = res.ok ? await res.json() : null;
		} catch {
			user = null;
		} finally {
			loading = false;
		}
	}

	function clear() {
		user = null;
	}

	return {
		get user() {
			return user;
		},
		get signedIn() {
			return signedIn;
		},
		get isAdmin() {
			return isAdmin;
		},
		get loading() {
			return loading;
		},
		refresh,
		clear
	};
}
