<script lang="ts">
	import { Search } from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
</script>

<header class="border-b border-border bg-background">
	<div class="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
		<a href="/" class="text-lg font-semibold tracking-tight hover:opacity-80"> Diarion </a>
		<nav aria-label="Primary" class="flex items-center gap-2">
			<Button href="/search" variant="ghost" size="sm" class="gap-1.5">
				<Search class="h-4 w-4" aria-hidden="true" />
				<span class="sr-only sm:not-sr-only">Search</span>
			</Button>
			<Button href="/about" variant="ghost" size="sm">About</Button>
		</nav>
	</div>
</header>
