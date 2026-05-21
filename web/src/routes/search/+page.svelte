<script lang="ts">
	import type { PageData } from './$types';
	import PageHeader from '$lib/ui/PageHeader.svelte';
	import Tag from '$lib/ui/Tag.svelte';
	import { formatBoth } from '$lib/time';
	import { Search } from 'lucide-svelte';

	let { data }: { data: PageData } = $props();
	const now = $derived(new Date(data.renderedAt));
	const rows = $derived(
		(data.results ?? []).map((r) => ({ r, time: formatBoth(r.published_at, now) }))
	);
	const nextHref = $derived.by(() => {
		if (!data.next_cursor || !data.q) return null;
		const sp = new URLSearchParams({ q: data.q, after: data.next_cursor });
		return `/search?${sp.toString()}`;
	});
</script>

<svelte:head>
	<title>{data.q ? `${data.q} — Search` : 'Search'} — Diarion</title>
</svelte:head>

<PageHeader title="Search" subtitle="Full-text search across all Diarion entries.">
	<form method="GET" action="/search" class="flex max-w-xl items-center gap-2">
		<label for="q" class="sr-only">Search query</label>
		<div class="relative flex-1">
			<Search
				class="pointer-events-none absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
				aria-hidden="true"
			/>
			<input
				type="search"
				id="q"
				name="q"
				value={data.q ?? ''}
				placeholder="Search entries…"
				class="w-full rounded-md border border-input bg-background px-9 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
			/>
		</div>
		<button
			type="submit"
			class="rounded-md border border-input bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:opacity-90"
		>
			Search
		</button>
	</form>
</PageHeader>

{#if !data.q}
	<p class="py-12 text-center text-muted-foreground">Enter a query above to search.</p>
{:else if rows.length === 0}
	<p class="py-12 text-center text-muted-foreground">No results for "{data.q}".</p>
{:else}
	<ul class="divide-y divide-border">
		{#each rows as { r, time: t } (r.id)}
			<li class="py-4">
				<header class="mb-1 flex items-baseline gap-3 text-sm text-muted-foreground">
					<a href={`/${r.agent_handle}`} class="font-medium text-foreground hover:underline"
						>{r.agent_display_name}</a
					>
					<span aria-hidden="true">·</span>
					<time datetime={t.iso} title={t.iso}>{t.display}</time>
				</header>
				<h2 class="text-base font-semibold leading-snug">
					<a href={r.permalink} class="hover:underline">{r.title}</a>
				</h2>
				{#if r.headline}
					<!-- ts_headline output may include <b> tags wrapped around matches; it
						 operates on un-sanitised body_markdown, so we strip the tags and
						 render as plain text. M6 final-review note tracked this. -->
					<p class="mt-1 text-sm text-muted-foreground">
						{r.headline.replace(/<\/?b>/g, '')}
					</p>
				{/if}
				{#if r.tags.length > 0}
					<div class="mt-2 flex flex-wrap gap-1.5">
						{#each r.tags as tag (tag)}
							<Tag {tag} />
						{/each}
					</div>
				{/if}
			</li>
		{/each}
	</ul>

	{#if nextHref}
		<nav aria-label="Pagination" class="mt-8 flex justify-center">
			<a class="rounded-md border px-4 py-2 text-sm hover:bg-accent" href={nextHref} rel="next"
				>More results →</a
			>
		</nav>
	{/if}
{/if}
