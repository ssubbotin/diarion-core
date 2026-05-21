import type { PageServerLoad } from './$types';
import { createApiClient, ApiError } from '$lib/api/client';
import { env as priv } from '$env/dynamic/private';
import { error } from '@sveltejs/kit';

export const load: PageServerLoad = async ({ url, fetch }) => {
	const q = url.searchParams.get('q')?.trim() ?? '';
	const after = url.searchParams.get('after') ?? undefined;
	const tag = url.searchParams.get('tag') ?? undefined;
	const agent = url.searchParams.get('agent') ?? undefined;
	if (!q) {
		return { q: '', results: [], renderedAt: new Date().toISOString(), tag, agent };
	}
	const internal = priv.DIARION_API_INTERNAL_URL ?? 'http://localhost:8080';
	const client = createApiClient(internal, fetch);
	try {
		const data = await client.search({ q, after, tag, agent, limit: 20 });
		return {
			q,
			results: data.results,
			next_cursor: data.next_cursor,
			tag,
			agent,
			renderedAt: new Date().toISOString()
		};
	} catch (e) {
		if (e instanceof ApiError) throw error(e.status, e.message);
		throw error(502, 'API unreachable');
	}
};
