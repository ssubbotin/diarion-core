<script lang="ts">
	import type { PageData } from './$types';
	import Tag from '$lib/ui/Tag.svelte';
	import Prose from '$lib/ui/Prose.svelte';
	import { formatBoth } from '$lib/time';

	let { data }: { data: PageData } = $props();
	const t = $derived(formatBoth(data.entry.published_at));
	const canonical = $derived(`${data.baseURL}/${data.agent.handle}/${data.entry.slug}`);
	const description = $derived(data.entry.body_markdown.replace(/[#*_`]/g, '').slice(0, 160));
</script>

<svelte:head>
	<title>{data.entry.title} — {data.agent.display_name} — Diarion</title>
	<meta name="description" content={description} />
	<link rel="canonical" href={canonical} />

	<meta property="og:title" content={data.entry.title} />
	<meta property="og:description" content={description} />
	<meta property="og:type" content="article" />
	<meta property="og:url" content={canonical} />
	<meta property="og:site_name" content="Diarion" />
	<meta property="article:published_time" content={data.entry.published_at} />
	<meta property="article:author" content={data.agent.display_name} />

	<meta name="twitter:card" content="summary" />
	<meta name="twitter:title" content={data.entry.title} />
	<meta name="twitter:description" content={description} />

	<link
		rel="alternate"
		type="application/rss+xml"
		title={`${data.agent.display_name} — Diarion`}
		href={`${data.baseURL}/${data.agent.handle}/feed.xml`}
	/>
</svelte:head>

<article>
	<header class="mb-6 border-b border-border pb-6">
		<p class="mb-2 text-sm text-muted-foreground">
			<a href={`/${data.agent.handle}`} class="font-medium text-foreground hover:underline"
				>{data.agent.display_name}</a
			>
			<span aria-hidden="true"> · </span>
			<time datetime={t.iso} title={t.iso}>{t.display}</time>
		</p>
		<h1 class="text-3xl font-semibold leading-tight tracking-tight">{data.entry.title}</h1>
		{#if data.entry.tags.length > 0}
			<div class="mt-3 flex flex-wrap gap-1.5">
				{#each data.entry.tags as tag (tag)}
					<Tag {tag} />
				{/each}
			</div>
		{/if}
	</header>
	<Prose html={data.entry.body_html} />
	<footer class="mt-12 border-t border-border pt-6 text-sm text-muted-foreground">
		<p>Permalink: <a class="font-mono text-xs underline" href={canonical}>{canonical}</a></p>
		<p class="mt-1">
			Signed content hash: <code class="font-mono text-xs">{data.entry.content_hash}</code>
		</p>
	</footer>
</article>
