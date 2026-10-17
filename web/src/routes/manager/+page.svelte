<script>
	import { onMount } from 'svelte';

	const version = 'dev';
	let status = 'Checking RunWisp…';
	let reachable = false;
	onMount(async () => {
		try {
			const response = await fetch('/manager/api/status');
			const data = await response.json();
			reachable = response.ok && !data.lastError;
			status = reachable ? `RunWisp ${data.daemon?.version || 'connected'}` : 'RunWisp unreachable';
		} catch {
			status = 'RunWisp unreachable';
		}
	});
</script>

<svelte:head><title>Manager · RunWisp</title></svelte:head>
<section class="welcome">
	<p class="kicker">RunWisp Manager</p>
	<h1>Configuration, without the guesswork.</h1>
	<p class="intro">The manager shell is ready. Editing, validation, and safe apply arrive in the next phases.</p>
	<div class="status"><span class:offline={!reachable} class="dot"></span><span>{status}</span><span class="version">{version}</span></div>
</section>

<style>
	.welcome { max-width: 680px; padding: 12vh 3vw; }
	.kicker { font: 11px ui-monospace, monospace; opacity: .55; margin: 0 0 18px; }
	h1 { max-width: 560px; font-size: clamp(32px, 5vw, 58px); line-height: 1.02; letter-spacing: -.06em; font-weight: 560; margin: 0 0 22px; }
	.intro { max-width: 480px; line-height: 1.65; font-size: 14px; opacity: .68; margin: 0; }
	.status { display: flex; align-items: center; gap: 9px; font: 11px ui-monospace, monospace; margin-top: 44px; }
	.dot { width: 7px; height: 7px; border-radius: 50%; background: #3eaa72; }
	.dot.offline { background: #c87952; }
	.version { opacity: .45; margin-left: 10px; }
</style>
