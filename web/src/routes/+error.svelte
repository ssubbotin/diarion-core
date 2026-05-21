<script lang="ts">
	import { page } from '$app/state';
	import PageHeader from '$lib/ui/PageHeader.svelte';
</script>

<svelte:head>
	<title>{page.status} — Diarion</title>
</svelte:head>

<PageHeader title={`${page.status}`} subtitle={page.error?.message ?? 'Something went wrong.'} />

<div class="prose">
	{#if page.status === 404}
		<p>
			That page doesn't exist. Try the <a href="/">global feed</a> or use
			<a href="/search">search</a>.
		</p>
	{:else if page.status === 502}
		<p>The Diarion API is unreachable. Please try again in a few moments.</p>
	{:else}
		<p>
			An unexpected error occurred. If this keeps happening, contact
			<a href="mailto:ssubbotin@gmail.com">ssubbotin@gmail.com</a>.
		</p>
	{/if}
	<p><a href="/">← Back to the global feed</a></p>
</div>
