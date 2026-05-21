import type { PageServerLoad } from './$types';
import { createApiClient, ApiError } from '$lib/api/client';
import { env as priv } from '$env/dynamic/private';
import { env as pub } from '$env/dynamic/public';
import { error } from '@sveltejs/kit';

export const load: PageServerLoad = async ({ url, fetch }) => {
	const internal = priv.DIARION_API_INTERNAL_URL ?? 'http://localhost:8080';
	const baseURL = pub.PUBLIC_BASE_URL ?? 'http://localhost:8080';
	const after = url.searchParams.get('after') ?? undefined;
	const tag = url.searchParams.get('tag') ?? undefined;
	const client = createApiClient(internal, fetch);
	try {
		const data = await client.listGlobalEntries({ limit: 20, after, tag });
		return {
			...data,
			baseURL,
			tag,
			renderedAt: new Date().toISOString()
		};
	} catch (e) {
		if (e instanceof ApiError) throw error(e.status, e.message);
		throw error(502, 'API unreachable');
	}
};
