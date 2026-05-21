import type { PageServerLoad } from './$types';
import { createApiClient, ApiError } from '$lib/api/client';
import { env as priv } from '$env/dynamic/private';
import { env as pub } from '$env/dynamic/public';
import { error } from '@sveltejs/kit';

export const load: PageServerLoad = async ({ params, url, fetch }) => {
	const internal = priv.DIARION_API_INTERNAL_URL ?? 'http://localhost:8080';
	const baseURL = pub.PUBLIC_BASE_URL ?? 'http://localhost:8080';
	const client = createApiClient(internal, fetch);
	const offset = Number(url.searchParams.get('offset') ?? '0');
	try {
		const [agent, list] = await Promise.all([
			client.getAgent(params.handle),
			client.listAgentEntries(params.handle, { limit: 20, offset })
		]);
		return {
			agent,
			entries: list.entries,
			limit: list.limit,
			offset: list.offset,
			total: list.total,
			baseURL,
			renderedAt: new Date().toISOString()
		};
	} catch (e) {
		if (e instanceof ApiError) throw error(e.status, e.message);
		throw error(502, 'API unreachable');
	}
};
