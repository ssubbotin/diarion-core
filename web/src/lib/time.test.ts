import { describe, it, expect } from 'vitest';
import { formatRelative, formatAbsolute, formatBoth } from './time';

describe('formatAbsolute', () => {
	it('renders YYYY-MM-DD style', () => {
		expect(formatAbsolute('2026-05-21T12:00:00Z')).toMatch(/May 21, 2026/);
	});
});

describe('formatRelative', () => {
	const now = new Date('2026-05-21T12:00:00Z');

	it('returns "just now" for <60s', () => {
		expect(formatRelative('2026-05-21T11:59:30Z', now)).toBe('just now');
	});

	it('returns minutes for <1h', () => {
		expect(formatRelative('2026-05-21T11:30:00Z', now)).toMatch(/30 min/);
	});

	it('returns hours for <24h', () => {
		expect(formatRelative('2026-05-21T06:00:00Z', now)).toMatch(/6 hr|hour/);
	});

	it('returns days for <7d', () => {
		expect(formatRelative('2026-05-19T12:00:00Z', now)).toMatch(/2 day/);
	});

	it('falls back to absolute date for >7d', () => {
		expect(formatRelative('2026-04-01T12:00:00Z', now)).toMatch(/Apr 1, 2026/);
	});
});

describe('formatBoth', () => {
	it('returns relative + ISO title', () => {
		const now = new Date('2026-05-21T12:00:00Z');
		const out = formatBoth('2026-05-21T11:00:00Z', now);
		expect(out.display).toMatch(/1 hr|hour/);
		expect(out.iso).toBe('2026-05-21T11:00:00.000Z');
	});
});
