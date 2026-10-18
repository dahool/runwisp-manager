<script>
	// @ts-nocheck
	import TaskEditor from '$lib/editor/TaskEditor.svelte';
	import SettingsPage from '$lib/settings/SettingsPage.svelte';
	let { data } = $props();
	let tree = $state(null), units = $state([]), detail = $state(null), error = $state(''), loading = $state(true), gitStatus = $state(null), gitDiffs = $state({}), commitMessage = $state(''), commitResult = $state('');
	const path = $derived(data.path ?? ''), parts = $derived(path.split('/').filter(Boolean));
	const section = $derived(parts[0] || 'overview'), kind = $derived(section === 'services' ? 'service' : section === 'tasks' ? 'task' : '');
	const editMode = $derived(Boolean(kind && (parts[1] === 'new' || parts[2] === 'edit'))), editName = $derived(parts[1] === 'new' ? '' : parts[1]);
	const isDetail = $derived(Boolean(kind && parts.length > 1 && !editMode));
	const settingsSection = $derived(section === 'settings' ? (parts[1] || 'files') : '');
	const settingsName = $derived(parts[2] || '');
	const settingsEditor = $derived(section === 'settings' && ['notifiers'].includes(settingsSection) && parts.length > 2);
	async function loadGit() {
		const response = await fetch('/manager/api/git/status');
		if (!response.ok) throw new Error('Git integration unavailable');
		gitStatus = await response.json();
		for (const file of gitStatus.files ?? []) {
			const diff = await fetch(`/manager/api/git/diff?path=${encodeURIComponent(file.fileId || file.path)}`);
			if (diff.ok) gitDiffs[file.fileId || file.path] = (await diff.json()).diff;
		}
	}
	async function commitGit() {
		commitResult = '';
		const response = await fetch('/manager/api/git/commit', { method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify({paths: (gitStatus?.files ?? []).map((file) => file.fileId || file.path), message: commitMessage})});
		commitResult = response.ok ? `Committed ${(await response.json()).sha}` : ((await response.json()).detail || 'Commit failed');
		if (response.ok) await loadGit();
	}
	async function loadPage() {
		tree = null; units = []; detail = null; error = ''; loading = true; gitStatus = null; gitDiffs = {};
		if (editMode) { loading = false; return; }
		try {
			const treeResponse = await fetch('/manager/api/config/tree'); if (treeResponse.ok) tree = await treeResponse.json();
			if (section === 'git') await loadGit();
			if (isDetail) { const response = await fetch(`/manager/api/units/${kind}/${encodeURIComponent(parts[1])}`); if (response.ok) detail = await response.json(); else error = 'Unit not found'; }
			else if (kind) { const response = await fetch(`/manager/api/units?kind=${kind}`); if (response.ok) units = (await response.json()).units ?? []; }
		} catch { error = 'Configuration tree is unavailable'; } loading = false;
	}
	$effect(() => { path; loadPage(); });
</script>

