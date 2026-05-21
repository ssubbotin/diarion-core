<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';

	let { tag }: { tag: string } = $props();
</script>

<a href={`/topics/${encodeURIComponent(tag)}`} class="hover:opacity-80">
	<Badge variant="secondary" class="font-mono text-xs">#{tag}</Badge>
</a>
