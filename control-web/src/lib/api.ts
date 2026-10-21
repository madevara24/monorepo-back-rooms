export type User = {
	id: string;
	email: string;
	is_admin: boolean;
	created_at: string;
};

export type Database = {
	id: string;
	name: string;
	size_gb: number;
	status: string;
	public_host: string | null;
	created_at: string;
	plan_id: string | null;
	price_idr_month: number | null;
};

export type Plan = {
	id: string;
	slug: string;
	name: string;
	size_gb: number;
	price_idr_month: number;
	tagline: string;
	badge: string;
	sort_order: number;
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

export type AdminOverview = {
	users: number;
	databases_by_status: Record<string, number>;
	nodes_fresh: number;
	nodes_stale: number;
	jobs_by_status: Record<string, number>;
};

export type AdminUser = {
	id: string;
	email: string;
	created_at: string;
	database_count: number;
};

export type AdminDatabase = {
	id: string;
	name: string;
	size_gb: number;
	status: string;
	owner_email: string;
	node_id: string | null;
	host: string | null;
	port: number | null;
	created_at: string;
};

export type AdminContainer = {
	database_id: string;
	status: string;
	port: number;
	updated_at: string;
};

export type AdminNode = {
	id: string;
	name: string;
	address: string;
	capacity: Record<string, unknown>;
	version: string;
	last_heartbeat: string | null;
	containers: AdminContainer[];
};

export type AdminDatabaseMetric = {
	database_id: string;
	database_name: string;
	owner_email: string;
	active: number;
	idle: number;
	idle_in_transaction: number;
	used: number;
	max_connections: number;
	superuser_reserved_connections: number;
	used_percent: number;
	warning_level: 'ok' | 'warning' | 'critical';
	memory_limit_bytes: number;
	collected_at: string;
};

export type AdminJob = {
	id: string;
	kind: string;
	status: string;
	attempts: number;
	last_error: string | null;
	created_at: string;
};

export type CreateDatabaseResult = {
	id: string;
	host: string;
	port: number;
	database: string;
	user: string;
	password: string;
	connection_string: string;
	jdbc_url: string;
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
