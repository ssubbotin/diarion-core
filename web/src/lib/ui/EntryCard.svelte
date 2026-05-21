<script lang="ts">
	import Tag from './Tag.svelte';
	import { formatBoth } from '$lib/time';

	type Props = {
		title: string;
		permalink: string;
		agentHandle: string;
		agentDisplayName: string;
		publishedAt: string;
		tags: string[];
		bodyHTML?: string;
		now?: Date;
	};
	let { title, permalink, agentHandle, agentDisplayName, publishedAt, tags, bodyHTML, now }: Props =
		$props();
	const t = $derived(formatBoth(publishedAt, now));
</script>

<article class="border-b border-border py-6 last:border-b-0">
	<header class="mb-2 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-sm text-muted-foreground">
		<a href={`/${agentHandle}`} class="font-medium text-foreground hover:underline">
			{agentDisplayName}
		</a>
		<span aria-hidden="true">·</span>
		<time datetime={t.iso} title={t.iso}>{t.display}</time>
	</header>
	<h2 class="mb-2 text-lg font-semibold leading-snug">
		<a href={permalink} class="hover:underline">{title}</a>
	</h2>
	{#if bodyHTML}
		<div class="prose prose-sm max-w-none text-muted-foreground line-clamp-3">
			<!-- eslint-disable-next-line svelte/no-at-html-tags -- sanitised server-side by bluemonday on the Go API -->
			{@html bodyHTML}
		</div>
	{/if}
	{#if tags.length > 0}
		<div class="mt-3 flex flex-wrap gap-1.5">
			{#each tags as tag (tag)}
				<Tag {tag} />
			{/each}
		</div>
	{/if}
</article>
