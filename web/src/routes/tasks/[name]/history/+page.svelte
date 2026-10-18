<script>
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	let versions = $state(/** @type {any[]} */ ([])); let error = $state(''); let busy = $state(false);
	const name = () => page.params.name ?? '';
	async function load() { const response = await fetch(`/manager/api/units/task/${encodeURIComponent(name())}/history`); if (!response.ok) throw new Error('History unavailable'); versions = (await response.json()).versions ?? []; }
	/** @param {any} version */ async function restore(version) { if (!confirm('Restore this version through the save pipeline?')) return; busy = true; try { const response = await fetch(`/manager/api/units/task/${encodeURIComponent(name())}/history/${encodeURIComponent(version.id)}:restore`, { method: 'POST', headers: { 'content-type': 'application/json' } }); if (!response.ok) throw new Error('Restore failed'); await load(); } catch (cause) { error = cause instanceof Error ? cause.message : 'Restore failed'; } finally { busy = false; } }
	onMount(async () => { try { await load(); } catch (cause) { error = cause instanceof Error ? cause.message : 'History unavailable'; } });
</script>
<svelte:head><title>Task history · RunWisp Manager</title></svelte:head>
<section class="history"><p class="kicker">Task history</p><h1>{page.params.name}.</h1>{#if error}<p class="error">{error}</p>{/if}{#each versions as version}<article><div><strong>{version.source}</strong><time>{new Date(version.ts).toLocaleString()}</time><p>{version.summary || version.unitHash}</p></div><button disabled={busy} onclick={() => restore(version)}>Restore</button></article>{:else}<p>No recoverable versions.</p>{/each}</section>
<style>.history{max-width:760px;padding:7vh 2vw}.kicker{font:11px ui-monospace,monospace;opacity:.55}h1{font-size:clamp(34px,5vw,58px);letter-spacing:-.06em;margin:0 0 28px}.history article{display:flex;justify-content:space-between;gap:20px;border-top:1px solid var(--color-border,#d8dee8);padding:16px 0;font-size:12px}.history time{display:block;opacity:.55;margin-top:5px}.history p{opacity:.65}.history button{height:max-content;border:1px solid currentColor;background:transparent;color:inherit;padding:8px 12px;font:inherit}.error{color:#a45b2a}</style>
