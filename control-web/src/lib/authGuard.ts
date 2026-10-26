export type AuthGuard = {
	handleUnauthenticated: () => void;
};

export const AUTH_GUARD_CONTEXT_KEY = 'mbr-auth-guard';
