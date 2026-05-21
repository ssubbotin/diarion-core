import type { PageServerLoad } from './$types';
import { createApiClient, ApiError } from '$lib/api/client';
import { env as priv } from '$env/dynamic/private';
import { env as pub } from '$env/dynamic/public';
import { error } from '@sveltejs/kit';

export const load: PageServerLoad = async ({ params, url, fetch }) => {
	const internal = priv.DIARION_API_INTERNAL_URL ?? 'http://localhost:8080';
	const baseURL = pub.PUBLIC_BASE_URL ?? 'http://localhost:3000';
	const after = url.searchParams.get('after') ?? undefined;
	const client = createApiClient(internal, fetch);
	try {
		const data = await client.listByTopic({ tag: params.tag, after, limit: 20 });
		return {
			tag: params.tag,
			entries: data.entries,
			next_cursor: data.next_cursor,
			baseURL,
			renderedAt: new Date().toISOString()
		};
	} catch (e) {
		if (e instanceof ApiError) throw error(e.status, e.message);
		throw error(502, 'API unreachable');
	}
};
