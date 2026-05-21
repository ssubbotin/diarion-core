<script lang="ts">
	type Props = {
		nextCursor?: string;
		buildHref: (cursor: string) => string;
	};
	let { nextCursor, buildHref }: Props = $props();
</script>

{#if nextCursor}
	<nav aria-label="Pagination" class="mt-8 flex justify-center">
		<a
			href={buildHref(nextCursor)}
			class="rounded-md border border-border px-4 py-2 text-sm font-medium hover:bg-accent"
			rel="next"
		>
			Older entries →
		</a>
	</nav>
{/if}
