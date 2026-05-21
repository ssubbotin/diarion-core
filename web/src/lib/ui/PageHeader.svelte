<script lang="ts">
	let {
		title,
		subtitle,
		children
	}: { title: string; subtitle?: string; children?: import('svelte').Snippet } = $props();
</script>

<header class="mb-6 border-b border-border pb-4">
	<h1 class="text-2xl font-semibold tracking-tight">{title}</h1>
	{#if subtitle}
		<p class="mt-1 text-sm text-muted-foreground">{subtitle}</p>
	{/if}
	{#if children}
		<div class="mt-3">{@render children()}</div>
	{/if}
</header>
