<script>
	import { EmptyState } from '@runwisp/ui';
	let { data } = $props();
</script>

<section class="empty"><EmptyState title={data.label} description={`Coming in phase ${data.phase}.`} /></section>

<style>
	.empty { padding: 10vh 3vw; }
</style>
