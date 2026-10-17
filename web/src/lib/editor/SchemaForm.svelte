<script>
	// @ts-nocheck
	import { getAt, setAt } from './ops.js';
	let { schema = {}, value = {}, errors = {}, onChange = () => {}, readOnly = false, nextRuns = [] } = $props();
	const properties = $derived(schema?.properties ?? {});
	const required = $derived(new Set(schema?.required ?? []));
	const labelFor = (key) => key.replace(/([A-Z])/g, ' $1').replace(/^./, (letter) => letter.toUpperCase());
	const kind = (definition) => definition?.type ?? (definition?.enum ? 'enum' : 'string');
	function update(path, next) { onChange(setAt(value, path, next)); }
	function errorFor(path) { return errors[path.join('.')] ?? errors[`/${path.join('/')}`] ?? ''; }
	const secretField = (key, definition) => Boolean(definition?.secret || /token|password|webhook|credential|private/i.test(key));
	const fieldValue = (key, path) => { const current = getAt(value, path); return current && typeof current === 'object' && current.masked ? '' : current ?? ''; };
	function move(path, index, direction) {
		const items = [...(getAt(value, path) ?? [])]; const target = index + direction;
		if (target < 0 || target >= items.length) return;
		[items[index], items[target]] = [items[target], items[index]]; update(path, items);
	}
</script>

