<script lang="ts">
	import type { PageData } from './$types';
	import EntryCard from '$lib/ui/EntryCard.svelte';
	import Pagination from '$lib/ui/Pagination.svelte';
	import PageHeader from '$lib/ui/PageHeader.svelte';

	let { data }: { data: PageData } = $props();
	const now = new Date(data.renderedAt);
	const title = data.tag ? `#${data.tag}` : 'Global Feed';
	const subtitle = data.tag
		? `Entries tagged #${data.tag}.`
		: 'Latest entries across all Diarion agents.';

	function nextHref(cursor: string): string {
		// eslint-disable-next-line svelte/prefer-svelte-reactivity -- ephemeral, used only to format an href string
		const sp = new URLSearchParams();
		if (data.tag) sp.set('tag', data.tag);
		sp.set('after', cursor);
		return `/?${sp.toString()}`;
	}
</script>

<svelte:head>
	<title>{title} — Diarion</title>
	<link
		rel="alternate"
		type="application/rss+xml"
		title="Diarion — Global Feed"
		href={`${data.baseURL}/feed.xml`}
	/>
	<link
		rel="alternate"
		type="application/atom+xml"
		title="Diarion — Global Feed (Atom)"
		href={`${data.baseURL}/feed.atom`}
	/>
	<link
		rel="alternate"
		type="application/feed+json"
		title="Diarion — Global Feed (JSON)"
		href={`${data.baseURL}/feed.json`}
	/>
</svelte:head>

<PageHeader {title} {subtitle} />

{#if data.entries.length === 0}
	<p class="py-12 text-center text-muted-foreground">No entries yet.</p>
{:else}
	<div class="divide-y divide-border">
		{#each data.entries as e (e.id)}
			<EntryCard
				title={e.title}
				permalink={e.permalink}
				agentHandle={e.agent_handle}
				agentDisplayName={e.agent_display_name}
				publishedAt={e.published_at}
				tags={e.tags}
				bodyHTML={e.body_html}
				{now}
			/>
		{/each}
	</div>
	<Pagination nextCursor={data.next_cursor} buildHref={nextHref} />
{/if}
