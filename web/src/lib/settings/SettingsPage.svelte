<script>
	// @ts-nocheck
	import SchemaForm from '$lib/editor/SchemaForm.svelte';
	import { editorApi } from '$lib/editor/api.js';
	import { diffOps, toSaveOps } from '$lib/editor/ops.js';
	let { section = 'files', name = '' } = $props();
	let tree = $state(null), schema = $state(null), unit = $state(null), form = $state({}), original = $state({});
	let routeOrder = $state([]), matchPattern = $state('task-*'), matchedTasks = $state([]), routeBusy = $state(false);
	let newFilePath = $state('includes/new.toml');
	let identity = $state(null), loading = $state(true), busy = $state(false), error = $state(''), notice = $state('');
	const singletonKinds = { defaults: 'defaults', daemon: 'daemon', storage: 'storage', notify: 'notify' };
	const titles = { defaults: 'Defaults', daemon: 'Daemon', storage: 'Storage', notify: 'Notifications', compose: 'Compose', files: 'Files', notifiers: 'Notifiers', routes: 'Routes' };
	const editable = $derived(['editor', 'admin'].includes(identity?.role));
	const adminOnly = $derived(section === 'daemon');
	const ops = $derived(diffOps(original, form));
	const nav = Object.keys(titles);
	const schemaRoot = $derived(schema?.resolvedSchema ?? schema?.schema ?? {});
	function sectionSchema() {
		const source = schemaRoot?.$defs?.[section] ?? schemaRoot?.properties?.[section] ?? { type: 'object', properties: Object.fromEntries(Object.keys(form).map((key) => [key, { type: typeof form[key] === 'boolean' ? 'boolean' : typeof form[key] === 'number' ? 'number' : 'string', title: key }])) };
		if (section !== 'daemon') return source;
		const restartKeys = new Set(['tls', 'metrics', 'metricsListener', 'allow_station_dispatch']);
		return { ...source, properties: Object.fromEntries(Object.entries(source.properties ?? {}).map(([key, definition]) => [key, { ...definition, restartRequired: restartKeys.has(key) }])) };
	}
	function label(value) { return value.replace(/([A-Z])/g, ' $1').replace(/^./, (letter) => letter.toUpperCase()); }
	async function load() {
		loading = true; error = ''; notice = ''; unit = null; form = {}; original = {};
		try {
			const [me, treeResult, schemaResult] = await Promise.all([fetch('/manager/api/me').then((r) => r.ok ? r.json() : null).catch(() => null), fetch('/manager/api/config/tree').then((r) => r.json()), editorApi.schema().catch(() => null)]);
			identity = me?.identity ?? me; tree = treeResult; schema = schemaResult;
			if (singletonKinds[section]) {
				unit = await editorApi.singleton(singletonKinds[section]);
			} else if (section === 'notifiers' || section === 'routes' || section === 'compose') {
				const response = await editorApi.units(section === 'notifiers' ? 'notifier' : section === 'routes' ? 'route' : 'composeAlias');
				const overrides = section === 'compose' ? await editorApi.units('composeOverride') : { units: [] };
				unit = { list: response?.units ?? [], overrides: overrides.units ?? [] }; routeOrder = [...unit.list];
			}
			if (unit?.fields) { form = structuredClone(unit.fields); original = structuredClone(form); }
		} catch (cause) { error = cause.message ?? 'Settings are unavailable'; }
		loading = false;
	}
	function reverseOverrides(key) {
		return (tree?.units ?? []).filter((item) => ['task', 'service'].includes(item.kind) && item.fields?.[key] !== undefined).map((item) => item.name);
	}
	function request(apply) {
		return { idempotencyKey: crypto.randomUUID(), base: { [unit?.fileId ?? unit?.file ?? '']: unit?.fileVersion ?? unit?.hash ?? '' }, ops: [{ file: unit?.fileId ?? unit?.file ?? '', ops: toSaveOps(ops) }], apply, reason: `edit ${section}` };
	}
	async function save(apply = false) {
		busy = true; error = ''; notice = '';
		try { const result = await editorApi.save(request(apply)); original = structuredClone(form); unit = { ...unit, hash: result?.hash ?? unit?.hash }; notice = apply ? 'Saved. Reload the daemon if the banner calls for it.' : 'Saved.'; }
		catch (cause) { error = cause.message ?? 'Could not save settings'; }
		busy = false;
	}
	async function moveRoute(index, direction) {
		const target = index + direction; if (target < 0 || target >= routeOrder.length || routeBusy) return;
		const current = routeOrder[index]; routeBusy = true; error = '';
		try { await editorApi.reorderRoutes({ idempotencyKey: crypto.randomUUID(), file: current.fileId, base: current.fileVersion, from: index, to: target, apply: false }); [routeOrder[index], routeOrder[target]] = [routeOrder[target], routeOrder[index]]; notice = 'Route order saved.'; }
		catch (cause) { error = cause.message ?? 'Could not reorder routes'; }
		routeBusy = false;
	}
	async function previewMatches() { const result = await editorApi.routeMatches(matchPattern).catch((cause) => ({ matches: [], error: cause.message })); matchedTasks = result.matches ?? []; if (result.error) error = result.error; }
	async function createInclude() { try { await editorApi.createFile({ path: newFilePath, template: 'empty' }); notice = 'Include file created. Add its glob under daemon settings.'; await load(); } catch (cause) { error = cause.message ?? 'Could not create include file'; } }
	$effect(() => { section; name; load(); });
