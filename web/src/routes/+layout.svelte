<script>
	import '../app.css';
	import ManagerLayout from '$lib/layouts/ManagerLayout.svelte';

	let { children } = $props();
</script>

<ManagerLayout>
		{@render children()}
</ManagerLayout>