<div class="form-groups">
	{#each Object.entries(properties) as [key, definition]}
		{@const path = [key]}
		{@const fieldError = errorFor(path)}
		<section class:error={fieldError} class="field">
			<label for={`field-${key}`}><span>{definition.title ?? labelFor(key)}</span>{#if required.has(key)} <b aria-label="required">required</b>{/if}</label>
			{#if definition.description}<p class="help">{definition.description}</p>{/if}
			{#if definition.restartRequired}<p class="restart-hint">Takes effect after a daemon restart.</p>{/if}
			{#if kind(definition) === 'boolean'}
				<label class="toggle"><input id={`field-${key}`} type="checkbox" checked={Boolean(getAt(value, path))} disabled={readOnly} onchange={(event) => update(path, event.currentTarget.checked)} /><span>{getAt(value, path) ? 'Enabled' : 'Disabled'}</span></label>
			{:else if definition.enum}
				<select id={`field-${key}`} disabled={readOnly} value={getAt(value, path) ?? ''} onchange={(event) => update(path, event.currentTarget.value)}><option value="">Choose…</option>{#each definition.enum as option}<option value={option}>{option}</option>{/each}</select>
			{:else if kind(definition) === 'number' || kind(definition) === 'integer'}
				<input id={`field-${key}`} type="number" min={definition.minimum} max={definition.maximum} step={kind(definition) === 'integer' ? 1 : 'any'} value={getAt(value, path) ?? ''} disabled={readOnly} aria-describedby={fieldError ? `error-${key}` : undefined} oninput={(event) => update(path, event.currentTarget.value === '' ? undefined : Number(event.currentTarget.value))} />
			{:else if definition.oneOf}
				<select id={`field-${key}`} value={getAt(value, path) ?? ''} disabled={readOnly} onchange={(event) => update(path, event.currentTarget.value)}><option value="">Choose variant…</option>{#each definition.oneOf as variant, index}<option value={variant.const ?? variant.title ?? index}>{variant.title ?? variant.const ?? `Variant ${index + 1}`}</option>{/each}</select>
			{:else if definition.type === 'array'}
				<div class="repeatable">{#each getAt(value, path) ?? [] as item, index}<div class="repeat-row"><input aria-label={`${definition.title ?? key} item ${index + 1}`} value={typeof item === 'object' ? JSON.stringify(item) : item} disabled={readOnly} oninput={(event) => { const next = [...(getAt(value, path) ?? [])]; next[index] = definition.items?.type === 'object' ? event.currentTarget.value : event.currentTarget.value; update(path, next); }} /><button type="button" class="icon" aria-label="Move item up" disabled={readOnly || index === 0} onclick={() => move(path, index, -1)}>↑</button><button type="button" class="icon" aria-label="Move item down" disabled={readOnly || index === (getAt(value, path)?.length ?? 1) - 1} onclick={() => move(path, index, 1)}>↓</button></div>{/each}<button type="button" class="add" disabled={readOnly} onclick={() => update(path, [...(getAt(value, path) ?? []), definition.items?.type === 'object' ? {} : ''])}>Add item</button></div>
			{:else if definition.type === 'object'}
				<div class="map-editor">{#each Object.entries(getAt(value, path) ?? {}) as [mapKey, mapValue]}<div class="map-row"><input aria-label={`${key} key`} value={mapKey} disabled={readOnly} /><input aria-label={`${mapKey} value`} type={definition.secret ? 'password' : 'text'} value={mapValue} disabled={readOnly} oninput={(event) => { const next = { ...(getAt(value, path) ?? {}) }; next[mapKey] = event.currentTarget.value; update(path, next); }} /></div>{/each}<button type="button" class="add" disabled={readOnly} onclick={() => update(path, { ...(getAt(value, path) ?? {}), [`key-${Date.now()}`]: '' })}>Add key</button><p class="help">Values may use <code>{'${VAR}'}</code> or <code>{'${file:…}'}</code>.</p></div>
			{:else if definition.format === 'duration' || definition.format === 'byteSize' || definition.widget === 'cron' || definition['x-widget'] === 'code'}
				<textarea id={`field-${key}`} class:mono={definition.widget === 'cron' || definition['x-widget'] === 'code'} rows={definition['x-widget'] === 'code' ? 6 : 2} placeholder={definition.format === 'duration' ? 'e.g. 30s' : definition.widget === 'cron' ? 'e.g. 0 * * * *' : ''} value={getAt(value, path) ?? ''} disabled={readOnly} aria-describedby={fieldError ? `error-${key}` : undefined} oninput={(event) => update(path, event.currentTarget.value)}></textarea>
				{#if definition.widget === 'cron' && nextRuns.length}<p class="help">Next runs: {nextRuns.join(' · ')}</p>{/if}
			{:else}
				<input id={`field-${key}`} type={secretField(key, definition) ? 'password' : 'text'} placeholder={secretField(key, definition) ? 'Write a value or use ${VAR}' : ''} value={fieldValue(key, path)} disabled={readOnly} aria-describedby={fieldError ? `error-${key}` : undefined} oninput={(event) => update(path, event.currentTarget.value)} />
				{#if secretField(key, definition)}<p class="help">Keep secrets out of Git. Use <code>{'${VAR}'}</code> or <code>{'${file:…}'}</code>.</p>{/if}
			{/if}
			{#if fieldError}<p id={`error-${key}`} class="field-error" role="alert">{fieldError}</p>{/if}
		</section>
	{/each}
	{#if Object.keys(properties).length === 0}<p class="help">No schema fields were returned. Use the Advanced editor to preserve this unit.</p>{/if}
</div>

<style>
	.form-groups { display: grid; gap: 20px; }
	.field { border-bottom: 1px solid var(--color-border, #d8dee8); padding-bottom: 18px; }
	.field.error { border-color: var(--color-danger, #b24d43); }
	label { display: flex; align-items: baseline; gap: 7px; font-size: 12px; font-weight: 600; }
	label b { color: var(--color-danger, #b24d43); font-size: 10px; font-weight: 500; }
	.help { margin: 5px 0 8px; font-size: 11px; line-height: 1.5; opacity: .62; }
	input:not([type='checkbox']), select, textarea { box-sizing: border-box; width: 100%; border: 1px solid var(--color-border, #d8dee8); border-radius: 3px; background: var(--color-background, white); color: inherit; padding: 10px; font: inherit; font-size: 12px; }
	input:focus-visible, select:focus-visible, textarea:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }
	textarea { resize: vertical; min-height: 42px; }
	textarea.mono { font-family: var(--font-mono, ui-monospace, monospace); }
	.toggle { justify-content: space-between; border: 1px solid var(--color-border, #d8dee8); border-radius: 3px; padding: 10px; font-weight: 400; }
	.toggle input { accent-color: currentColor; }
	.field-error { margin: 6px 0 0; color: var(--color-danger, #b24d43); font-size: 11px; }
	.restart-hint { margin: 5px 0 8px; color: #996b1f; font-size: 11px; }
	.repeatable, .map-editor { display: grid; gap: 7px; } .repeat-row, .map-row { display: grid; grid-template-columns: 1fr auto auto; gap: 6px; } .map-row { grid-template-columns: .8fr 1.2fr; } .icon, .add { border: 1px solid var(--color-border, #d8dee8); border-radius: 3px; background: transparent; color: inherit; padding: 7px 9px; font: inherit; font-size: 11px; } .add { justify-self: start; }
</style>