</script>

{#if loading}<section class="state">Reading the configuration ledger…</section>
{:else if error && !unit}<section class="state error" role="alert">{error}</section>
{:else}<section class="settings-page">
	<header class="settings-head"><div><p class="eyebrow">Manager settings</p><h1>{titles[section] ?? 'Settings'}</h1><p class="muted">Global configuration lives outside task files. Changes use the same review and apply pipeline.</p></div><a class="file-link" href="/manager/settings/files">View file graph</a></header>
	<nav class="section-nav" aria-label="Settings sections">{#each nav as item}<a class:active={item === section} href={`/manager/settings/${item}`}>{titles[item]}</a>{/each}</nav>
	{#if adminOnly && identity?.role !== 'admin'}<div class="banner danger" role="alert">This section is available to administrators only.</div>
	{:else if singletonKinds[section] && unit}
		{#if section === 'daemon'}<div class="banner restart"><strong>Some daemon changes need a restart.</strong> TLS, metrics listener, and station dispatch take effect after a daemon restart. The manager will never restart it for you; use <code>docker compose restart &lt;daemon service&gt;</code>.</div>{/if}
		{#if error}<div class="banner danger" role="alert">{error}</div>{/if}{#if notice}<div class="banner success" role="status">{notice}</div>{/if}
		<div class="editor-layout"><div class="editor-copy"><span class="rail-mark">{section === 'defaults' ? 'Inherited values' : 'Live settings'}</span><p>{section === 'defaults' ? 'These values are the starting point for every task and service.' : `Edit the ${titles[section].toLowerCase()} block in its managed file.`}</p>{#if section === 'defaults'}<div class="overrides"><h2>Overrides</h2><p class="muted">A key appears here when a task or service sets its own value.</p>{#each Object.keys(form) as key}{@const names = reverseOverrides(key)}{#if names.length}<details><summary>{label(key)} <span>{names.length}</span></summary><p>{names.join(', ')}</p></details>{/if}{/each}</div>{/if}</div><div class="form-pane"><SchemaForm schema={sectionSchema()} value={form} readOnly={!editable} onChange={(next) => { form = next; notice = ''; }} /></div></div>
		{#if ops.length}<footer class="savebar"><span><strong>{ops.length}</strong> unsaved {ops.length === 1 ? 'change' : 'changes'}</span><button class="quiet" onclick={() => form = structuredClone(original)}>Discard</button><button class="quiet" disabled={!editable || busy} onclick={() => save(false)}>Save</button><button class="primary" disabled={!editable || busy} onclick={() => save(true)}>Save &amp; apply</button></footer>{/if}
	{:else if unit?.list}
		<div class="list-head"><p class="muted">{unit.list.length} indexed units</p>{#if section === 'notifiers'}<a class="primary-link" href="/manager/settings/notifiers/new">New notifier</a>{/if}</div>
		{#if section === 'routes'}<div class="route-tools"><h2>Match preview</h2><p class="muted">Routes match task names in order; a pattern is a glob.</p><div class="preview-row"><input aria-label="Task pattern" bind:value={matchPattern} /><button class="quiet" onclick={previewMatches}>Which tasks match?</button></div>{#if matchedTasks.length}<p class="muted">Matches: {matchedTasks.join(', ')}</p>{:else}<p class="muted">No matches yet.</p>{/if}</div><div class="unit-list">{#each routeOrder as item, index}<div class="route-row"><span><strong>Route {index + 1}</strong><small>{item.file}</small></span><button class="icon" aria-label="Move route up" disabled={index === 0 || routeBusy} onclick={() => moveRoute(index, -1)}>↑</button><button class="icon" aria-label="Move route down" disabled={index === routeOrder.length - 1 || routeBusy} onclick={() => moveRoute(index, 1)}>↓</button></div>{:else}<p class="muted">No routes found.</p>{/each}</div>
		{:else}<div class="unit-list">{#each unit.list as item}<a href={`/manager/settings/${section}/${encodeURIComponent(item.name)}`}><strong>{item.name || '(unnamed)'}</strong><span>{item.file}</span></a>{:else}<p class="muted">No {titles[section].toLowerCase()} found.</p>{/each}</div>{#if section === 'compose' && unit.overrides.length}<h2 class="subhead">Overrides</h2><div class="unit-list">{#each unit.overrides as item}<a href={`/manager/settings/compose/${encodeURIComponent(item.name)}`}><strong>{item.name || '(unnamed)'}</strong><span>{item.file}</span></a>{/each}</div>{/if}{/if}
	{:else}<div class="file-actions"><p class="muted">Create an empty managed include file, then add its glob in daemon settings.</p><div class="preview-row"><input aria-label="New include path" bind:value={newFilePath} /><button class="quiet" disabled={identity?.role !== 'admin'} onclick={createInclude}>Create file</button></div></div><div class="files">{#each tree?.files ?? [] as file}<article class:error={file.error}><code>{file.path}</code><span>{file.error ?? `${file.size} bytes · ${file.version?.slice(0, 12)}`}</span></article>{/each}</div>{/if}
</section>{/if}

<style>
	.settings-page { max-width: 1080px; padding: 4vh 0 90px; } .settings-head { display:flex; justify-content:space-between; align-items:end; gap:24px; border-bottom:1px solid var(--color-border,#d8dee8); padding-bottom:24px; } .eyebrow,.rail-mark { font:11px var(--font-mono,monospace); opacity:.58; } h1 { margin:8px 0; font-size:clamp(30px,4vw,50px); letter-spacing:-.06em; } .muted { font-size:12px; line-height:1.6; opacity:.63; } .file-link,.section-nav a,.primary-link { color:inherit; font-size:12px; } .section-nav { display:flex; gap:18px; overflow:auto; border-bottom:1px solid var(--color-border,#d8dee8); padding:14px 0; } .section-nav a { text-decoration:none; opacity:.58; white-space:nowrap; } .section-nav a.active { opacity:1; border-bottom:2px solid currentColor; padding-bottom:13px; margin-bottom:-15px; } .editor-layout { display:grid; grid-template-columns:minmax(190px,.55fr) minmax(0,1.45fr); gap:48px; padding-top:34px; } .editor-copy { max-width:270px; } .rail-mark { display:block; margin-bottom:10px; } .editor-copy > p { margin-top:0; font-size:13px; line-height:1.7; } .form-pane { max-width:700px; } .overrides { margin-top:38px; border-top:1px solid var(--color-border,#d8dee8); } .overrides h2 { font-size:15px; margin-bottom:4px; } details { border-bottom:1px solid var(--color-border,#d8dee8); padding:10px 0; font-size:12px; } summary { cursor:pointer; } summary span { float:right; opacity:.55; } details p { line-height:1.5; opacity:.65; } .banner { margin:18px 0; border-left:3px solid currentColor; padding:11px 13px; font-size:12px; background:color-mix(in srgb,currentColor 7%,transparent); } .banner.danger,.error { color:var(--color-danger,#b24d43); } .banner.success { color:var(--color-success,#2d7659); } .banner.restart { color:#996b1f; } .banner code { font-family:var(--font-mono,monospace); } .savebar { position:fixed; bottom:0; left:232px; right:0; display:flex; align-items:center; gap:12px; padding:13px 32px; border-top:1px solid var(--color-border,#d8dee8); background:var(--color-background,#fff); z-index:3; font-size:11px; } .savebar span { margin-right:auto; } .savebar strong { font-size:16px; } .quiet,.primary { border:1px solid currentColor; border-radius:3px; padding:9px 13px; background:transparent; color:inherit; font:inherit; cursor:pointer; } .primary { background:currentColor; color:var(--color-background,#fff); text-decoration:none; } .unit-list,.files { display:grid; gap:8px; margin-top:24px; } .unit-list a,.files article { display:flex; flex-direction:column; gap:7px; border:1px solid var(--color-border,#d8dee8); padding:15px; color:inherit; text-decoration:none; } .unit-list a:hover,.unit-list a:focus-visible { border-color:currentColor; } .unit-list span,.files span { font-size:11px; opacity:.6; overflow-wrap:anywhere; } .state { padding:10vh 0; } .list-head { display:flex; justify-content:space-between; align-items:center; margin-top:26px; } @media(max-width:760px){.settings-head,.editor-layout{display:block}.file-link{display:inline-block;margin-top:12px}.editor-copy{max-width:none;margin-bottom:28px}.savebar{left:0;padding:12px 18px;flex-wrap:wrap}.savebar span{width:100%}}
</style>
