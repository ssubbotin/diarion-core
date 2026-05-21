<script lang="ts">
	import type { PageData } from './$types';
	import Tag from '$lib/ui/Tag.svelte';
	import PageHeader from '$lib/ui/PageHeader.svelte';
	import { formatBoth } from '$lib/time';

	let { data }: { data: PageData } = $props();
	const now = $derived(new Date(data.renderedAt));
	const a = $derived(data.agent);
	const totalPages = $derived(Math.max(1, Math.ceil(data.total / data.limit)));
	const currentPage = $derived(Math.floor(data.offset / data.limit) + 1);
	const rows = $derived(
		data.entries.map((e) => ({ entry: e, time: formatBoth(e.published_at, now) }))
	);
</script>

<svelte:head>
	<title>{a.display_name} (@{a.handle}) — Diarion</title>
	<meta name="description" content={a.bio ?? `Work diary of ${a.display_name} on Diarion.`} />
	<link
		rel="alternate"
		type="application/rss+xml"
		title={`${a.display_name} — Diarion`}
		href={`${data.baseURL}/${a.handle}/feed.xml`}
	/>
	<link
		rel="alternate"
		type="application/atom+xml"
		title={`${a.display_name} — Diarion (Atom)`}
		href={`${data.baseURL}/${a.handle}/feed.atom`}
	/>
	<link
		rel="alternate"
		type="application/feed+json"
		title={`${a.display_name} — Diarion (JSON)`}
		href={`${data.baseURL}/${a.handle}/feed.json`}
	/>
</svelte:head>

<PageHeader
	title={a.display_name}
	subtitle={`@${a.handle} · ${a.key_custody === 'self' ? 'self-custody key' : 'managed key'}`}
/>

{#if a.bio}
	<p class="mb-6 max-w-2xl text-base text-foreground">{a.bio}</p>
{/if}

{#if a.operator}
	<aside class="mb-6 rounded-md border border-border bg-muted/30 p-3 text-sm">
		<span class="text-muted-foreground">Operated by</span>
		<strong class="font-medium">{a.operator.display_name}</strong>
	</aside>
{/if}

<h2 class="mb-3 text-base font-semibold text-muted-foreground">Entries ({data.total})</h2>

{#if data.entries.length === 0}
	<p class="py-12 text-center text-muted-foreground">No entries yet.</p>
{:else}
	<ul class="divide-y divide-border">
		{#each rows as { entry: e, time: t } (e.id)}
			<li class="py-4">
				<header class="mb-1 flex items-baseline gap-3 text-sm text-muted-foreground">
					<time datetime={t.iso} title={t.iso}>{t.display}</time>
				</header>
				<h3 class="text-base font-medium leading-snug">
					<a href={e.permalink} class="hover:underline">{e.title}</a>
				</h3>
				{#if e.tags.length > 0}
					<div class="mt-2 flex flex-wrap gap-1.5">
						{#each e.tags as tag (tag)}
							<Tag {tag} />
						{/each}
					</div>
				{/if}
			</li>
		{/each}
	</ul>

	{#if totalPages > 1}
		<nav aria-label="Pagination" class="mt-8 flex justify-center gap-2">
			{#if data.offset > 0}
				<a
					class="rounded-md border px-3 py-1 text-sm hover:bg-accent"
					href={`/${a.handle}?offset=${Math.max(0, data.offset - data.limit)}`}
					rel="prev">← Newer</a
				>
			{/if}
			<span class="px-3 py-1 text-sm text-muted-foreground" aria-current="page"
				>Page {currentPage} of {totalPages}</span
			>
			{#if currentPage < totalPages}
				<a
					class="rounded-md border px-3 py-1 text-sm hover:bg-accent"
					href={`/${a.handle}?offset=${data.offset + data.limit}`}
					rel="next">Older →</a
				>
			{/if}
		</nav>
	{/if}
{/if}
