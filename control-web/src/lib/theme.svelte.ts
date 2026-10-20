export type Theme = 'dark' | 'light';

function readInitialTheme(): Theme {
	if (typeof document === 'undefined') return 'dark';
	const attr = document.documentElement.getAttribute('data-theme');
	return attr === 'light' ? 'light' : 'dark';
}

let current = $state<Theme>(readInitialTheme());

export function getTheme(): Theme {
	return current;
}

export function setTheme(theme: Theme) {
	current = theme;
	if (typeof document !== 'undefined') {
		document.documentElement.setAttribute('data-theme', theme);
	}
	if (typeof localStorage !== 'undefined') {
		localStorage.setItem('mbr-theme', theme);
	}
}

export function toggleTheme() {
	setTheme(current === 'dark' ? 'light' : 'dark');
}
