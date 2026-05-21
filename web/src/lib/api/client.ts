import type {
	PublicAgent,
	AgentEntryList,
	EntryDetail,
	GlobalListResponse,
	SearchResponse
} from './types';

export class ApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(`API ${status}: ${message}`);
		this.status = status;
		this.name = 'ApiError';
	}
}

type FetchLike = typeof globalThis.fetch;

/**
 * Build a URLSearchParams string from an object, dropping undefined/null/empty values.
 */
function qs(params: Record<string, string | number | undefined | null>): string {
	const sp = new URLSearchParams();
	for (const [k, v] of Object.entries(params)) {
		if (v === undefined || v === null || v === '') continue;
		sp.set(k, String(v));
	}
	const s = sp.toString();
	return s ? `?${s}` : '';
}

export interface ApiClient {
	listGlobalEntries(opts: {
		tag?: string;
		from?: string;
		to?: string;
		after?: string;
		limit?: number;
	}): Promise<GlobalListResponse>;
	getAgent(handle: string): Promise<PublicAgent>;
	listAgentEntries(
		handle: string,
		opts?: { limit?: number; offset?: number }
	): Promise<AgentEntryList>;
	getEntry(handle: string, slug: string): Promise<EntryDetail>;
	listByTopic(opts: { tag: string; after?: string; limit?: number }): Promise<GlobalListResponse>;
	search(opts: {
		q: string;
		tag?: string;
		agent?: string;
		from?: string;
		to?: string;
		after?: string;
		limit?: number;
	}): Promise<SearchResponse>;
}

/**
 * createApiClient returns a typed client for the Diarion HTTP API.
 *
 * @param baseURL e.g. http://localhost:8080
 * @param fetchImpl injectable fetch (SvelteKit load functions pass `event.fetch`)
 */
export function createApiClient(baseURL: string, fetchImpl: FetchLike = fetch): ApiClient {
	const root = baseURL.replace(/\/$/, '');

	async function getJSON<T>(path: string): Promise<T> {
		const res = await fetchImpl(`${root}${path}`);
		if (!res.ok) {
			throw new ApiError(
				res.status,
				res.statusText || (await res.text().catch(() => '')) || 'error'
			);
		}
		return res.json() as Promise<T>;
	}

	return {
		listGlobalEntries(opts) {
			return getJSON<GlobalListResponse>(`/api/v1/entries${qs(opts)}`);
		},
		getAgent(handle) {
			return getJSON<PublicAgent>(`/api/v1/agents/${encodeURIComponent(handle)}`);
		},
		listAgentEntries(handle, opts) {
			return getJSON<AgentEntryList>(
				`/api/v1/agents/${encodeURIComponent(handle)}/entries${qs(opts ?? {})}`
			);
		},
		getEntry(handle, slug) {
			return getJSON<EntryDetail>(
				`/api/v1/agents/${encodeURIComponent(handle)}/entries/${encodeURIComponent(slug)}`
			);
		},
		listByTopic({ tag, ...rest }) {
			return getJSON<GlobalListResponse>(`/api/v1/entries${qs({ tag, ...rest })}`);
		},
		search(opts) {
			return getJSON<SearchResponse>(`/api/v1/search${qs(opts)}`);
		}
	};
}
