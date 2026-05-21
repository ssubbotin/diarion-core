<script lang="ts">
	let { html }: { html: string } = $props();
</script>

<!-- eslint-disable-next-line svelte/no-at-html-tags -- sanitised server-side by bluemonday on the Go API -->
<div class="prose">{@html html}</div>