{#if editMode}<TaskEditor {kind} name={editName} isNew={parts[1] === 'new'} />
{:else if loading}<section class="state">Loading configuration…</section>
{:else if error}<section class="state error">{error}</section>
{:else if section === 'git'}<section class="page"><p class="eyebrow">Local repository</p><h1>Git changes</h1>{#if gitStatus}<p class="muted">{gitStatus.branch || 'detached'} · {gitStatus.files?.length ?? 0} managed changes</p>{#if gitStatus.inProgress}<div class="notice">A {gitStatus.inProgress} is in progress; commits are disabled.</div>{/if}<div class="files">{#each gitStatus.files ?? [] as file}<article><strong>{file.path}</strong><span>{file.staged ? 'staged' : file.untracked ? 'untracked' : 'modified'}</span><pre>{gitDiffs[file.fileId || file.path] || 'No diff available.'}</pre></article>{:else}<p class="muted">Working tree is clean.</p>{/each}</div>{#if gitStatus.files?.length}<label class="commit-label" for="commit-message">Commit message</label><textarea id="commit-message" bind:value={commitMessage} placeholder="Describe this configuration change"></textarea><button onclick={commitGit} disabled={!commitMessage.trim() || !!gitStatus.inProgress}>Commit selected changes</button>{/if}{#if commitResult}<p class="notice">{commitResult}</p>{/if}{:else}<p class="muted">Git integration is unavailable for this configuration.</p>{/if}</section>
{:else if settingsEditor}<TaskEditor kind="notifier" name={settingsName === 'new' ? '' : settingsName} isNew={settingsName === 'new'} />
{:else if section === 'settings'}<SettingsPage section={settingsSection} name={settingsName} />
{:else if isDetail}<section class="page"><a class="back" href={`/manager/${section}`}>← {kind}s</a><h1>{detail?.name}</h1><p class="muted">{detail?.file} · line {detail?.line?.[0]}–{detail?.line?.[1]}</p><div class="detail-actions"><a href={`/manager/${section}/${encodeURIComponent(detail?.name)}/edit`}>Edit</a><a href={`/manager/${section}/new?clone=${encodeURIComponent(detail?.name)}`}>Clone</a><a href={`/tasks/${encodeURIComponent(detail?.name)}`}>Open in RunWisp</a></div><p class="muted">Rename and delete are available from the editor after references are reviewed.</p><dl>{#each Object.entries(detail?.fields ?? {}) as [key, value]}<div><dt>{key}</dt><dd><code>{typeof value === 'object' ? JSON.stringify(value) : value}</code></dd></div>{/each}</dl></section>
{:else if kind}<section class="page"><p class="eyebrow">Configuration</p><div class="list-head"><h1>{kind[0].toUpperCase() + kind.slice(1)}s</h1><a class="new" href={`/manager/${section}/new`}>New {kind}</a></div><div class="toolbar">{units.length} indexed units</div><div class="grid">{#each units as unit}<a href={`/manager/${section}/${encodeURIComponent(unit.name)}`}><strong>{unit.name || '(global)'}</strong><span>{unit.file}</span><small>{unit.schedule || 'No schedule'}{#if unit.duplicate} · duplicate{/if}</small></a>{:else}<p class="muted">No units found.</p>{/each}</div></section>
{:else}<section class="page"><p class="eyebrow">RunWisp Manager</p><h1>Configuration overview</h1>{#if tree?.stale}<div class="notice">Configuration changed; reload RunWisp to clear stale state.</div>{/if}<div class="summary"><div><strong>{tree?.files?.length ?? 0}</strong><span>files</span></div><div><strong>{tree?.units?.length ?? 0}</strong><span>units</span></div><div><strong>{tree?.warnings?.length ?? 0}</strong><span>warnings</span></div></div>{#if tree?.warnings?.length}<ul>{#each tree.warnings as warning}<li>{warning}</li>{/each}</ul>{/if}</section>{/if}

<style>
	.page { max-width: 920px; padding: 5vh 2vw; } h1 { font-size: clamp(28px, 4vw, 48px); letter-spacing: -.05em; margin: 8px 0 12px; } .eyebrow { font: 11px ui-monospace, monospace; opacity: .55; text-transform: uppercase; } .muted, .grid span, .grid small, .files span { opacity: .62; font-size: 12px; } .state { padding: 10vh 3vw; } .error, .files .error { color: #b24d43; } .summary { display: flex; gap: 12px; margin: 38px 0; flex-wrap: wrap; } .summary div { border: 1px solid var(--color-border, #d8dee8); padding: 18px 24px; min-width: 100px; } .summary strong { display: block; font-size: 28px; } .summary span { font-size: 11px; opacity: .6; } .notice { padding: 12px 14px; background: color-mix(in srgb, currentColor 10%, transparent); font-size: 12px; } li { font-size: 12px; margin: 8px 0; } .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 10px; margin-top: 16px; } .grid a, .files article { display: flex; flex-direction: column; gap: 8px; border: 1px solid var(--color-border, #d8dee8); padding: 15px; color: inherit; text-decoration: none; } .grid a:hover, .grid a:focus-visible { border-color: currentColor; } .files { display: grid; gap: 8px; margin-top: 25px; } .files code { overflow-wrap: anywhere; } .files pre { white-space: pre-wrap; overflow: auto; max-height: 360px; font-size: 11px; } textarea { display: block; width: 100%; min-height: 90px; margin: 8px 0 12px; padding: 10px; font: inherit; } button { border: 1px solid currentColor; background: transparent; padding: 10px 14px; cursor: pointer; } button:disabled { opacity: .45; cursor: not-allowed; } .commit-label { display: block; margin-top: 24px; } .back, .detail-actions a, .new { color: inherit; font-size: 12px; } .detail-actions { display: flex; gap: 16px; margin: 24px 0; } .list-head { display: flex; align-items: end; justify-content: space-between; gap: 16px; } .new { border: 1px solid currentColor; border-radius: 3px; padding: 9px 12px; text-decoration: none; }
</style>
