<script>
	// @ts-nocheck
	import { onMount } from 'svelte';
	import SchemaForm from './SchemaForm.svelte';
	import { editorApi } from './api.js';
	import { diffOps, toSaveOps } from './ops.js';
	let { kind, name, isNew = false, onClose = () => {} } = $props();
	let unit = $state(null), schema = $state(null), form = $state({}), original = $state({}), errors = $state({});
	let identity = $state(null), loading = $state(true), busy = $state(false), error = $state('');
	let review = $state(null), notice = $state(''), tab = $state('form'), raw = $state(''), rawOriginal = $state('');
	let conflict = $state(false);
	const editable = $derived(['editor', 'admin'].includes(identity?.role));
	const operations = $derived(diffOps(original, form));
	const dirty = $derived(operations.length > 0 || raw !== rawOriginal);
	const title = $derived(isNew ? `New ${kind}` : `Edit ${kind} · ${name}`);

	function findSchema(document) {
		const root = document?.resolvedSchema ?? document?.schema ?? {};
		return root?.$defs?.[kind] ?? root?.properties?.[kind] ?? root?.[kind] ?? { type: 'object', properties: Object.fromEntries(Object.keys(form).map((key) => [key, { type: typeof form[key] === 'boolean' ? 'boolean' : typeof form[key] === 'number' ? 'number' : 'string', title: key }])) };
	}
	async function load() {
		try {
			const [me, document] = await Promise.all([fetch('/manager/api/me').then((response) => response.ok ? response.json() : null).catch(() => null), editorApi.schema().catch(() => null)]);
		identity = me?.identity ?? me;
		schema = document;
		if (!isNew) { unit = await editorApi.unit(kind, name); form = structuredClone(unit.fields ?? {}); original = structuredClone(form); }
		else { form = { name: '' }; original = {}; }
	} catch (cause) { error = cause.message ?? 'Unable to load editor'; }
		loading = false;
	}
	async function loadRaw() {
		if (!unit?.fileId) return;
		try { raw = await fetch(`/manager/api/config/files/${encodeURIComponent(unit.fileId)}?raw=true`).then((response) => response.text()); rawOriginal = raw; } catch { error = 'Raw configuration is unavailable'; }
	}
	function update(next) { form = next; notice = ''; }
	$effect(() => {
		const timer = setTimeout(() => {
			const next = {};
			for (const field of schema?.resolvedSchema?.$defs?.[kind]?.required ?? schema?.schema?.$defs?.[kind]?.required ?? []) if (form[field] === undefined || form[field] === '') next[field] = 'This field is required.';
			const schedule = form.schedule;
			if (typeof schedule === 'string' && schedule.trim() && schedule.trim().split(/\s+/).length !== 5) next.schedule = 'Use five cron fields, for example 0 * * * *.';
			errors = next;
		}, 400);
		return () => clearTimeout(timer);
	});
	function setField(path, value) { form = { ...form, [path]: value }; }
	function issueMap(result) {
		const next = {};
		for (const issue of result?.errors ?? result?.issues ?? []) { const path = issue.path ?? issue.pointer ?? ''; next[String(path).replace(/^#?\/?/, '').replaceAll('/', '.')] = issue.message; }
		return next;
	}
	async function reviewChanges() {
		busy = true; error = '';
		try {
			const validation = await editorApi.validate([], 'full').catch((cause) => ({ errors: [{ message: cause.message }] }));
			errors = issueMap(validation);
			review = { validation, ops: operations };
		} catch (cause) { error = cause.message; }
		busy = false;
	}
	function changeRequest(apply) {
		return { idempotencyKey: crypto.randomUUID(), base: { [unit?.fileId ?? unit?.file ?? '']: unit?.fileVersion ?? unit?.hash ?? '' }, ops: tab === 'raw' ? [] : [{ file: unit?.fileId ?? unit?.file ?? '', ops: toSaveOps(operations) }], raw: tab === 'raw' ? [{ file: unit?.fileId ?? unit?.file ?? '', content: raw }] : [], apply, reason: `${isNew ? 'create' : 'edit'} ${kind} ${name ?? form.name ?? ''}` };
	}
	async function save(apply = false) {
		busy = true; error = ''; notice = '';
		try {
			const result = await editorApi.save(changeRequest(apply));
			notice = result?.warnings?.join(' ') ?? (apply ? 'Saved and applied.' : 'Saved.');
			review = null; original = structuredClone(form); rawOriginal = raw; unit = { ...unit, hash: result?.hash ?? unit?.hash };
		} catch (cause) {
			if (cause.status === 409) conflict = true; else error = cause.message;
		}
		busy = false;
	}
	onMount(load);
	$effect(() => {
		if (!dirty || typeof window === 'undefined') return;
		const guard = (event) => { event.preventDefault(); event.returnValue = ''; };
		window.addEventListener('beforeunload', guard);
		return () => window.removeEventListener('beforeunload', guard);
	});
</script>

{#if loading}<section class="state">Loading editor…</section>
{:else if error && !unit && !isNew}<section class="state error" role="alert">{error}</section>
{:else}<section class="editor-page">
		<div class="editor-head"><div><a class="back" href={`/manager/${kind}s`}>← {kind}s</a><h1>{title}</h1><p class="muted">{unit?.file ?? 'New unit'}{#if unit?.fileVersion} · version {unit.fileVersion.slice(0, 12)}{/if}</p></div><div class="head-actions">{#if !isNew}<a class="quiet" href={`/manager/${kind}s/${encodeURIComponent(name)}`}>Read view</a>{/if}</div></div>
		{#if !editable}<div class="banner" role="status">You have view access. An editor role is required to save changes.</div>{/if}
		{#if error}<div class="banner danger" role="alert">{error}</div>{/if}{#if notice}<div class="banner success" role="status">{notice}</div>{/if}
		<div class="tabs" role="tablist"><button class:active={tab === 'form'} role="tab" aria-selected={tab === 'form'} onclick={() => tab = 'form'}>Form</button>{#if identity?.role === 'admin'}<button class:active={tab === 'raw'} role="tab" aria-selected={tab === 'raw'} onclick={() => { tab = 'raw'; loadRaw(); }}>Raw TOML</button>{/if}</div>
		<div class="editor-body">
			{#if tab === 'form'}<SchemaForm schema={findSchema(schema)} value={form} {errors} readOnly={!editable} onChange={update} />
			{:else}<label class="raw-label" for="raw-editor">Raw TOML <span>bypasses the form; comments are preserved as typed.</span></label><textarea id="raw-editor" class="raw-editor" bind:value={raw} placeholder="Paste this unit's TOML here…" disabled={!editable}></textarea>{/if}
		</div>
		{#if dirty}<footer class="savebar"><span><strong>{operations.length}</strong> unsaved {operations.length === 1 ? 'change' : 'changes'}</span><button class="quiet" onclick={() => { form = structuredClone(original); notice = 'Changes discarded.'; }}>Discard</button><button class="primary" disabled={!editable || busy} onclick={reviewChanges}>{busy ? 'Checking…' : 'Review changes'}</button></footer>{/if}
	</section>{/if}

{#if review}<div class="scrim" role="presentation"><div class="dialog" role="dialog" aria-modal="true" aria-labelledby="review-title"><h2 id="review-title">Review changes</h2><p class="muted">{review.ops.length} surgical operations will be sent. Untouched bytes stay in place.</p>{#if Object.keys(errors).length}<div class="banner danger">Fix the highlighted fields before saving.</div>{/if}<pre>{review.ops.map((op) => `${op.op === 'delete' ? 'delete' : 'set'} ${op.path.join('.')}${op.value === undefined ? '' : ` = ${JSON.stringify(op.value)}`}`).join('\n') || 'No changes'}</pre><div class="dialog-actions"><button class="quiet" onclick={() => review = null}>Cancel</button><button class="quiet" disabled={Object.keys(errors).length > 0 || busy} onclick={() => save(false)}>Save</button><button class="primary" disabled={Object.keys(errors).length > 0 || busy} onclick={() => save(true)}>Save &amp; apply</button></div></div></div>{/if}
{#if conflict}<div class="scrim" role="presentation"><div class="dialog" role="dialog" aria-modal="true" aria-labelledby="conflict-title"><h2 id="conflict-title">File changed since you opened it</h2><p class="muted">Another change updated this file. Reload the form to start from the current version, or keep your changes and re-merge them carefully.</p><div class="dialog-actions"><button class="quiet" onclick={() => { conflict = false; location.reload(); }}>Reload form</button><button class="primary" onclick={() => { conflict = false; error = 'Re-merge is available after reloading the current file; review your changes before saving again.'; }}>Keep my changes and re-merge</button></div></div></div>{/if}

<style>
	.editor-page { max-width: 900px; padding: 16px 0 88px; } .editor-head { display: flex; justify-content: space-between; gap: 20px; align-items: start; border-bottom: 1px solid var(--color-border, #d8dee8); padding-bottom: 22px; } h1 { font-size: clamp(25px, 4vw, 42px); letter-spacing: -.05em; margin: 10px 0; } .muted, .back, .quiet, .raw-label span { font-size: 11px; opacity: .62; } .back, .quiet { color: inherit; } .head-actions { padding-top: 28px; } .banner { margin: 16px 0; border-left: 3px solid currentColor; padding: 11px 13px; font-size: 12px; background: color-mix(in srgb, currentColor 7%, transparent); } .banner.danger { color: var(--color-danger, #b24d43); } .banner.success { color: var(--color-success, #2d7659); } .tabs { display: flex; gap: 18px; border-bottom: 1px solid var(--color-border, #d8dee8); margin: 22px 0 28px; } .tabs button { border: 0; border-bottom: 2px solid transparent; background: none; color: inherit; cursor: pointer; padding: 10px 0; font: inherit; font-size: 12px; } .tabs button.active { border-color: currentColor; } .editor-body { max-width: 680px; } .raw-label { display: block; margin-bottom: 10px; } .raw-label span { display: block; margin-top: 6px; font-weight: 400; } .raw-editor { width: 100%; min-height: 440px; box-sizing: border-box; padding: 14px; border: 1px solid var(--color-border, #d8dee8); border-radius: 3px; background: var(--color-background, white); color: inherit; font: 12px/1.6 var(--font-mono, monospace); } button { font: inherit; cursor: pointer; } .primary, .quiet { border: 1px solid currentColor; border-radius: 3px; padding: 9px 13px; background: transparent; color: inherit; } .primary { background: currentColor; color: var(--color-background, white); } button:disabled { opacity: .45; cursor: not-allowed; } .savebar { position: fixed; bottom: 0; left: 232px; right: 0; display: flex; align-items: center; justify-content: flex-end; gap: 12px; padding: 13px 32px; border-top: 1px solid var(--color-border, #d8dee8); background: var(--color-background, white); font-size: 11px; z-index: 3; } .savebar span { margin-right: auto; } .savebar strong { font-size: 16px; } .scrim { position: fixed; inset: 0; display: grid; place-items: center; padding: 20px; background: #0008; z-index: 5; } .dialog { width: min(680px, 100%); max-height: 80vh; overflow: auto; background: var(--color-background, white); color: inherit; padding: 24px; border: 1px solid var(--color-border, #d8dee8); border-radius: 4px; } .dialog h2 { margin-top: 0; } pre { overflow: auto; padding: 14px; background: color-mix(in srgb, currentColor 7%, transparent); font: 12px/1.7 var(--font-mono, monospace); white-space: pre-wrap; } .dialog-actions { display: flex; justify-content: flex-end; gap: 10px; } .state { padding: 10vh 0; } .error { color: var(--color-danger, #b24d43); } @media (max-width: 700px) { .savebar { left: 0; padding: 12px 18px; flex-wrap: wrap; } .savebar span { width: 100%; } }
</style>
