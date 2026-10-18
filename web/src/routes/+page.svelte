<script>
	import { onMount } from 'svelte';
	/** @type {any} */ let tree = $state(null);
	let error = $state('');
	onMount(async () => { try { const response = await fetch('/manager/api/config/tree'); if (!response.ok) throw new Error(); tree = await response.json(); } catch { error = 'Configuration tree unavailable'; } });
</script>
<svelte:head><title>Manager · RunWisp configuration</title></svelte:head>
<section class="welcome"><p class="kicker">RunWisp Manager</p><h1>Configuration overview.</h1>{#if error}<p class="intro">{error}</p>{:else}<p class="intro">{tree?.files?.length ?? '…'} files · {tree?.units?.length ?? '…'} indexed units</p>{#if tree?.stale}<div class="notice">Configuration is stale.</div>{/if}<nav><a href="/manager/tasks">Tasks</a><a href="/manager/services">Services</a><a href="/manager/settings/files">Files</a></nav>{/if}</section>

<style>
	.welcome { max-width: 680px; padding: 12vh 3vw; }
	.kicker { font: 11px ui-monospace, monospace; opacity: .55; margin: 0 0 18px; }
	h1 { max-width: 560px; font-size: clamp(32px, 5vw, 58px); line-height: 1.02; letter-spacing: -.06em; font-weight: 560; margin: 0 0 22px; }
	.intro { max-width: 480px; line-height: 1.65; font-size: 14px; opacity: .68; margin: 0; }
	nav { display: flex; gap: 18px; margin-top: 35px; } nav a { color: inherit; font-size: 12px; } .notice { margin-top: 18px; font-size: 12px; color: #a45b2a; }
</style>
