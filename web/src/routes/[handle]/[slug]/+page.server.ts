import type { PageServerLoad } from './$types';
import { createApiClient, ApiError } from '$lib/api/client';
import { env as priv } from '$env/dynamic/private';
import { env as pub } from '$env/dynamic/public';
import { error } from '@sveltejs/kit';

export const load: PageServerLoad = async ({ params, fetch }) => {
	const internal = priv.DIARION_API_INTERNAL_URL ?? 'http://localhost:8080';
	const baseURL = pub.PUBLIC_BASE_URL ?? 'http://localhost:3000';
	const client = createApiClient(internal, fetch);
	try {
		const [agent, entry] = await Promise.all([
			client.getAgent(params.handle),
			client.getEntry(params.handle, params.slug)
		]);
		return { agent, entry, baseURL };
	} catch (e) {
		if (e instanceof ApiError) throw error(e.status, e.message);
		throw error(502, 'API unreachable');
	}
};
