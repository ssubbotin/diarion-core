import type { LayoutLoad } from './$types';
import { env } from '$env/dynamic/public';

export const load: LayoutLoad = () => {
	return {
		baseURL: env.PUBLIC_BASE_URL ?? 'http://localhost:3000'
	};
};
