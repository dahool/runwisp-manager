// @ts-nocheck
const clone = (value) => value === undefined ? undefined : structuredClone(value);
const equal = (a, b) => JSON.stringify(a) === JSON.stringify(b);
export function pathParts(path) { return Array.isArray(path) ? path : String(path).split('.').filter(Boolean); }
export function getAt(value, path) { return pathParts(path).reduce((current, key) => current == null ? undefined : current[key], value); }
export function setAt(value, path, next) {
	const parts = pathParts(path); if (!parts.length) return clone(next);
	const result = clone(value) ?? {}; let current = result;
	for (let i = 0; i < parts.length - 1; i += 1) { if (!current[parts[i]] || typeof current[parts[i]] !== 'object') current[parts[i]] = {}; current = current[parts[i]]; }
	if (next === undefined || next === '') delete current[parts.at(-1)]; else current[parts.at(-1)] = clone(next);
	return result;
}
export function diffOps(original, current, prefix = []) {
	if (equal(original, current)) return [];
	if (original && current && typeof original === 'object' && typeof current === 'object' && !Array.isArray(original) && !Array.isArray(current)) {
		const keys = new Set([...Object.keys(original), ...Object.keys(current)]);
		return [...keys].sort().flatMap((key) => diffOps(original[key], current[key], [...prefix, key]));
	}
	if (current === undefined || current === '') return [{ op: 'delete', path: prefix }];
	return [{ op: 'set', path: prefix, value: clone(current) }];
}
export function toSaveOps(operations) { return operations.map((operation) => operation.op === 'delete' ? { type: 'deleteKey', path: operation.path } : { type: 'setValue', path: operation.path, value: operation.value }); }
