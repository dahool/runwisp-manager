// @ts-nocheck
const json = async (url, options = {}) => {
	const response = await fetch(url, { headers: { Accept: 'application/json', ...(options.headers ?? {}) }, ...options });
	let body = null;
	try { body = await response.json(); } catch { /* non-JSON response */ }
	if (!response.ok) { const error = new Error(body?.message ?? body?.error ?? `Request failed (${response.status})`); error.status = response.status; error.body = body; throw error; }
	return body;
};
export const editorApi = {
	unit: (kind, name) => json(`/manager/api/units/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`),
	units: (kind) => json(`/manager/api/units?kind=${encodeURIComponent(kind)}`),
	schema: () => json('/manager/api/schema'),
	validate: (files, mode = 'quick') => json(`/manager/api/validate?mode=${mode}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ files }) }),
	preview: (change) => json('/manager/api/config/changes:preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(change) }),
	save: (change) => json('/manager/api/config/changes', { method: 'POST', headers: { 'Content-Type': 'application/json', 'Idempotency-Key': change.idempotencyKey }, body: JSON.stringify(change) }),
	reload: () => json('/manager/api/config/reload', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }),
	reveal: (file, keyPath) => json('/manager/api/config/secrets:reveal', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ file, keyPath }) })
};
