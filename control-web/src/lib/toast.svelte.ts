export type ToastKind = 'success' | 'error';

export type ToastItem = {
	id: number;
	kind: ToastKind;
	message: string;
};

let nextId = 1;
let items = $state<ToastItem[]>([]);

export function getToasts(): ToastItem[] {
	return items;
}

export function pushToast(kind: ToastKind, message: string, timeoutMs = 4000) {
	const id = nextId++;
	items = [...items, { id, kind, message }];
	setTimeout(() => dismissToast(id), timeoutMs);
}

export function dismissToast(id: number) {
	items = items.filter((t) => t.id !== id);
}
