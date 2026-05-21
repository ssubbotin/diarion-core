import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createApiClient, ApiError } from './client';

describe('createApiClient', () => {
	let fetchMock: ReturnType<typeof vi.fn>;

	beforeEach(() => {
		fetchMock = vi.fn();
	});

	function jsonResponse(body: unknown, status = 200): Response {
		return new Response(JSON.stringify(body), {
			status,
			headers: { 'Content-Type': 'application/json' }
		});
	}

	it('listGlobalEntries fetches /api/v1/entries with limit + cursor', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({ entries: [], limit: 20, next_cursor: 'abc' }));
		const client = createApiClient('http://api.test', fetchMock as unknown as typeof fetch);
		const res = await client.listGlobalEntries({ limit: 20, after: 'xyz' });
		expect(res.next_cursor).toBe('abc');

		const [url] = fetchMock.mock.calls[0];
		expect(String(url)).toContain('/api/v1/entries');
		expect(String(url)).toContain('limit=20');
		expect(String(url)).toContain('after=xyz');
	});

	it('getAgent fetches /api/v1/agents/{handle}', async () => {
		fetchMock.mockResolvedValueOnce(
			jsonResponse({
				handle: 'ada',
				display_name: 'Ada',
				key_custody: 'managed',
				created_at: '2026-05-21T00:00:00Z'
			})
		);
		const client = createApiClient('http://api.test', fetchMock as unknown as typeof fetch);
		const agent = await client.getAgent('ada');
		expect(agent.handle).toBe('ada');
		expect(fetchMock.mock.calls[0][0]).toBe('http://api.test/api/v1/agents/ada');
	});

	it('throws ApiError on non-2xx', async () => {
		fetchMock.mockResolvedValue(new Response('not found', { status: 404 }));
		const client = createApiClient('http://api.test', fetchMock as unknown as typeof fetch);
		await expect(client.getAgent('ghost')).rejects.toThrow(ApiError);
		await expect(client.getAgent('ghost')).rejects.toThrow(/404/);
	});

	it('search includes ?q + optional filters', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({ results: [], limit: 20 }));
		const client = createApiClient('http://api.test', fetchMock as unknown as typeof fetch);
		await client.search({ q: 'rust', tag: 'lang', agent: 'ada', limit: 10 });
		const url = String(fetchMock.mock.calls[0][0]);
		expect(url).toContain('q=rust');
		expect(url).toContain('tag=lang');
		expect(url).toContain('agent=ada');
		expect(url).toContain('limit=10');
	});

	it('search drops empty optional filters', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({ results: [], limit: 20 }));
		const client = createApiClient('http://api.test', fetchMock as unknown as typeof fetch);
		await client.search({ q: 'rust' });
		const url = String(fetchMock.mock.calls[0][0]);
		expect(url).not.toContain('tag=');
		expect(url).not.toContain('agent=');
	});

	it('getEntry fetches /api/v1/agents/{handle}/entries/{slug}', async () => {
		fetchMock.mockResolvedValueOnce(
			jsonResponse({
				id: 1,
				agent_handle: 'ada',
				slug: 'p1',
				title: 'P1',
				body_markdown: '#hi',
				body_html: '<h1>hi</h1>',
				tags: [],
				content_hash: 'abc',
				published_at: '2026-05-21T00:00:00Z',
				permalink: '/ada/p1'
			})
		);
		const client = createApiClient('http://api.test', fetchMock as unknown as typeof fetch);
		const entry = await client.getEntry('ada', 'p1');
		expect(entry.title).toBe('P1');
	});

	it('listByTopic builds /api/v1/entries?tag=...', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({ entries: [], limit: 20 }));
		const client = createApiClient('http://api.test', fetchMock as unknown as typeof fetch);
		await client.listByTopic({ tag: 'rust', limit: 5 });
		const url = String(fetchMock.mock.calls[0][0]);
		expect(url).toContain('/api/v1/entries');
		expect(url).toContain('tag=rust');
		expect(url).toContain('limit=5');
	});
});
