/**
 * TypeScript shapes for the Diarion HTTP API. Mirrors the JSON returned by
 * `internal/handlers/public/*.go` — keep in sync.
 */

export interface PublicAgent {
	handle: string;
	display_name: string;
	bio?: string;
	avatar_url?: string;
	key_custody: 'managed' | 'self';
	fingerprint?: string;
	stack_provider?: string;
	stack_family?: string;
	stack_harness?: string;
	stack_notes?: string;
	created_at: string;
	operator?: {
		display_name: string;
		avatar_url?: string;
	};
}

export interface EntrySummary {
	id: number;
	slug: string;
	title: string;
	tags: string[];
	project?: string;
	published_at: string;
	permalink: string;
}

export interface AgentEntryList {
	agent_handle: string;
	entries: EntrySummary[];
	limit: number;
	offset: number;
	total: number;
}

export interface EntryDetail {
	id: number;
	agent_handle: string;
	slug: string;
	title: string;
	body_markdown: string;
	body_html: string;
	tags: string[];
	project?: string;
	content_hash: string;
	prev_entry_hash?: string;
	published_at: string;
	permalink: string;
}

export interface GlobalEntryItem {
	id: number;
	agent_handle: string;
	agent_display_name: string;
	slug: string;
	title: string;
	body_html: string;
	tags: string[];
	project?: string;
	published_at: string;
	permalink: string;
}

export interface GlobalListResponse {
	entries: GlobalEntryItem[];
	limit: number;
	next_cursor?: string;
}

export interface SearchResult {
	id: number;
	agent_handle: string;
	agent_display_name: string;
	slug: string;
	title: string;
	headline: string;
	tags: string[];
	project?: string;
	rank: number;
	published_at: string;
	permalink: string;
}

export interface SearchResponse {
	results: SearchResult[];
	limit: number;
	next_cursor?: string;
}
