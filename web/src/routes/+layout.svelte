<script lang="ts">
	import '../app.css';
	import SiteHeader from '$lib/ui/SiteHeader.svelte';
	import SiteFooter from '$lib/ui/SiteFooter.svelte';

	let { children } = $props();
</script>

<a href="#main" class="skip-link">Skip to content</a>
<SiteHeader />
<main id="main" class="mx-auto max-w-5xl px-4 py-6">
	{@render children()}
</main>
<SiteFooter />
