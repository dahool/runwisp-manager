<script>
	import { onMount } from 'svelte';
	let status = $state(/** @type {any} */ (null));
	let error = $state('');
	let busy = $state(false);
	let conflicts = $state(/** @type {any[]} */ ([]));
	async function load() {
		const response = await fetch('/manager/api/git/status');
		if (!response.ok) throw new Error('Git status unavailable');
		status = await response.json();
	}
	/** @param {string} path @param {any} body */
	async function action(path, body = {}) {
		busy = true; error = '';
		try { const response = await fetch(path, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) }); if (!response.ok) throw new Error((await response.json()).detail || 'Git operation failed'); await load(); } catch (cause) { error = cause instanceof Error ? cause.message : 'Git operation failed'; } finally { busy = false; }
	}
	onMount(async () => { try { await load(); } catch (cause) { error = cause instanceof Error ? cause.message : 'Git status unavailable'; } });
</script>

<svelte:head><title>Git sync · RunWisp Manager</title></svelte:head>
<section class="git-page">
	<p class="kicker">Repository sync</p><h1>Git sync.</h1>
	{#if error}<div class="error" role="alert">{error}</div>{/if}
	{#if status}
		<div class="status"><strong>{status.branch || 'detached'}</strong><span>{status.ahead ? `↑${status.ahead}` : ''} {status.behind ? `↓${status.behind}` : ''} {status.clean ? 'clean' : 'local changes'}</span>{#if status.inProgress}<span class="warn">{status.inProgress} in progress</span>{/if}</div>
		<div class="actions"><button disabled={busy} onclick={() => action('/manager/api/git/fetch')}>Fetch</button><button disabled={busy} onclick={() => action('/manager/api/git/pull', { strategy: 'ff-only' })}>Pull</button><button disabled={busy} onclick={() => action('/manager/api/git/push', { force: false })}>Push</button><button disabled={busy} onclick={async () => { const response = await fetch('/manager/api/git/conflicts'); conflicts = response.ok ? (await response.json()).conflicts : []; }}>Conflicts</button></div>
		{#if conflicts.length}<h2>Conflicts</h2><ul>{#each conflicts as conflict}<li><code>{conflict.path}</code></li>{/each}</ul>{/if}
	{:else if !error}<p>Loading repository status…</p>{/if}
</section>

<style>
	.git-page { max-width: 760px; padding: 8vh 2vw; } .kicker { font: 11px ui-monospace, monospace; opacity: .55; } h1 { font-size: clamp(34px, 5vw, 58px); letter-spacing: -.06em; margin: 0 0 28px; } .status { display: flex; gap: 18px; flex-wrap: wrap; border-block: 1px solid var(--color-border, #d8dee8); padding: 18px 0; font-size: 13px; } .status span { opacity: .62; } .warn, .error { color: #a45b2a; } .error { padding: 12px 0; font-size: 12px; } .actions { display: flex; gap: 8px; flex-wrap: wrap; margin: 24px 0; } button { border: 1px solid currentColor; background: transparent; color: inherit; padding: 9px 13px; cursor: pointer; font: inherit; font-size: 12px; } button:disabled { opacity: .45; cursor: wait; } h2 { font-size: 18px; } li { margin: 8px 0; font-size: 12px; }
</style>
