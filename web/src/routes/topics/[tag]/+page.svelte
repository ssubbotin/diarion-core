<script lang="ts">
	import type { PageData } from './$types';
	import EntryCard from '$lib/ui/EntryCard.svelte';
	import Pagination from '$lib/ui/Pagination.svelte';
	import PageHeader from '$lib/ui/PageHeader.svelte';

	let { data }: { data: PageData } = $props();
	const now = $derived(new Date(data.renderedAt));

	function nextHref(cursor: string): string {
		// eslint-disable-next-line svelte/prefer-svelte-reactivity -- ephemeral, used only to format an href string
		const sp = new URLSearchParams();
		sp.set('after', cursor);
		return `/topics/${encodeURIComponent(data.tag)}?${sp.toString()}`;
	}
</script>

<svelte:head>
	<title>#{data.tag} — Diarion</title>
	<meta name="description" content={`Diary entries tagged #${data.tag} on Diarion.`} />
	<link
		rel="alternate"
		type="application/rss+xml"
		title={`#${data.tag} — Diarion`}
		href={`${data.baseURL}/topics/${encodeURIComponent(data.tag)}/feed.xml`}
	/>
	<link
		rel="alternate"
		type="application/atom+xml"
		title={`#${data.tag} — Diarion (Atom)`}
		href={`${data.baseURL}/topics/${encodeURIComponent(data.tag)}/feed.atom`}
	/>
	<link
		rel="alternate"
		type="application/feed+json"
		title={`#${data.tag} — Diarion (JSON)`}
		href={`${data.baseURL}/topics/${encodeURIComponent(data.tag)}/feed.json`}
	/>
</svelte:head>

<PageHeader title={`#${data.tag}`} subtitle={`Entries tagged #${data.tag}.`} />

{#if data.entries.length === 0}
	<p class="py-12 text-center text-muted-foreground">No entries tagged #{data.tag} yet.</p>
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
