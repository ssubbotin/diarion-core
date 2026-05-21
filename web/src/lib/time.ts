const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;

const absoluteFmt = new Intl.DateTimeFormat('en-US', {
	year: 'numeric',
	month: 'short',
	day: 'numeric'
});

const relTimeFmt = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

/** Render the timestamp in absolute form (e.g. "May 21, 2026"). */
export function formatAbsolute(rfc3339: string): string {
	return absoluteFmt.format(new Date(rfc3339));
}

/**
 * Render the timestamp relative to `now` if within a week; otherwise fall
 * back to the absolute form. SSR-safe — pass a fixed `now` from the server
 * so client and server render identically.
 */
export function formatRelative(rfc3339: string, now: Date = new Date()): string {
	const then = new Date(rfc3339);
	const diff = now.getTime() - then.getTime();
	if (diff < MINUTE) return 'just now';
	if (diff < HOUR) return relTimeFmt.format(-Math.round(diff / MINUTE), 'minute');
	if (diff < DAY) return relTimeFmt.format(-Math.round(diff / HOUR), 'hour');
	if (diff < WEEK) return relTimeFmt.format(-Math.round(diff / DAY), 'day');
	return formatAbsolute(rfc3339);
}

/** Returns both a human-readable display string and the ISO form for <time title=...>. */
export function formatBoth(
	rfc3339: string,
	now: Date = new Date()
): { display: string; iso: string } {
	return {
		display: formatRelative(rfc3339, now),
		iso: new Date(rfc3339).toISOString()
	};
}
