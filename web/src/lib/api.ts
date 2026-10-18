export type User = {
	id: string;
	email: string;
	created_at: string;
};

export type Database = {
	id: string;
	name: string;
	size_gb: number;
	status: string;
	created_at: string;
};

export type Invoice = {
	id: string;
	status: string;
	total_amount_idr: number;
	period_start: string;
	period_end: string;
	created_at: string;
	paid_at: string | null;
};

export function postJSON(path: string, body?: unknown): Promise<Response> {
	return fetch(path, {
		method: 'POST',
		headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
		body: body === undefined ? undefined : JSON.stringify(body)
	});
}

export async function errorMessage(res: Response): Promise<string> {
	try {
		const data = await res.json();
		if (typeof data?.error === 'string') return data.error;
	} catch {
		// response had no JSON body; fall through to the generic message
	}
	return `request failed (${res.status})`;
}
